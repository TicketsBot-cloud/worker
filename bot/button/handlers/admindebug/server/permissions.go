package server

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	permcache "github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/button"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
)

type AdminDebugServerPermissionsHandler struct{}

func (h *AdminDebugServerPermissionsHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "admin_debug_permissions")
	})
}

func (h *AdminDebugServerPermissionsHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:           registry.SumFlags(registry.GuildAllowed, registry.CanEdit),
		Timeout:         time.Second * 15,
		PermissionLevel: permcache.Support,
		HelperOnly:      true,
	}
}

func (h *AdminDebugServerPermissionsHandler) Execute(ctx *context.ButtonContext) {
	guildId, err := strconv.ParseUint(strings.Replace(ctx.InteractionData.CustomId, "admin_debug_permissions_", "", -1), 10, 64)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Get panels to build dynamic options
	panels, err := dbclient.Client.Panel.GetByGuild(ctx, guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Build select menu options
	serverWideDesc := "Check general server permissions"
	options := []component.SelectOption{
		{
			Label:       "Server Permissions",
			Value:       fmt.Sprintf("server_%d", guildId),
			Description: &serverWideDesc,
		},
	}

	for i, panel := range panels {
		if i >= 24 { // Discord limit is 25 options total (1 for server-wide + 24 panels)
			break
		}
		panelDesc := fmt.Sprintf("Check permissions for panel #%d", i+1)
		options = append(options, component.SelectOption{
			Label:       fmt.Sprintf("Panel: %s", panel.Title),
			Value:       fmt.Sprintf("panel_%d_%d", panel.MessageId, guildId),
			Description: &panelDesc,
		})
	}

	if len(options) > 25 {
		options = options[:25]
	}

	minValues := 1
	maxValues := len(options)

	selectMenu := component.BuildSelectMenu(component.SelectMenu{
		CustomId:  fmt.Sprintf("admin_debug_permissions_select_%d", guildId),
		Options:   options,
		MinValues: &minValues,
		MaxValues: &maxValues,
	})

	label := component.BuildLabel(component.Label{
		Label:     "Select Locations",
		Component: selectMenu,
	})

	ctx.Modal(button.ResponseModal{
		Data: interaction.ModalResponseData{
			CustomId:   fmt.Sprintf("admin_debug_permissions_modal_%d", guildId),
			Title:      "Bot Permission Check",
			Components: []component.Component{label},
		},
	})
}
