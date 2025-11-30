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
)

type AdminDebugServerTicketPermissionsHandler struct{}

func (h *AdminDebugServerTicketPermissionsHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "admin_debug_user_permissions")
	})
}

func (h *AdminDebugServerTicketPermissionsHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:           registry.SumFlags(registry.GuildAllowed, registry.CanEdit),
		Timeout:         time.Second * 15,
		PermissionLevel: permcache.Support,
		HelperOnly:      true,
	}
}

func (h *AdminDebugServerTicketPermissionsHandler) Execute(ctx *context.ButtonContext) {
	guildId, err := strconv.ParseUint(strings.Replace(ctx.InteractionData.CustomId, "admin_debug_user_permissions_", "", -1), 10, 64)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	textInputLabel := "User/Role IDs (comma separated)"
	textInputRequired := true
	textInputPlaceholder := "1325579039888511056,1328106570965585951"

	textInput := component.BuildInputText(component.InputText{
		CustomId:    "user_ids",
		Style:       component.TextStyleParagraph,
		Label:       &textInputLabel,
		Required:    &textInputRequired,
		Placeholder: &textInputPlaceholder,
	})

	ctx.Modal(button.ResponseModal{
		Data: interaction.ModalResponseData{
			CustomId: fmt.Sprintf("admin_debug_user_permissions_modal_%d", guildId),
			Title:    "Staff Permission Check",
			Components: []component.Component{
				component.BuildActionRow(textInput),
			},
		},
	})
}
