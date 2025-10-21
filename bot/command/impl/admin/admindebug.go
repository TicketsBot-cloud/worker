package admin

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/common/premium"
	"github.com/TicketsBot-cloud/gdl/objects/application"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/gdl/rest"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/experiments"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type AdminDebugCommand struct{}

func (AdminDebugCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:            "debug",
		Description:     "Debug command for a guild",
		Type:            interaction.ApplicationCommandTypeChatInput,
		PermissionLevel: permission.Everyone,
		Category:        command.Settings,
		HelperOnly:      true,
		Arguments: command.Arguments(
			command.NewRequiredArgument("guild_id", "ID of the guild", interaction.OptionTypeString, i18n.MessageInvalidArgument),
		),
		Timeout: time.Second * 10,
	}
}

func (c AdminDebugCommand) GetExecutor() interface{} {
	return c.Execute
}

func (AdminDebugCommand) Execute(ctx registry.CommandContext, raw string) {
	guildId, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	guild, err := ctx.Worker().GetGuild(guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	settings, err := dbclient.Client.Settings.Get(ctx, guild.Id)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	owner, err := ctx.Worker().GetUser(guild.OwnerId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	tier, source, err := utils.PremiumClient.GetTierByGuild(ctx, guild)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	panels, err := dbclient.Client.Panel.GetByGuild(ctx, guild.Id)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	integrations, err := dbclient.Client.CustomIntegrationGuilds.GetGuildIntegrations(ctx, guild.Id)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	importLogs, err := dbclient.Client.ImportLogs.GetRuns(ctx, guild.Id)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	botId, botFound, err := dbclient.Client.WhitelabelGuilds.GetBotByGuild(ctx, guild.Id)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	featuresEnabled := []string{}

	for i := range experiments.List {
		feature := experiments.List[i]
		if experiments.HasFeature(ctx, guild.Id, feature) {
			featuresEnabled = append(featuresEnabled, string(feature))
		}
	}

	var bInf application.Application

	if botFound {
		botDbInfo, err := dbclient.Client.Whitelabel.GetByBotId(ctx, botId)
		if err != nil {
			ctx.HandleError(err)
			return
		}

		botInfo, err := rest.GetCurrentApplication(ctx, botDbInfo.Token, nil)
		if err != nil {
			ctx.HandleError(err)
			return
		}

		bInf = botInfo
	}

	// Helper to get ticket notification channel info
	getTicketNotifChannel := func() (string, string) {
		if settings.UseThreads && settings.TicketNotificationChannel != nil {
			ch, err := ctx.Worker().GetChannel(*settings.TicketNotificationChannel)
			if err == nil {
				return ch.Name, strconv.FormatUint(ch.Id, 10)
			}
		}
		return "Disabled", "Disabled"
	}

	ticketNotifChannelName, ticketNotifChannelId := getTicketNotifChannel()

	ownerId := strconv.FormatUint(owner.Id, 10)
	ownerName := owner.Username

	panelLimit := "3"
	premiumTier := "None"
	premiumSource := "None"
	if tier != premium.None {
		premiumTier = tier.String()
		premiumSource = string(source)
		panelLimit = "∞"
	}

	panelCount := len(panels)

	guildInfo := []string{
		fmt.Sprintf("ID: `%d`", guild.Id),
		fmt.Sprintf("Name: `%s`", guild.Name),
		fmt.Sprintf("Owner: `%s` (%s)", ownerName, ownerId),
	}
	if guild.VanityUrlCode != "" {
		guildInfo = append(guildInfo, fmt.Sprintf("Vanity URL: `.gg/%s`", guild.VanityUrlCode))
	}
	if tier != premium.None {
		guildInfo = append(guildInfo, fmt.Sprintf("Premium Tier: `%s`", premiumTier))
		guildInfo = append(guildInfo, fmt.Sprintf("Premium Source: `%s`", premiumSource))
	}

	if botFound {
		guildInfo = append(guildInfo, fmt.Sprintf("Whitelabel Bot: `%s` (%d)", bInf.Name, botId))
	}

	settingsInfo := []string{
		fmt.Sprintf("Transcripts Enabled: `%t`", settings.StoreTranscripts),
		fmt.Sprintf("Panel Count: `%d/%s`", panelCount, panelLimit),
	}
	if settings.UseThreads {
		settingsInfo = append(settingsInfo, fmt.Sprintf("Thread Mode: `%t`", settings.UseThreads))
		settingsInfo = append(settingsInfo, fmt.Sprintf("Thread Mode Channel: `#%s` (%s)", ticketNotifChannelName, ticketNotifChannelId))
	}

	if len(integrations) > 0 {
		enabledIntegrations := make([]string, len(integrations))
		for i, integ := range integrations {
			enabledIntegrations[i] = integ.Name
		}
		settingsInfo = append(settingsInfo, fmt.Sprintf("Enabled Integrations: %d (%s)", len(enabledIntegrations), strings.Join(enabledIntegrations, ", ")))
	}

	hasDataRun, hasTranscriptRun := false, false
	for _, log := range importLogs {
		switch log.RunType {
		case "DATA":
			hasDataRun = true
		case "TRANSCRIPT":
			hasTranscriptRun = true
		}
	}
	settingsInfo = append(settingsInfo, fmt.Sprintf("Data Imported: `%t`", hasDataRun))
	settingsInfo = append(settingsInfo, fmt.Sprintf("Transcripts Imported: `%t`", hasTranscriptRun))

	debugResponse := []string{
		fmt.Sprintf("**Guild Info**\n- %s", strings.Join(guildInfo, "\n- ")),
		fmt.Sprintf("**Settings**\n- %s", strings.Join(settingsInfo, "\n- ")),
	}

	if len(featuresEnabled) > 0 {
		debugResponse = append(debugResponse, fmt.Sprintf("**Experiments Enabled**\n- %s", strings.Join(featuresEnabled, "\n- ")))
	}

	ctx.ReplyWith(command.NewEphemeralMessageResponseWithComponents([]component.Component{
		utils.BuildContainerRaw(
			ctx,
			customisation.Orange,
			"Admin - Debug",
			strings.Join(debugResponse, "\n\n"),
		),
		component.BuildActionRow(component.BuildButton(component.Button{
			Label:    "Recache Guild",
			Style:    component.ButtonStylePrimary,
			CustomId: fmt.Sprintf("admin_debug_recache_%d", guild.Id),
		})),
	}))
}
