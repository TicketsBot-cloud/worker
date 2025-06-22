package statistics

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/TicketsBot-cloud/analytics-client"
	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
	"github.com/getsentry/sentry-go"
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
		PremiumOnly:      false,
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

	group.Go(func() (err error) {
		span := sentry.StartSpan(span.Context(), "GetTotalTicketCount")
		defer span.Finish()

		totalTickets, err = dbclient.Analytics.GetTotalTicketCount(ctx, ctx.GuildId())
		return
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

	group.Go(func() (err error) {
		span := sentry.StartSpan(span.Context(), "GetAverageFeedbackRating")
		defer span.Finish()

		feedbackRating, err = dbclient.Analytics.GetAverageFeedbackRatingGuild(ctx, ctx.GuildId())
		return
	})

	group.Go(func() (err error) {
		span := sentry.StartSpan(span.Context(), "GetFeedbackCount")
		defer span.Finish()

		feedbackCount, err = dbclient.Analytics.GetFeedbackCountGuild(ctx, ctx.GuildId())
		return
	})

	// first response times
	var firstResponseTime analytics.TripleWindow
	group.Go(func() (err error) {
		span := sentry.StartSpan(span.Context(), "GetFirstResponseTimeStats")
		defer span.Finish()

		firstResponseTime, err = dbclient.Analytics.GetFirstResponseTimeStats(ctx, ctx.GuildId())
		return
	})

	// // ticket duration
	var ticketDuration analytics.TripleWindow
	group.Go(func() (err error) {
		span := sentry.StartSpan(span.Context(), "GetTicketDurationStats")
		defer span.Finish()

		ticketDuration, err = dbclient.Analytics.GetTicketDurationStats(ctx, ctx.GuildId())
		return
	})

	// tickets per day
	var ticketVolumeData []analytics.CountOnDate
	group.Go(func() error {
		span := sentry.StartSpan(span.Context(), "GetLastNTicketsPerDayGuild")
		defer span.Finish()

		counts, err := dbclient.Analytics.GetLastNTicketsPerDayGuild(ctx, ctx.GuildId(), 7)
		if err != nil {
			return err
		}

		ticketVolumeData = counts
		return nil
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

	spacers := "\u200e \u200e \u200e \u200e \u200e \u200e \u200e \u200e"
	last7DaysStats := make([]string, 0, len(ticketVolumeData))

	for _, tv := range ticketVolumeData {
		date := tv.Date.Format("2006-01-02")
		count := fmt.Sprintf("%d", tv.Count)
		extraPadding := ""
		switch {
		case tv.Count < 10:
			extraPadding = " \u200e \u200e"
		case tv.Count < 100:
			extraPadding = " \u200e"
		}
		last7DaysStats = append(last7DaysStats,
			fmt.Sprintf("`%s %s %s\u200e \u200e \u200e \u200e \u200e \u200e \u200e%s`", date, spacers, count, extraPadding),
		)
	}

	innerComponents := []component.Component{
		component.BuildSection(component.Section{
			Accessory: component.BuildThumbnail(component.Thumbnail{
				Media: component.UnfurledMediaItem{
					Url: guildData.IconUrl(),
				},
			}),
			Components: []component.Component{
				component.BuildTextDisplay(component.TextDisplay{Content: "## Server Ticket Statistics"}),
				component.BuildTextDisplay(component.TextDisplay{
					Content: fmt.Sprintf("` ● ` %s", strings.Join(mainStats, "\n` ● ` ")),
				}),
			},
		}),
		component.BuildSeparator(component.Separator{}),
		component.BuildTextDisplay(component.TextDisplay{
			Content: fmt.Sprintf("### Average Response Time\n` ● ` %s", strings.Join(responseTimeStats, "\n` ● ` ")),
		}),
		component.BuildSeparator(component.Separator{}),
		component.BuildTextDisplay(component.TextDisplay{
			Content: fmt.Sprintf("### Average Ticket Duration\n` ● ` %s", strings.Join(ticketDurationStats, "\n` ● ` ")),
		}),
		component.BuildSeparator(component.Separator{}),
		component.BuildTextDisplay(component.TextDisplay{
			Content: fmt.Sprintf(
				"### Ticket Volume\n__**\u200e \u200e \u200e \u200e \u200e \u200e \u200e Date \u200e \u200e \u200e \u200e \u200e \u200e \u200e | \u200e \u200e \u200e \u200e \u200eTicket Volume\u200e \u200e \u200e \u200e \u200e\u200e**__\n%s",
				strings.Join(last7DaysStats, "\n"),
			),
		}),
	}

	ctx.ReplyWith(command.NewEphemeralMessageResponseWithComponents(utils.Slice(component.BuildContainer(component.Container{
		Components: innerComponents,
	}))))

	span.Finish()
}

func formatNullableTime(duration *time.Duration) string {
	return utils.FormatNullableTime(duration)
}
