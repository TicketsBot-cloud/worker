package logic

import (
	"context"
	"fmt"
	"strings"

	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/worker/bot/blacklist"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/i18n"
)

// ValidatePanelAccess checks if the user can access the given panel.
// Returns (canProceed, outOfHoursWarning, error). outOfHoursWarning is non-nil
// when the panel is outside support hours but the behaviour is allow_with_warning.
func ValidatePanelAccess(ctx registry.InteractionContext, panel database.Panel) (bool, *string, error) {
	// Check support hours
	hasSupportHours, err := dbclient.Client.PanelSupportHours.HasSupportHours(ctx, panel.PanelId)
	if err != nil {
		return false, nil, err
	}

	if hasSupportHours {
		isActive, err := dbclient.Client.PanelSupportHours.IsActiveNow(ctx, panel.PanelId)
		if err != nil {
			return false, nil, err
		}

		if !isActive {
			// Fetch behaviour settings for this panel
			settings, exists, err := dbclient.Client.PanelSupportHoursSettings.Get(ctx, panel.PanelId)
			if err != nil {
				return false, nil, err
			}

			// Determine the warning/error message
			var outOfHoursMessage string
			if exists && settings.OutOfHoursMessage != "" {
				outOfHoursMessage = settings.OutOfHoursMessage
			}

			behaviour := database.OutOfHoursBehaviourBlockCreation
			if exists {
				behaviour = settings.OutOfHoursBehaviour
			}

			switch behaviour {
			case database.OutOfHoursBehaviourAllowWithWarning:
				// Allow ticket creation but pass warning through
				if outOfHoursMessage == "" {
					outOfHoursMessage = ctx.GetMessage(i18n.MessageOutsideSupportHours)
				}
				return true, &outOfHoursMessage, nil
			default: // block_creation
				if outOfHoursMessage != "" {
					ctx.ReplyRaw(customisation.Red, ctx.GetMessage(i18n.Error), outOfHoursMessage)
				} else {
					ctx.Reply(customisation.Red, i18n.Error, i18n.MessageOutsideSupportHours)
				}
				return false, nil, nil
			}
		}
	}

	// Check blacklist
	blacklisted, err := ctx.IsBlacklisted(ctx)
	if err != nil {
		return false, nil, err
	}

	if blacklisted {
		var message i18n.MessageId

		if ctx.GuildId() == 0 || blacklist.IsUserBlacklisted(ctx.UserId()) {
			message = i18n.MessageUserBlacklisted
		} else {
			message = i18n.MessageBlacklisted
		}

		ctx.Reply(customisation.Red, i18n.TitleBlacklisted, message)
		return false, nil, nil
	}

	// Check access control
	member, err := ctx.Member()
	if err != nil {
		return false, nil, err
	}

	matchedRole, action, err := dbclient.Client.PanelAccessControlRules.GetFirstMatched(
		ctx,
		panel.PanelId,
		append(member.Roles, ctx.GuildId()),
	)

	if err != nil {
		return false, nil, err
	}

	if action == database.AccessControlActionDeny {
		if err := sendAccessControlDeniedMessage(ctx, ctx, panel.PanelId, matchedRole); err != nil {
			return false, nil, err
		}
		return false, nil, nil
	} else if action != database.AccessControlActionAllow {
		return false, nil, fmt.Errorf("invalid access control action %s", action)
	}

	return true, nil, nil
}

func sendAccessControlDeniedMessage(ctx context.Context, cmd registry.InteractionContext, panelId int, matchedRole uint64) error {
	rules, err := dbclient.Client.PanelAccessControlRules.GetAll(ctx, panelId)
	if err != nil {
		return err
	}

	allowedRoleIds := make([]uint64, 0, len(rules))
	for _, rule := range rules {
		if rule.Action == database.AccessControlActionAllow {
			allowedRoleIds = append(allowedRoleIds, rule.RoleId)
		}
	}

	if len(allowedRoleIds) == 0 {
		cmd.Reply(customisation.Red, i18n.MessageNoPermission, i18n.MessageOpenAclNoAllowRules)
		return nil
	}

	if matchedRole == cmd.GuildId() {
		mentions := make([]string, 0, len(allowedRoleIds))
		for _, roleId := range allowedRoleIds {
			mentions = append(mentions, fmt.Sprintf("<@&%d>", roleId))
		}

		if len(allowedRoleIds) == 1 {
			cmd.Reply(customisation.Red, i18n.MessageNoPermission, i18n.MessageOpenAclNotAllowListedSingle, strings.Join(mentions, ", "))
		} else {
			cmd.Reply(customisation.Red, i18n.MessageNoPermission, i18n.MessageOpenAclNotAllowListedMultiple, strings.Join(mentions, ", "))
		}
	} else {
		cmd.Reply(customisation.Red, i18n.MessageNoPermission, i18n.MessageOpenAclDenyListed, matchedRole)
	}

	return nil
}
