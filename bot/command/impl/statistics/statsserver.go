package statistics

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
	"github.com/getsentry/sentry-go"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"golang.org/x/sync/errgroup"
)

type StatsServerCommand struct {
}

func (StatsServerCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:             "server",
		Description:      i18n.HelpStatsServer,
		Type:             interaction.ApplicationCommandTypeChatInput,
		PermissionLevel:  permission.Support,
		Category:         command.Statistics,
		PremiumOnly:      true,
		DefaultEphemeral: true,
		Timeout:          time.Second * 10,
	}
}

func (c StatsServerCommand) GetExecutor() interface{} {
	return c.Execute
}

func (StatsServerCommand) Execute(ctx registry.CommandContext) {
	span := sentry.StartTransaction(ctx, "/stats server")
	span.SetTag("guild", strconv.FormatUint(ctx.GuildId(), 10))
	defer span.Finish()

	group, _ := errgroup.WithContext(ctx)

	var totalTickets, openTickets uint64

	// totalTickets
	group.Go(func() error {
		span := sentry.StartSpan(span.Context(), "GetTotalTicketCount")
		defer span.Finish()

		count, err := dbclient.Client.Tickets.GetTotalTicketCount(ctx, ctx.GuildId())
		if err != nil {
			return err
		}
		totalTickets = uint64(count)
		return nil
	})

	// openTickets
	group.Go(func() error {
		span := sentry.StartSpan(span.Context(), "GetGuildOpenTickets")
		defer span.Finish()

		tickets, err := dbclient.Client.Tickets.GetGuildOpenTickets(ctx, ctx.GuildId())
		if err != nil {
			return err
		}

		openTickets = uint64(len(tickets))
		return nil
	})

	var feedbackRating float64
	var feedbackCount uint64

	group.Go(func() error {
		span := sentry.StartSpan(span.Context(), "GetAverageFeedbackRating")
		defer span.Finish()

		avg, err := dbclient.Client.ServiceRatings.GetAverage(ctx, ctx.GuildId())
		if err != nil {
			return err
		}
		feedbackRating = float64(avg)
		return nil
	})

	group.Go(func() error {
		span := sentry.StartSpan(span.Context(), "GetFeedbackCount")
		defer span.Finish()

		count, err := dbclient.Client.ServiceRatings.GetCount(ctx, ctx.GuildId())
		if err != nil {
			return err
		}
		feedbackCount = uint64(count)
		return nil
	})

	var firstResponseTime database.TripleWindow
	group.Go(func() (err error) {
		span := sentry.StartSpan(span.Context(), "GetFirstResponseTimeStats")
		defer span.Finish()

		firstResponseTime, err = dbclient.Client.FirstResponseTime.GetAverageTripleWindow(ctx, ctx.GuildId())
		return
	})

	var ticketDuration database.TripleWindow
	group.Go(func() (err error) {
		span := sentry.StartSpan(span.Context(), "GetTicketDurationStats")
		defer span.Finish()

		ticketDuration, err = dbclient.Client.Tickets.GetTicketDurationTripleWindow(ctx, ctx.GuildId())
		return
	})

	var ticketVolumeTable string
	group.Go(func() error {
		span := sentry.StartSpan(span.Context(), "GetLastNTicketsPerDayGuild")
		defer span.Finish()

		counts, err := dbclient.Client.Tickets.GetTicketsPerDay(ctx, ctx.GuildId(), 7)
		if err != nil {
			return err
		}

		tw := table.NewWriter()
		tw.SetStyle(table.StyleLight)
		tw.Style().Format.Header = text.FormatDefault

		tw.AppendHeader(table.Row{"Date", "Ticket Volume"})
		for _, count := range counts {
			tw.AppendRow(table.Row{count.Date.Format("2006-01-02"), count.Count})
		}

		ticketVolumeTable = tw.Render()
		return nil
	})

	var feedbackDist [5]int
	group.Go(func() (err error) {
		span := sentry.StartSpan(span.Context(), "GetFeedbackDistribution")
		defer span.Finish()

		feedbackDist, err = dbclient.Client.ServiceRatings.GetDistribution(ctx, ctx.GuildId())
		return
	})

	var feedbackRate database.FeedbackResponseRate
	group.Go(func() (err error) {
		span := sentry.StartSpan(span.Context(), "GetFeedbackResponseRate")
		defer span.Finish()

		feedbackRate, err = dbclient.Client.ServiceRatings.GetResponseRate(ctx, ctx.GuildId(), 30)
		return
	})

	var autoCloseStats database.AutoCloseStats
	group.Go(func() (err error) {
		span := sentry.StartSpan(span.Context(), "GetAutoCloseVsManualClose")
		defer span.Finish()

		autoCloseStats, err = dbclient.Client.CloseReason.GetAutoCloseVsManualClose(ctx, ctx.GuildId(), 30)
		return
	})

	var threadSplit database.ThreadChannelSplit
	group.Go(func() (err error) {
		span := sentry.StartSpan(span.Context(), "GetThreadChannelSplit")
		defer span.Finish()

		threadSplit, err = dbclient.Client.Tickets.GetThreadChannelSplit(ctx, ctx.GuildId(), 30)
		return
	})

	var oneTouchResolution database.OneTouchResolution
	group.Go(func() (err error) {
		span := sentry.StartSpan(span.Context(), "GetOneTouchResolutionRate")
		defer span.Finish()

		oneTouchResolution, err = dbclient.Client.TicketMessageCounts.GetOneTouchResolutionRate(ctx, ctx.GuildId(), 30)
		return
	})

	var avgMessageCounts database.AverageMessageCounts
	group.Go(func() (err error) {
		span := sentry.StartSpan(span.Context(), "GetAverageMessageCounts")
		defer span.Finish()

		avgMessageCounts, err = dbclient.Client.TicketMessageCounts.GetAverageMessageCounts(ctx, ctx.GuildId(), 30)
		return
	})

	if err := group.Wait(); err != nil {
		ctx.HandleError(err)
		return
	}

	span = sentry.StartSpan(span.Context(), "Send Message")

	guildData, err := ctx.Guild()
	if err != nil {
		ctx.HandleError(err)
		return
	}

	mainStats := []string{
		fmt.Sprintf("**Total Tickets**: %d", totalTickets),
		fmt.Sprintf("**Open Tickets**: %d", openTickets),
		fmt.Sprintf("**Feedback Rating**: %.1f / 5 ★", feedbackRating),
		fmt.Sprintf("**Feedback Count**: %d", feedbackCount),
	}

	responseTimeStats := []string{
		fmt.Sprintf("**Total**: %s", formatNullableTime(firstResponseTime.AllTime)),
		fmt.Sprintf("**Monthly**: %s", formatNullableTime(firstResponseTime.Monthly)),
		fmt.Sprintf("**Weekly**: %s", formatNullableTime(firstResponseTime.Weekly)),
	}

	ticketDurationStats := []string{
		fmt.Sprintf("**Total**: %s", formatNullableTime(ticketDuration.AllTime)),
		fmt.Sprintf("**Monthly**: %s", formatNullableTime(ticketDuration.Monthly)),
		fmt.Sprintf("**Weekly**: %s", formatNullableTime(ticketDuration.Weekly)),
	}

	var topSection []component.Component

	iconUrl := guildData.IconUrl()
	if iconUrl == "" {
		topSection = []component.Component{
			component.BuildTextDisplay(component.TextDisplay{Content: "## Server Ticket Statistics"}),
			component.BuildTextDisplay(component.TextDisplay{
				Content: fmt.Sprintf("● %s", strings.Join(mainStats, "\n● ")),
			}),
		}
	} else {
		topSection = []component.Component{
			component.BuildSection(component.Section{
				Accessory: component.BuildThumbnail(component.Thumbnail{
					Media: component.UnfurledMediaItem{
						Url: iconUrl,
					},
				}),
				Components: []component.Component{
					component.BuildTextDisplay(component.TextDisplay{Content: "## Server Ticket Statistics"}),
					component.BuildTextDisplay(component.TextDisplay{
						Content: fmt.Sprintf("● %s", strings.Join(mainStats, "\n● ")),
					}),
				},
			}),
		}
	}

	autoCloseTotal := autoCloseStats.AutoClosed + autoCloseStats.ManualClosed
	var autoClosePct, manualClosePct float64
	if autoCloseTotal > 0 {
		autoClosePct = float64(autoCloseStats.AutoClosed) / float64(autoCloseTotal) * 100
		manualClosePct = float64(autoCloseStats.ManualClosed) / float64(autoCloseTotal) * 100
	}

	threadTotal := threadSplit.ThreadCount + threadSplit.ChannelCount
	var threadPct, channelPct float64
	if threadTotal > 0 {
		threadPct = float64(threadSplit.ThreadCount) / float64(threadTotal) * 100
		channelPct = float64(threadSplit.ChannelCount) / float64(threadTotal) * 100
	}

	feedbackDistStats := []string{
		fmt.Sprintf("**★1**: %d  **★2**: %d  **★3**: %d  **★4**: %d  **★5**: %d",
			feedbackDist[0], feedbackDist[1], feedbackDist[2], feedbackDist[3], feedbackDist[4]),
		fmt.Sprintf("**Response Rate**: %.0f%% (%d/%d tickets)", feedbackRate.Rate*100, feedbackRate.RatedTickets, feedbackRate.ClosedTickets),
	}

	var oneTouchPct float64
	if oneTouchResolution.TotalClosed > 0 {
		oneTouchPct = float64(oneTouchResolution.OneTouchCount) / float64(oneTouchResolution.TotalClosed) * 100
	}

	messageStats := []string{
		fmt.Sprintf("**One-Touch Resolution**: %.0f%% (%d/%d)", oneTouchPct, oneTouchResolution.OneTouchCount, oneTouchResolution.TotalClosed),
		fmt.Sprintf("**Avg Staff Messages**: %s", formatNullableFloat(avgMessageCounts.AvgStaffMessages)),
		fmt.Sprintf("**Avg User Messages**: %s", formatNullableFloat(avgMessageCounts.AvgUserMessages)),
		fmt.Sprintf("**Avg Total Messages**: %s", formatNullableFloat(avgMessageCounts.AvgTotalMessages)),
	}

	closureStats := []string{
		fmt.Sprintf("**Auto-closed**: %d (%.0f%%)", autoCloseStats.AutoClosed, autoClosePct),
		fmt.Sprintf("**Manual**: %d (%.0f%%)", autoCloseStats.ManualClosed, manualClosePct),
	}

	threadStats := []string{
		fmt.Sprintf("**Thread**: %d (%.0f%%)", threadSplit.ThreadCount, threadPct),
		fmt.Sprintf("**Channel**: %d (%.0f%%)", threadSplit.ChannelCount, channelPct),
	}

	innerComponents := append(topSection, []component.Component{
		component.BuildSeparator(component.Separator{}),
		component.BuildTextDisplay(component.TextDisplay{
			Content: fmt.Sprintf("### Average Response Time\n● %s", strings.Join(responseTimeStats, "\n● ")),
		}),
		component.BuildSeparator(component.Separator{}),
		component.BuildTextDisplay(component.TextDisplay{
			Content: fmt.Sprintf("### Average Ticket Duration\n● %s", strings.Join(ticketDurationStats, "\n● ")),
		}),
		component.BuildSeparator(component.Separator{}),
		component.BuildTextDisplay(component.TextDisplay{
			Content: fmt.Sprintf("### Feedback Distribution\n● %s", strings.Join(feedbackDistStats, "\n● ")),
		}),
		component.BuildSeparator(component.Separator{}),
		component.BuildTextDisplay(component.TextDisplay{
			Content: fmt.Sprintf("### Message Analytics\n● %s", strings.Join(messageStats, "\n● ")),
		}),
		component.BuildSeparator(component.Separator{}),
		component.BuildTextDisplay(component.TextDisplay{
			Content: fmt.Sprintf("### Closure Method\n● %s", strings.Join(closureStats, "\n● ")),
		}),
		component.BuildSeparator(component.Separator{}),
		component.BuildTextDisplay(component.TextDisplay{
			Content: fmt.Sprintf("### Thread / Channel Split\n● %s", strings.Join(threadStats, "\n● ")),
		}),
		component.BuildSeparator(component.Separator{}),
		component.BuildTextDisplay(component.TextDisplay{
			Content: fmt.Sprintf(
				"### Ticket Volume\n```\n%s\n```",
				ticketVolumeTable,
			),
		}),
	}...)

	ctx.ReplyWith(command.NewEphemeralMessageResponseWithComponents(utils.Slice(component.BuildContainer(component.Container{
		Components: innerComponents,
	}))))

	span.Finish()
}

func formatNullableTime(duration *time.Duration) string {
	return utils.FormatNullableTime(duration)
}

func formatNullableFloat(f *float64) string {
	if f == nil {
		return "N/A"
	}
	return fmt.Sprintf("%.1f", *f)
}
