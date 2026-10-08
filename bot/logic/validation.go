package logic

import (
	"fmt"

	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/worker/bot/blacklist"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

// replyIfPanelUnavailable reports whether the panel is switched off, replying with
// the reason when it is. OpenTicket repeats this check as a last line of defence;
// doing it here too means a form-backed panel is rejected on click, rather than
// after the user has filled the modal in.
func replyIfPanelUnavailable(cmd registry.InteractionContext, panel *database.Panel) (bool, error) {
	if panel == nil {
		return false, nil
	}

	if panel.ForceDisabled {
		commands, err := command.LoadCommandIds(cmd.Worker(), cmd.Worker().BotId)
		if err != nil {
			return true, err
		}

		premiumCommand := "`/premium`"
		if id, ok := commands["premium"]; ok {
			premiumCommand = fmt.Sprintf("</premium:%d>", id)
		}

		cmd.Reply(customisation.Red, i18n.Error, i18n.MessageOpenPanelForceDisabled, premiumCommand)
		return true, nil
	}

	if panel.Disabled {
		cmd.Reply(customisation.Red, i18n.Error, i18n.MessageOpenPanelDisabled)
		return true, nil
	}

	return false, nil
}

// ValidatePanelAccess checks if the user can access the given panel.
// Returns (canProceed, outOfHoursWarningTitle, outOfHoursWarning, outOfHoursColour, error).
// outOfHoursWarning is non-nil when the panel is outside support hours but the behaviour is allow_with_warning.
// outOfHoursColour is non-nil when a custom colour is configured for the out-of-hours embed.
func ValidatePanelAccess(ctx registry.InteractionContext, panel database.Panel) (bool, *string, *string, *int, error) {
	// Variables to hold out-of-hours warning info if behaviour is allow_with_warning
	var outOfHoursWarningTitle *string
	var outOfHoursWarningMessage *string
	var outOfHoursWarningColour *int

	// Check the panel is switched on before anything else
	unavailable, err := replyIfPanelUnavailable(ctx, &panel)
	if err != nil {
		return false, nil, nil, nil, err
	}

	if unavailable {
		return false, nil, nil, nil, nil
	}

	// Check support hours
	hasSupportHours, err := dbclient.Client.PanelSupportHours.HasSupportHours(ctx, panel.PanelId)
	if err != nil {
		return false, nil, nil, nil, err
	}

	if hasSupportHours {
		isActive, err := dbclient.Client.PanelSupportHours.IsActiveNow(ctx, panel.PanelId)
		if err != nil {
			return false, nil, nil, nil, err
		}

		if !isActive {
			// Fetch behaviour settings for this panel
			settings, exists, err := dbclient.Client.PanelSupportHoursSettings.Get(ctx, panel.PanelId)
			if err != nil {
				return false, nil, nil, nil, err
			}

			// Determine the warning/error title
			var outOfHoursTitle string
			if exists && settings.OutOfHoursTitle != "" {
				outOfHoursTitle = settings.OutOfHoursTitle
			}

			// Determine the warning/error message
			var outOfHoursMessage string
			if exists && settings.OutOfHoursMessage != "" {
				outOfHoursMessage = settings.OutOfHoursMessage
			}

			// Determine the custom colour (nil means use default)
			var outOfHoursColour *int
			if exists && settings.OutOfHoursColour != 0 {
				outOfHoursColour = &settings.OutOfHoursColour
			}

			behaviour := database.OutOfHoursBehaviourBlockCreation
			if exists {
				behaviour = settings.OutOfHoursBehaviour
			}

			// Allow ticket creation but pass warning through
			if outOfHoursMessage == "" {
				outOfHoursMessage = ctx.GetMessage(i18n.MessageOutsideSupportHours)
			}
			if outOfHoursTitle == "" {
				outOfHoursTitle = ctx.GetMessage(i18n.MessageOutsideSupportHoursTitle)
			}

			switch behaviour {
			case database.OutOfHoursBehaviourAllowWithWarning:
				outOfHoursWarningTitle = &outOfHoursTitle
				outOfHoursWarningMessage = &outOfHoursMessage
				outOfHoursWarningColour = outOfHoursColour
			default:
				if outOfHoursColour != nil {
					embed := utils.BuildEmbedRaw(*outOfHoursColour, outOfHoursTitle, outOfHoursMessage, nil, ctx.PremiumTier())
					ctx.ReplyWith(command.NewEphemeralEmbedMessageResponse(embed))
				} else {
					ctx.ReplyRaw(customisation.Red, outOfHoursTitle, outOfHoursMessage)
				}
				return false, nil, nil, nil, nil
			}
		}
	}

	// Check blacklist
	blacklisted, err := ctx.IsBlacklisted(ctx)
	if err != nil {
		return false, nil, nil, nil, err
	}

	if blacklisted {
		var message i18n.MessageId

		if ctx.GuildId() == 0 || blacklist.IsUserBlacklisted(ctx.UserId()) {
			message = i18n.MessageUserBlacklisted
		} else {
			message = i18n.MessageBlacklisted
		}

		ctx.Reply(customisation.Red, i18n.TitleBlacklisted, message)
		return false, nil, nil, nil, nil
	}

	// Check access control
	member, err := ctx.Member()
	if err != nil {
		return false, nil, nil, nil, err
	}

	rules, err := dbclient.Client.PanelAccessControlRules.GetAll(ctx, panel.PanelId)
	if err != nil {
		return false, nil, nil, nil, err
	}

	outcome, matched := EvaluateAccessControl(rules, member.Roles, ctx.GuildId())

	if matched != nil &&
		matched.Action != database.AccessControlActionAllow &&
		matched.Action != database.AccessControlActionDeny {
		// Log only: HandleWarning would send an error embed on top of the denial reply.
		sentry.LogWithContext(fmt.Errorf(
			"panel %d has an access control rule for role %d with invalid action %q",
			panel.PanelId, matched.RoleId, matched.Action,
		), ctx.ToErrorContext())
	}

	if !outcome.Allowed() {
		denial := buildAclDenial(rules, matched, ctx.GuildId())
		ctx.Reply(customisation.Red, i18n.MessageNoPermission, denial.Content, denial.Args...)
		return false, nil, nil, nil, nil
	}

	return true, outOfHoursWarningTitle, outOfHoursWarningMessage, outOfHoursWarningColour, nil
}
