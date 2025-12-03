package listeners

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/TicketsBot-cloud/common/chatrelay"
	"github.com/TicketsBot-cloud/common/model"
	"github.com/TicketsBot-cloud/common/premium"
	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/gateway/payloads/events"
	"github.com/TicketsBot-cloud/worker"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/metrics/prometheus"
	"github.com/TicketsBot-cloud/worker/bot/metrics/statsd"
	"github.com/TicketsBot-cloud/worker/bot/redis"
	"github.com/TicketsBot-cloud/worker/bot/utils"
)

// proxy messages to web UI + set last message id
func OnMessage(worker *worker.Context, e events.MessageCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*7) // TODO: Propagate context
	defer cancel()

	span := sentry.StartTransaction(ctx, "OnMessage")
	defer span.Finish()

	if e.GuildId != 0 {
		span.SetTag("guild_id", strconv.FormatUint(e.GuildId, 10))
	}

	statsd.Client.IncrementKey(statsd.KeyMessages)

	// ignore DMs
	if e.GuildId == 0 {
		return
	}

	ticket, isTicket, err := getTicket(span.Context(), e.ChannelId)
	if err != nil {
		sentry.ErrorWithContext(err, utils.MessageCreateErrorContext(e))
		return
	}

	// ensure valid ticket channel
	if !isTicket || ticket.Id == 0 {
		return
	}

	var isStaffCached *bool

	// Start fetching premium tier early (in parallel)
	premiumChan := make(chan struct {
		tier premium.PremiumTier
		err  error
	}, 1)
	go func() {
		tier, err := sentry.WithSpan2(span.Context(), "Get premium tier", func(span *sentry.Span) (premium.PremiumTier, error) {
			// Use fresh context for premium check
			premiumCtx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			defer cancel()
			return utils.PremiumClient.GetTierByGuildId(premiumCtx, e.GuildId, true, worker.Token, worker.RateLimiter)
		})
		premiumChan <- struct {
			tier premium.PremiumTier
			err  error
		}{tier, err}
	}()

	// ignore our own messages
	if e.Author.Id != worker.BotId && !e.Author.Bot {
		// Run participant set and isStaff check in parallel
		var wg sync.WaitGroup
		var isStaffErr error

		// Set participants (parallel)
		wg.Add(1)
		go func() {
			defer wg.Done()
			sentry.WithSpan0(span.Context(), "Add participant", func(span *sentry.Span) {
				// Use fresh context for DB operation
				participantCtx, cancel := context.WithTimeout(context.Background(), time.Second*3)
				defer cancel()
				if err := dbclient.Client.Participants.Set(participantCtx, e.GuildId, ticket.Id, e.Author.Id); err != nil {
					sentry.ErrorWithContext(err, utils.MessageCreateErrorContext(e))
				}
			})
		}()

		// Check if user is staff (parallel)
		wg.Add(1)
		go func() {
			defer wg.Done()
			var v bool
			v, isStaffErr = sentry.WithSpan2(span.Context(), "Check if staff", func(span *sentry.Span) (bool, error) {
				// isStaff creates its own timeout context
				return isStaff(context.Background(), e, ticket)
			})
			isStaffCached = &v
		}()

		// Wait for both operations to complete
		wg.Wait()

		if isStaffErr != nil {
			sentry.ErrorWithContext(isStaffErr, utils.MessageCreateErrorContext(e))
		} else if isStaffCached != nil {
			// Run updateLastMessage and FirstResponseTime in parallel for staff
			var dbWg sync.WaitGroup

			// Update last message
			dbWg.Add(1)
			go func() {
				defer dbWg.Done()
				// Pass context.Background() directly since updateLastMessage creates its own timeouts
				if err := updateLastMessage(context.Background(), e, ticket, *isStaffCached); err != nil {
					sentry.ErrorWithContext(err, utils.MessageCreateErrorContext(e))
				}
			}()

			// Set first response time if staff
			if *isStaffCached {
				dbWg.Add(1)
				go func() {
					defer dbWg.Done()
					sentry.WithSpan0(span.Context(), "Set first response time", func(span *sentry.Span) {
						responseCtx, cancel := context.WithTimeout(context.Background(), time.Second*3)
						defer cancel()
						if err := dbclient.Client.FirstResponseTime.Set(responseCtx, e.GuildId, e.Author.Id, ticket.Id, time.Now().Sub(ticket.OpenTime)); err != nil {
							sentry.ErrorWithContext(err, utils.MessageCreateErrorContext(e))
						}
					})
				}()
			}

			dbWg.Wait()
		}
	}

	// Get premium tier result
	premiumResult := <-premiumChan
	premiumTier, err := premiumResult.tier, premiumResult.err
	if err != nil {
		sentry.ErrorWithContext(err, utils.MessageCreateErrorContext(e))
		return
	}

	// proxy msg to web UI
	if premiumTier > premium.None {
		if err := sentry.WithSpan1(span.Context(), "Relay message to dashboard", func(span *sentry.Span) error {
			data := chatrelay.MessageData{
				Ticket:  ticket,
				Message: e.Message,
			}

			prometheus.ForwardedDashboardMessages.Inc()

			return chatrelay.PublishMessage(redis.Client, data)
		}); err != nil {
			sentry.ErrorWithContext(err, utils.MessageCreateErrorContext(e))
		}

		// Ignore the welcome message and ping message
		if e.Author.Id != worker.BotId {
			var userIsStaff bool
			if isStaffCached != nil {
				userIsStaff = *isStaffCached
			} else {
				// isStaff creates its own timeout context
				tmp, err := isStaff(context.Background(), e, ticket)
				if err != nil {
					sentry.ErrorWithContext(err, utils.MessageCreateErrorContext(e))
					return
				}

				userIsStaff = tmp
			}

			var newStatus model.TicketStatus
			if userIsStaff {
				newStatus = model.TicketStatusPending
			} else {
				newStatus = model.TicketStatusOpen
			}

			if ticket.Status != newStatus {
				// Use fresh context for status update
				statusCtx, cancel := context.WithTimeout(context.Background(), time.Second*3)
				defer cancel()
				if err := dbclient.Client.Tickets.SetStatus(statusCtx, e.GuildId, ticket.Id, newStatus); err != nil {
					sentry.ErrorWithContext(err, utils.MessageCreateErrorContext(e))
				}

				if !ticket.IsThread {
					if err := sentry.WithSpan1(span.Context(), "Update status update queue", func(span *sentry.Span) error {
						queueCtx, queueCancel := context.WithTimeout(context.Background(), time.Second*3)
						defer queueCancel()
						return dbclient.Client.CategoryUpdateQueue.Add(queueCtx, e.GuildId, ticket.Id, newStatus)
					}); err != nil {
						sentry.ErrorWithContext(err, utils.MessageCreateErrorContext(e))
					}
				}
			}
		}
	}
}

