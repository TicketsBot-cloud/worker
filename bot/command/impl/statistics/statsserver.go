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

	var (
		totalTickets, openTickets uint64
		feedbackRating            float64
		feedbackCount             uint64
		firstResponseTime         analytics.TripleWindow
		ticketDuration            analytics.TripleWindow
		ticketVolumeData          []analytics.CountOnDate
	)

	group, _ := errgroup.WithContext(ctx)

	group.Go(func() (err error) {
		span := sentry.StartSpan(span.Context(), "GetTotalTicketCount")
		defer span.Finish()

		totalTickets, err = dbclient.Analytics.GetTotalTicketCount(ctx, ctx.GuildId())
		return
	})

	// open tickets
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
	group.Go(func() (err error) {
		span := sentry.StartSpan(span.Context(), "GetFirstResponseTimeStats")
		defer span.Finish()

		firstResponseTime, err = dbclient.Analytics.GetFirstResponseTimeStats(ctx, ctx.GuildId())
		return
	})

	// ticket duration
	group.Go(func() (err error) {
		span := sentry.StartSpan(span.Context(), "GetTicketDurationStats")
		defer span.Finish()

		ticketDuration, err = dbclient.Analytics.GetTicketDurationStats(ctx, ctx.GuildId())
		return
	})

	// tickets per day
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
	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	tw.Style().Format.Header = text.FormatDefault

	tw.AppendHeader(table.Row{"Date", "Ticket Volume"})
	for _, count := range ticketVolumeData {
		tw.AppendRow(table.Row{count.Date.Format("2006-01-02"), count.Count})
	}

	ticketVolumeTable := tw.Render()

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
			Content: fmt.Sprintf("### Average Ticket Duration\n● %s", strings.Join(ticketDurationStats, "\n● ")),
		}),
		component.BuildSeparator(component.Separator{}),
		component.BuildTextDisplay(component.TextDisplay{
			Content: fmt.Sprintf(
				"### Ticket Volume\n```\n%s\n```",
				ticketVolumeTable,
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
