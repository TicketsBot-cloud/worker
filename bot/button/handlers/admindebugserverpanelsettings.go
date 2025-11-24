package handlers

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
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/utils"
)

type AdminDebugServerPanelSettingsHandler struct{}

func (h *AdminDebugServerPanelSettingsHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "admin_debug_panel_settings")
	})
}

func (h *AdminDebugServerPanelSettingsHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:           registry.SumFlags(registry.GuildAllowed, registry.CanEdit),
		Timeout:         time.Second * 15,
		PermissionLevel: permcache.Support,
	}
}

func (h *AdminDebugServerPanelSettingsHandler) Execute(ctx *context.ButtonContext) {
	if !utils.IsBotHelper(ctx.UserId()) {
		ctx.ReplyRaw(customisation.Red, "Error", "You do not have permission to use this button.")
		return
	}

	guildId, err := strconv.ParseUint(strings.Replace(ctx.InteractionData.CustomId, "admin_debug_panel_settings_", "", -1), 10, 64)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	panels, err := dbclient.Client.Panel.GetByGuild(ctx, guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	options := []component.SelectOption{}

	for i, panel := range panels {
		if i >= 25 { // Discord limit is 25 options
			break
		}
		panelDesc := fmt.Sprintf("View settings for panel #%d", i+1)
		options = append(options, component.SelectOption{
			Label:       fmt.Sprintf("Panel: %s", panel.Title),
			Value:       fmt.Sprintf("panel_%d", panel.MessageId),
			Description: &panelDesc,
		})
	}

	if len(options) > 25 {
		options = options[:25]
	}

	minValues := 1
	maxValues := len(options)

	selectMenu := component.BuildSelectMenu(component.SelectMenu{
		CustomId:  fmt.Sprintf("admin_debug_panel_settings_select_%d", guildId),
		Options:   options,
		MinValues: &minValues,
		MaxValues: &maxValues,
	})

	label := component.BuildLabel(component.Label{
		Label:     "Select Panels",
		Component: selectMenu,
	})

	ctx.Modal(button.ResponseModal{
		Data: interaction.ModalResponseData{
			CustomId:   fmt.Sprintf("admin_debug_panel_settings_modal_%d", guildId),
			Title:      "Panel Settings",
			Components: []component.Component{label},
		},
	})
}