func updateLastMessage(ctx context.Context, msg events.MessageCreate, ticket database.Ticket, isStaff bool) error {
	span := sentry.StartSpan(ctx, "Update last message")
	defer span.Finish()

	// Create a fresh context for the Get operation to ensure we have enough time
	getCtx, getCancel := context.WithTimeout(context.Background(), time.Second*2)
	defer getCancel()

	// If last message was sent by staff, don't reset the timer
	lastMessage, err := dbclient.Client.TicketLastMessage.Get(getCtx, ticket.GuildId, ticket.Id)
	if err != nil {
		return err
	}

	// Create a fresh context for the Set operation
	setCtx, setCancel := context.WithTimeout(context.Background(), time.Second*2)
	defer setCancel()

	// No last message, or last message was before we started storing user IDs
	if lastMessage.UserId == nil {
		return dbclient.Client.TicketLastMessage.Set(setCtx, ticket.GuildId, ticket.Id, msg.Id, msg.Author.Id, false)
	}

	// If the last message was sent by the ticket opener, we can skip the rest of the logic, and update straight away
	if ticket.UserId == msg.Author.Id {
		return dbclient.Client.TicketLastMessage.Set(setCtx, ticket.GuildId, ticket.Id, msg.Id, msg.Author.Id, false)
	}

	// If the last message *and* this message were sent by staff members, then do not reset the timer
	if lastMessage.UserId != nil && *lastMessage.UserIsStaff && isStaff {
		return nil
	}

	return dbclient.Client.TicketLastMessage.Set(setCtx, ticket.GuildId, ticket.Id, msg.Id, msg.Author.Id, isStaff)
}

// This method should not be used for anything requiring elevated privileges
func isStaff(ctx context.Context, msg events.MessageCreate, ticket database.Ticket) (bool, error) {
	// If the user is the ticket opener, they are not staff
	if msg.Author.Id == ticket.UserId {
		return false, nil
	}

	// Create fresh context for database operation
	memberCtx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	members, err := dbclient.Client.TicketMembers.Get(memberCtx, ticket.GuildId, ticket.Id)
	if err != nil {
		return false, err
	}

	if utils.Contains(members, msg.Author.Id) {
		return false, nil
	}

	return true, nil
}

func getTicket(ctx context.Context, channelId uint64) (database.Ticket, bool, error) {
	isTicket, err := sentry.WithSpan2(ctx, "IsTicketChannel redis lookup", func(span *sentry.Span) (bool, error) {
		return redis.IsTicketChannel(ctx, channelId)
	})

	cacheHit := err == nil

	if err == nil && !isTicket {
		prometheus.LogOnMessageTicketLookup(false, cacheHit)
		return database.Ticket{}, false, nil
	} else if err != nil && !errors.Is(err, redis.ErrTicketStatusNotCached) {
		return database.Ticket{}, false, err
	}

	// Either cache miss or the ticket *does* exist, so we need to fetch the object from the database
	ticket, err := sentry.WithSpan2(ctx, "Get ticket by channel", func(span *sentry.Span) (database.Ticket, error) {
		ticket, ok, err := dbclient.Client.Tickets.GetByChannel(ctx, channelId)
		if err != nil {
			return database.Ticket{}, err
		}

		if !ok {
			return database.Ticket{}, nil
		}

		return ticket, nil
	})

	if err != nil {
		return database.Ticket{}, false, err
	}

	if err := redis.SetTicketChannelStatus(ctx, channelId, ticket.Id != 0); err != nil {
		return database.Ticket{}, false, err
	}

	if ticket.Id == 0 {
		prometheus.LogOnMessageTicketLookup(false, cacheHit)
		return database.Ticket{}, false, nil
	}

	prometheus.LogOnMessageTicketLookup(true, cacheHit)

	return ticket, true, nil
}
