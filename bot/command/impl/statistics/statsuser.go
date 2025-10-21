package statistics

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/channel/embed"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/experiments"
	"github.com/TicketsBot-cloud/worker/i18n"
	"github.com/getsentry/sentry-go"
	"golang.org/x/sync/errgroup"
)

type StatsUserCommand struct {
}

func (StatsUserCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:            "user",
		Description:     i18n.HelpStats,
		Type:            interaction.ApplicationCommandTypeChatInput,
		Aliases:         []string{"statistics"},
		PermissionLevel: permission.Support,
		Category:        command.Statistics,
		PremiumOnly:     true,
		Arguments: command.Arguments(
			command.NewRequiredArgument("user", "User whose statistics to retrieve", interaction.OptionTypeUser, i18n.MessageInvalidUser),
		),
		DefaultEphemeral: true,
		Timeout:          time.Second * 30,
	}
}

func (c StatsUserCommand) GetExecutor() interface{} {
	return c.Execute
}

func (StatsUserCommand) Execute(ctx registry.CommandContext, userId uint64) {
	span := sentry.StartTransaction(ctx, "/stats user")
	span.SetTag("guild", strconv.FormatUint(ctx.GuildId(), 10))
	span.SetTag("user", strconv.FormatUint(userId, 10))
	defer span.Finish()

	member, err := ctx.Worker().GetGuildMember(ctx.GuildId(), userId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	permLevel, err := permission.GetPermissionLevel(ctx, utils.ToRetriever(ctx.Worker()), member, ctx.GuildId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// User stats
	if permLevel == permission.Everyone {
		var isBlacklisted bool
		var totalTickets int
		var openTickets int
		var ticketLimit uint8

		group, _ := errgroup.WithContext(ctx)

		// load isBlacklisted
		group.Go(func() (err error) {
			span := sentry.StartSpan(span.Context(), "Is Blacklisted")
			defer span.Finish()

			isBlacklisted, err = utils.IsBlacklisted(ctx, ctx.GuildId(), userId, member, permLevel)
			return
		})

		// load totalTickets
		group.Go(func() error {
			span := sentry.StartSpan(span.Context(), "GetAllByUser")
			defer span.Finish()

			tickets, err := dbclient.Client.Tickets.GetAllByUser(ctx, ctx.GuildId(), userId)
			totalTickets = len(tickets)
			return err
		})

		// load openTickets
		group.Go(func() error {
			span := sentry.StartSpan(span.Context(), "GetOpenByUser")
			defer span.Finish()

			tickets, err := dbclient.Client.Tickets.GetOpenByUser(ctx, ctx.GuildId(), userId)
			openTickets = len(tickets)
			return err
		})

		// load ticketLimit
		group.Go(func() (err error) {
			span := sentry.StartSpan(span.Context(), "TicketLimit")
			defer span.Finish()

			ticketLimit, err = dbclient.Client.TicketLimit.Get(ctx, ctx.GuildId())
			return
		})

		if err := group.Wait(); err != nil {
			ctx.HandleError(err)
			return
		}

		span := sentry.StartSpan(span.Context(), "Reply")

		msgEmbed := embed.NewEmbed().
			SetTitle("Statistics").
			SetColor(ctx.GetColour(customisation.Green)).
			SetAuthor(member.User.Username, "", member.User.AvatarUrl(256)).
			AddField("Permission Level", "Regular", true).
			AddField("Is Blacklisted", strconv.FormatBool(isBlacklisted), true).
			AddBlankField(true).
			AddField("Total Tickets", strconv.Itoa(totalTickets), true).
			AddField("Open Tickets", fmt.Sprintf("%d / %d", openTickets, ticketLimit), true)

		_, _ = ctx.ReplyWith(command.NewEphemeralEmbedMessageResponse(msgEmbed))
		span.Finish()
	} else { // Support rep stats
		group, _ := errgroup.WithContext(ctx)

		var feedbackRating float32
		var feedbackCount int

		group.Go(func() (err error) {
			span := sentry.StartSpan(span.Context(), "GetAverageClaimedBy")
			defer span.Finish()

			feedbackRating, err = dbclient.Client.ServiceRatings.GetAverageClaimedBy(ctx, ctx.GuildId(), userId)
			return
		})

		group.Go(func() (err error) {
			span := sentry.StartSpan(span.Context(), "GetCountClaimedBy")
			defer span.Finish()

			feedbackCount, err = dbclient.Client.ServiceRatings.GetCountClaimedBy(ctx, ctx.GuildId(), userId)
			return
		})

		var weeklyAR, monthlyAR, totalAR *time.Duration
		var weeklyAnsweredTickets, monthlyAnsweredTickets, totalAnsweredTickets,
			weeklyTotalTickets, monthlyTotalTickets, totalTotalTickets,
			weeklyClaimedTickets, monthlyClaimedTickets, totalClaimedTickets int

		// totalAR
		group.Go(func() (err error) {
			span := sentry.StartSpan(span.Context(), "GetAverageAllTimeUser")
			defer span.Finish()

			totalAR, err = dbclient.Client.FirstResponseTime.GetAverageAllTimeUser(ctx, ctx.GuildId(), userId)
			return
		})

		// monthlyAR
		group.Go(func() (err error) {
			span := sentry.StartSpan(span.Context(), "GetAverageUser")
			defer span.Finish()

			monthlyAR, err = dbclient.Client.FirstResponseTime.GetAverageUser(ctx, ctx.GuildId(), userId, time.Hour*24*28)
			return
		})

		// weeklyAR
		group.Go(func() (err error) {
			span := sentry.StartSpan(span.Context(), "GetAverageUser")
			defer span.Finish()

			weeklyAR, err = dbclient.Client.FirstResponseTime.GetAverageUser(ctx, ctx.GuildId(), userId, time.Hour*24*7)
			return
		})

		// weeklyAnswered
		group.Go(func() (err error) {
			span := sentry.StartSpan(span.Context(), "GetParticipatedCountInterval")
			defer span.Finish()

			weeklyAnsweredTickets, err = dbclient.Client.Participants.GetParticipatedCountInterval(ctx, ctx.GuildId(), userId, time.Hour*24*7)
			return
		})

		// monthlyAnswered
		group.Go(func() (err error) {
			span := sentry.StartSpan(span.Context(), "GetParticipatedCountInterval")
			defer span.Finish()

			monthlyAnsweredTickets, err = dbclient.Client.Participants.GetParticipatedCountInterval(ctx, ctx.GuildId(), userId, time.Hour*24*28)
			return
		})

		// totalAnswered
		group.Go(func() (err error) {
			span := sentry.StartSpan(span.Context(), "GetParticipatedCount")
			defer span.Finish()

			totalAnsweredTickets, err = dbclient.Client.Participants.GetParticipatedCount(ctx, ctx.GuildId(), userId)
			return
		})

		// weeklyTotal
		group.Go(func() (err error) {
			span := sentry.StartSpan(span.Context(), "GetTotalTicketCountInterval")
			defer span.Finish()

			weeklyTotalTickets, err = dbclient.Client.Tickets.GetTotalTicketCountInterval(ctx, ctx.GuildId(), time.Hour*24*7)
			return
		})

		// monthlyTotal
		group.Go(func() (err error) {
			span := sentry.StartSpan(span.Context(), "GetTotalTicketCountInterval")
			defer span.Finish()

			monthlyTotalTickets, err = dbclient.Client.Tickets.GetTotalTicketCountInterval(ctx, ctx.GuildId(), time.Hour*24*28)
			return
		})

		// totalTotal
		group.Go(func() (err error) {
			span := sentry.StartSpan(span.Context(), "GetTotalTicketCount")
			defer span.Finish()

			totalTotalTickets, err = dbclient.Client.Tickets.GetTotalTicketCount(ctx, ctx.GuildId())
			return
		})

		// weeklyClaimed
		group.Go(func() (err error) {
			span := sentry.StartSpan(span.Context(), "GetClaimedSinceCount_Weekly")
			defer span.Finish()

			weeklyClaimedTickets, err = dbclient.Client.TicketClaims.GetClaimedSinceCount(ctx, ctx.GuildId(), userId, time.Hour*24*7)
			return
		})

		// monthlyClaimed
		group.Go(func() (err error) {
			span := sentry.StartSpan(span.Context(), "GetClaimedSinceCount_Monthly")
			defer span.Finish()

			monthlyClaimedTickets, err = dbclient.Client.TicketClaims.GetClaimedSinceCount(ctx, ctx.GuildId(), userId, time.Hour*24*28)
			return
		})

		// totalClaimed
		group.Go(func() (err error) {
			span := sentry.StartSpan(span.Context(), "GetClaimedCount")
			defer span.Finish()

			totalClaimedTickets, err = dbclient.Client.TicketClaims.GetClaimedCount(ctx, ctx.GuildId(), userId)
			return
		})

		if err := group.Wait(); err != nil {
			ctx.HandleError(err)
			return
		}

		var permissionLevel string
		if permLevel == permission.Admin {
			permissionLevel = "Admin"
		} else {
			permissionLevel = "Support"
		}

		span := sentry.StartSpan(span.Context(), "Reply")

		if experiments.HasFeature(ctx, ctx.GuildId(), experiments.COMPONENTS_V2_STATISTICS) {
			userData, err := ctx.Worker().GetUser(userId)
			if err != nil {
				ctx.HandleError(err)
				return
			}

			mainStats := []string{
				fmt.Sprintf("**Username**: %s", userData.Username),
				fmt.Sprintf("**Permission Level**: %s", permissionLevel),
				fmt.Sprintf("**Feedback Rating**: %.1f / 5 ★", feedbackRating),
				fmt.Sprintf("**Feedback Count**: %d", feedbackCount),
			}

			responseTimeStats := []string{
				fmt.Sprintf("**Total**: %s", formatNullableTime(totalAR)),
				fmt.Sprintf("**Monthly**: %s", formatNullableTime(monthlyAR)),
				fmt.Sprintf("**Weekly**: %s", formatNullableTime(weeklyAR)),
			}

			ticketsAnsweredStats := []string{
				fmt.Sprintf("**Total**: %d/%d", totalAnsweredTickets, totalTotalTickets),
				fmt.Sprintf("**Monthly**: %d/%d", monthlyAnsweredTickets, monthlyTotalTickets),
				fmt.Sprintf("**Weekly**: %d/%d", weeklyAnsweredTickets, weeklyTotalTickets),
			}

			claimedStats := []string{
				fmt.Sprintf("**Total**: %d", totalClaimedTickets),
				fmt.Sprintf("**Monthly**: %d", monthlyClaimedTickets),
				fmt.Sprintf("**Weekly**: %d", weeklyClaimedTickets),
			}

			innerComponents := []component.Component{
				component.BuildSection(component.Section{
					Accessory: component.BuildThumbnail(component.Thumbnail{
						Media: component.UnfurledMediaItem{
							Url: userData.AvatarUrl(256),
						},
					}),
					Components: []component.Component{
						component.BuildTextDisplay(component.TextDisplay{Content: "## Ticket User Statistics"}),
						component.BuildTextDisplay(component.TextDisplay{
							Content: fmt.Sprintf("● %s", strings.Join(mainStats, "\n● ")),
						}),
					},
				}),
				component.BuildSeparator(component.Separator{}),
				component.BuildTextDisplay(component.TextDisplay{
					Content: fmt.Sprintf("### Average Response Time\n● %s", strings.Join(responseTimeStats, "\n● ")),
				}),
				component.BuildSeparator(component.Separator{}),
				component.BuildTextDisplay(component.TextDisplay{
					Content: fmt.Sprintf(
						"### Tickets Answered\n● %s",
						strings.Join(ticketsAnsweredStats, "\n● "),
					),
				}),
				component.BuildSeparator(component.Separator{}),
				component.BuildTextDisplay(component.TextDisplay{
					Content: fmt.Sprintf(
						"### Claimed Tickets\n● %s",
						strings.Join(claimedStats, "\n● "),
					),
				}),
			}

			ctx.ReplyWith(command.NewEphemeralMessageResponseWithComponents(utils.Slice(component.BuildContainer(component.Container{
				Components: innerComponents,
			}))))
		} else {
			msgEmbed := embed.NewEmbed().
				SetTitle("Statistics").
				SetColor(ctx.GetColour(customisation.Green)).
				SetAuthor(member.User.Username, "", member.User.AvatarUrl(256)).
				AddField("Permission Level", permissionLevel, true).
				AddField("Feedback Rating", fmt.Sprintf("%.1f / 5 ⭐ (%d ratings)", feedbackRating, feedbackCount), true).
				AddBlankField(true).
				AddField("Average First Response Time (Weekly)", formatNullableTime(weeklyAR), true).
				AddField("Average First Response Time (Monthly)", formatNullableTime(monthlyAR), true).
				AddField("Average First Response Time (Total)", formatNullableTime(totalAR), true).
				AddField("Tickets Answered (Weekly)", fmt.Sprintf("%d / %d", weeklyAnsweredTickets, weeklyTotalTickets), true).
				AddField("Tickets Answered (Monthly)", fmt.Sprintf("%d / %d", monthlyAnsweredTickets, monthlyTotalTickets), true).
				AddField("Tickets Answered (Total)", fmt.Sprintf("%d / %d", totalAnsweredTickets, totalTotalTickets), true).
				AddField("Claimed Tickets (Weekly)", strconv.Itoa(weeklyClaimedTickets), true).
				AddField("Claimed Tickets (Monthly)", strconv.Itoa(monthlyClaimedTickets), true).
				AddField("Claimed Tickets (Total)", strconv.Itoa(totalClaimedTickets), true)

			_, _ = ctx.ReplyWith(command.NewEphemeralEmbedMessageResponse(msgEmbed))
		}

		span.Finish()
	}
}
