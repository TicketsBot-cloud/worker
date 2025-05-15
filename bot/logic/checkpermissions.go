package logic

import (
	"fmt"
	"sync"

	"github.com/TicketsBot-cloud/common/premium"
	"github.com/TicketsBot-cloud/gdl/objects/channel/embed"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	permissiongdl "github.com/TicketsBot-cloud/gdl/permission"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/permissionwrapper"
	"github.com/TicketsBot-cloud/worker/config"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type PageData struct {
	Title  string
	Fields []embed.EmbedField
	Footer string
}

type CheckPermissionsState struct {
	Pages   []PageData
	Premium premium.PremiumTier
	Colour  int
	IconUrl string
}

var (
	checkPermsStateStore   = make(map[string]CheckPermissionsState)
	checkPermsStateMutex   sync.Mutex
	checkPermsStateCounter int
)

func SaveCheckPermissionsState(state CheckPermissionsState) (string, error) {
	checkPermsStateMutex.Lock()
	defer checkPermsStateMutex.Unlock()
	checkPermsStateCounter++
	id := fmt.Sprintf("%d", checkPermsStateCounter)
	checkPermsStateStore[id] = state
	return id, nil
}

func LoadCheckPermissionsState(id string) (CheckPermissionsState, error) {
	checkPermsStateMutex.Lock()
	defer checkPermsStateMutex.Unlock()
	state, ok := checkPermsStateStore[id]
	if !ok {
		return CheckPermissionsState{}, fmt.Errorf("not found")
	}
	return state, nil
}

func BuildCheckPermissionsEmbed(state CheckPermissionsState, pageIdx int) *embed.Embed {
	page := state.Pages[pageIdx]
	e := embed.NewEmbed().
		SetTitle(page.Title).
		SetColor(state.Colour)
	for _, f := range page.Fields {
		e.AddField(f.Name, f.Value, false)
	}
	if state.Premium == premium.None {
		e.SetFooter(page.Footer, state.IconUrl)
	}
	return e
}

type PermissionCheck struct {
	Permission  permissiongdl.Permission
	Description i18n.MessageId
}

type PermissionResult struct {
	Name       string
	Id         uint64
	Missing    []PermissionCheck
	IsCategory bool
}

func BuildCheckPermissionsComponents(stateId string, pageIdx, totalPages int) []component.Component {
	if totalPages <= 1 {
		return nil
	}
	return []component.Component{
		component.BuildActionRow(
			component.BuildButton(component.Button{
				CustomId: fmt.Sprintf("checkperms_%s_%d_prev", stateId, pageIdx),
				Style:    component.ButtonStyleDanger,
				Label:    "<",
				Disabled: pageIdx == 0,
			}),
			component.BuildButton(component.Button{
				CustomId: "checkperms_page_count",
				Style:    component.ButtonStyleSecondary,
				Label:    fmt.Sprintf("%d/%d", pageIdx+1, totalPages),
				Disabled: true,
			}),
			component.BuildButton(component.Button{
				CustomId: fmt.Sprintf("checkperms_%s_%d_next", stateId, pageIdx),
				Style:    component.ButtonStyleSuccess,
				Label:    ">",
				Disabled: pageIdx == totalPages-1,
			}),
		),
	}
}

func BuildCheckPermissions(ctx registry.CommandContext) error {
	worker := ctx.Worker()
	guildId := ctx.GuildId()
	botId := worker.BotId

	// Check for Administrator permission
	if permissionwrapper.HasPermissions(worker, guildId, botId, permissiongdl.Administrator) {
		ctx.Reply(customisation.Green, i18n.PermissionsTitle, i18n.PermissionsHasAdministrator, botId)
		return nil
	}

	// All permissions that are checked for the bot
	allPermissions := []PermissionCheck{
		{permissiongdl.ViewChannel, i18n.PermissionsReadMessages},
		{permissiongdl.SendMessages, i18n.PermissionsSendMessages},
		{permissiongdl.EmbedLinks, i18n.PermissionsEmbedLinks},
		{permissiongdl.AttachFiles, i18n.PermissionsAttachFiles},
		{permissiongdl.AddReactions, i18n.PermissionsAddReactions},
		{permissiongdl.UseExternalEmojis, i18n.PermissionsUseExternalEmojis},
		{permissiongdl.MentionEveryone, i18n.PermissionsMentionEveryone},
		{permissiongdl.ReadMessageHistory, i18n.PermissionsReadMessageHistory},
		{permissiongdl.ManageChannels, i18n.PermissionsManageChannels},
		{permissiongdl.ManageRoles, i18n.PermissionsManageRoles},
		{permissiongdl.ManageWebhooks, i18n.PermissionsManageWebhooks},
		{permissiongdl.ManageThreads, i18n.PermissionsManageThreads},
		{permissiongdl.SendMessagesInThreads, i18n.PermissionsSendMessagesInThreads},
		{permissiongdl.CreatePublicThreads, i18n.PermissionsCreatePublicThreads},
		{permissiongdl.CreatePrivateThreads, i18n.PermissionsCreatePrivateThreads},
		{permissiongdl.UseApplicationCommands, i18n.PermissionsUseApplicationCommands},
	}
	// For ticket and overflow categories
	categoryPermissions := []PermissionCheck{
		{permissiongdl.ViewChannel, i18n.PermissionsReadMessages},
		{permissiongdl.ManageChannels, i18n.PermissionsManageChannels},
		{permissiongdl.EmbedLinks, i18n.PermissionsEmbedLinks},
		{permissiongdl.AttachFiles, i18n.PermissionsAttachFiles},
	}
	// For transcript channels
	transcriptChannelPermissions := []PermissionCheck{
		{permissiongdl.ViewChannel, i18n.PermissionsReadMessages},
		{permissiongdl.SendMessages, i18n.PermissionsSendMessages},
		{permissiongdl.EmbedLinks, i18n.PermissionsEmbedLinks},
		{permissiongdl.AttachFiles, i18n.PermissionsAttachFiles},
	}
	// For notification channels
	notificationChannelPermissions := []PermissionCheck{
		{permissiongdl.ViewChannel, i18n.PermissionsReadMessages},
		{permissiongdl.SendMessages, i18n.PermissionsSendMessages},
		{permissiongdl.EmbedLinks, i18n.PermissionsEmbedLinks},
		{permissiongdl.AttachFiles, i18n.PermissionsAttachFiles},
	}

	settings, err := ctx.Settings()
	if err != nil {
		return err
	}

	checkedIds := make(map[uint64]struct{})

	var pages []PageData

	// Check bot permissions (Guild-wide)
	missingBotPermissions := []PermissionCheck{}
	for _, check := range allPermissions {
		if !permissionwrapper.HasPermissions(worker, guildId, botId, check.Permission) {
			missingBotPermissions = append(missingBotPermissions, check)
		}
	}
	if len(missingBotPermissions) > 0 {
		guildFields := []embed.EmbedField{}
		for _, missing := range missingBotPermissions {
			permName := missing.Permission.String()
			if missing.Permission == permissiongdl.ViewChannel {
				permName = "View Channels"
			}
			guildFields = append(guildFields, embed.EmbedField{
				Name:  permName,
				Value: ctx.GetMessage(missing.Description),
			})
		}
		pages = append(pages, PageData{
			Title:  ctx.GetMessage(i18n.PermissionsMissing) + " - Server-wide",
			Fields: guildFields,
		})
	}

	// Check bot permissions (Channel and Category)
	checkPerms := func(id uint64, isCategory bool, name string, perms []PermissionCheck, pageType string) {
		if _, checked := checkedIds[id]; checked {
			return
		}
		checkedIds[id] = struct{}{}
		missing := []PermissionCheck{}
		for _, check := range perms {
			if !permissionwrapper.HasPermissionsChannel(worker, guildId, botId, id, check.Permission) {
				missing = append(missing, check)
			}
		}
		if len(missing) > 0 {
			fields := []embed.EmbedField{}
			for _, miss := range missing {
				permName := miss.Permission.String()
				if miss.Permission == permissiongdl.ViewChannel {
					permName = "View Channels"
				}
				fields = append(fields, embed.EmbedField{
					Name:  permName,
					Value: ctx.GetMessage(miss.Description),
				})
			}
			title := fmt.Sprintf("%s (%s)", name, pageType)
			pages = append(pages, PageData{
				Title:  ctx.GetMessage(i18n.PermissionsMissing) + " - " + title,
				Fields: fields,
			})
		}
	}

	// Check ticket category
	if categoryId, err := dbclient.Client.ChannelCategory.Get(ctx, guildId); err == nil && categoryId != 0 && !settings.UseThreads {
		ch, err := worker.GetChannel(categoryId)
		if err != nil {
			return err
		}
		checkPerms(categoryId, true, ch.Name, categoryPermissions, "Ticket Category")
	}

	// Check notification channel
	if settings.UseThreads && settings.TicketNotificationChannel != nil {
		channel, err := worker.GetChannel(*settings.TicketNotificationChannel)
		if err != nil {
			return err
		}
		checkPerms(*settings.TicketNotificationChannel, false, channel.Name, notificationChannelPermissions, "Notification Channel")
	}

	// Check transcript channel
	if archiveId, err := dbclient.Client.ArchiveChannel.Get(ctx, guildId); err == nil && archiveId != nil {
		channel, err := worker.GetChannel(*archiveId)
		if err != nil {
			return err
		}
		checkPerms(*archiveId, false, channel.Name, transcriptChannelPermissions, "Transcript Channel")
	}

	// Check overflow category
	if settings.OverflowEnabled && settings.OverflowCategoryId != nil {
		channel, err := worker.GetChannel(*settings.OverflowCategoryId)
		if err != nil {
			return err
		}
		checkPerms(*settings.OverflowCategoryId, true, channel.Name, categoryPermissions, "Overflow Category")
	}

	if len(pages) == 0 {
		ctx.Reply(customisation.Green, i18n.PermissionsTitle, i18n.PermissionsHasAll, botId)
		return nil
	}

	for i := range pages {
		if ctx.PremiumTier() == premium.None {
			pages[i].Footer = fmt.Sprintf("Powered by %s", config.Conf.Bot.PoweredBy)
		}
	}

	state := CheckPermissionsState{
		Pages:   pages,
		Premium: ctx.PremiumTier(),
		Colour:  ctx.GetColour(customisation.Red),
		IconUrl: config.Conf.Bot.IconUrl,
	}
	// Save state and get a state ID
	stateId, err := SaveCheckPermissionsState(state)
	if err != nil {
		return err
	}

	// Initial page index
	pageIdx := 0
	pageEmbed := BuildCheckPermissionsEmbed(state, pageIdx)

	// Only show navigation if more than 1 page
	components := []component.Component{}
	if len(pages) > 1 {
		components = BuildCheckPermissionsComponents(stateId, pageIdx, len(pages))
	}

	res := command.MessageResponse{
		Embeds:     []*embed.Embed{pageEmbed},
		Components: components,
	}

	if _, err := ctx.ReplyWith(res); err != nil {
		return err
	}
	return nil
}
