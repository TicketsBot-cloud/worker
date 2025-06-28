package context

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	permcache "github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/common/premium"
	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/gdl/objects/guild/emoji"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/gdl/permission"
	"github.com/TicketsBot-cloud/gdl/rest/request"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/logic"
	"github.com/TicketsBot-cloud/worker/bot/model"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/config"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type Replyable struct {
	ctx         registry.CommandContext
	colourCodes map[customisation.Colour]int
}

func NewReplyable(ctx registry.CommandContext) *Replyable {
	var colourCodes map[customisation.Colour]int
	if ctx.PremiumTier() > premium.None {
		// TODO: Propagate context
		tmp, err := customisation.GetColours(context.Background(), ctx.GuildId())
		if err != nil {
			sentry.ErrorWithContext(err, ctx.ToErrorContext())
			colourCodes = customisation.DefaultColours
		} else {
			colourCodes = tmp
		}
	} else {
		colourCodes = customisation.DefaultColours
	}

	return &Replyable{
		ctx:         ctx,
		colourCodes: colourCodes,
	}
}

func (r *Replyable) GetColour(colour customisation.Colour) int {
	return r.colourCodes[colour]
}

func (r *Replyable) Reply(colour customisation.Colour, title, content i18n.MessageId, format ...interface{}) {
	container := utils.BuildContainerRaw(r.GetColour(colour), r.GetMessage(title), r.GetMessage(content, format...), r.ctx.PremiumTier())
	_, _ = r.ctx.ReplyWith(command.NewEphemeralMessageResponseWithComponents(utils.Slice(container)))
}

func (r *Replyable) ReplyPermanent(colour customisation.Colour, title, content i18n.MessageId, format ...interface{}) {
	container := utils.BuildContainerRaw(r.GetColour(colour), r.GetMessage(title), r.GetMessage(content, format...), r.ctx.PremiumTier())
	_, _ = r.ctx.ReplyWith(command.NewMessageResponseWithComponents(utils.Slice(container)))
}

func (r *Replyable) ReplyWithFields(colour customisation.Colour, title, content i18n.MessageId, fields []model.Field, format ...interface{}) {
	container := utils.BuildContainerWithFields(r.ctx, colour, title, content, fields, format...)
	_, _ = r.ctx.ReplyWith(command.NewEphemeralMessageResponseWithComponents(utils.Slice(container)))
}

func (r *Replyable) ReplyWithFieldsPermanent(colour customisation.Colour, title, content i18n.MessageId, fields []model.Field, format ...interface{}) {
	container := utils.BuildContainerWithFields(r.ctx, colour, title, content, fields, format...)
	_, _ = r.ctx.ReplyWith(command.NewMessageResponseWithComponents(utils.Slice(container)))
}

func (r *Replyable) ReplyRaw(colour customisation.Colour, title, content string) {
	container := utils.BuildContainerRaw(r.GetColour(colour), title, content, r.ctx.PremiumTier())
	_, _ = r.ctx.ReplyWith(command.NewEphemeralMessageResponseWithComponents(utils.Slice(container)))
}

func (r *Replyable) ReplyRawWithComponents(colour customisation.Colour, title, content string, components ...component.Component) {
	container := utils.BuildContainerRaw(r.GetColour(colour), title, content, r.ctx.PremiumTier())
	_, _ = r.ctx.ReplyWith(command.NewEphemeralMessageResponseWithComponents(append(utils.Slice(container), components...)))
}

func (r *Replyable) ReplyRawPermanent(colour customisation.Colour, title, content string) {
	container := utils.BuildContainerRaw(r.GetColour(colour), title, content, r.ctx.PremiumTier())
	_, _ = r.ctx.ReplyWith(command.NewMessageResponseWithComponents(utils.Slice(container)))
}

func (r *Replyable) ReplyPlain(content string) {
	_, _ = r.ctx.ReplyWith(command.NewEphemeralTextMessageResponse(content))
}

func (r *Replyable) ReplyPlainPermanent(content string) {
	_, _ = r.ctx.ReplyWith(command.NewTextMessageResponse(content))
}

func (r *Replyable) HandleError(err error) {
	if config.Conf.DebugMode != "" {
		fmt.Printf("ctx.HandleError: %s\n", err.Error())
	}

	eventId := sentry.ErrorWithContext(err, r.ctx.ToErrorContext())

	if errors.Is(err, ErrReplyLimitReached) {
		return
	}

	// We should show the invite link if the user is staff (or if we failed to resolve their permission level, show it)
	ctx, cancel := context.WithTimeout(r.ctx, time.Second*3)
	defer cancel()

	permLevel, resolveError := r.ctx.UserPermissionLevel(ctx)
	showInviteLink := !r.ctx.Worker().IsWhitelabel && (resolveError != nil || permLevel > permcache.Everyone)

	res := r.buildErrorResponse(err, eventId, showInviteLink)
	_, _ = r.ctx.ReplyWith(res)
}

func (r *Replyable) HandleWarning(err error) {
	eventId := sentry.LogWithContext(err, r.ctx.ToErrorContext())

	if errors.Is(err, ErrReplyLimitReached) {
		return
	}

	ctx, cancel := context.WithTimeout(r.ctx, time.Second*3)
	defer cancel()

	// We should show the invite link if the user is staff (or if we failed to resolve their permission level, show it)
	permLevel, resolveError := r.ctx.UserPermissionLevel(ctx)
	showInviteLink := !r.ctx.Worker().IsWhitelabel && (resolveError != nil || permLevel > permcache.Everyone)

	res := r.buildErrorResponse(err, eventId, showInviteLink)
	_, _ = r.ctx.ReplyWith(res)
}

func (r *Replyable) GetMessage(messageId i18n.MessageId, format ...interface{}) string {
	return i18n.GetMessageFromGuild(r.ctx.GuildId(), messageId, format...)
}

func (r *Replyable) SelectValidEmoji(customEmoji customisation.CustomEmoji, fallback string) *emoji.Emoji {
	if r.ctx.Worker().IsWhitelabel {
		return utils.BuildEmoji(fallback) // TODO: Check whitelabel_guilds table for emojis server
	} else {
		return customEmoji.BuildEmoji()
	}
}

func (r *Replyable) buildErrorResponse(err error, eventId string, includeInviteLink bool) command.MessageResponse {
	var message string
	// var imageUrl *string

	var restError request.RestError
	if errors.As(err, &restError) {
		if restError.ApiError.Message == "Missing Permissions" { // Override for missing permissions
			interactionCtx, ok := r.ctx.(registry.InteractionContext)
			if ok {
				missingPermissions, err := findMissingPermissions(interactionCtx)
				if err == nil {
					if len(missingPermissions) > 0 {
						message = "I am missing the following permissions required to perform this action:\n"
						for _, perm := range missingPermissions {
							message += fmt.Sprintf("• `%s`\n", perm.String())
						}

						message += "\nPlease assign these permissions to the bot, or alternatively, the `Administrator` permission and try again."
					} else {
						message = formatDiscordError(restError, eventId)
					}
				} else {
					sentry.ErrorWithContext(err, r.ctx.ToErrorContext())
					message = formatDiscordError(restError, eventId)
				}
			} else {
				message = formatDiscordError(restError, eventId)
			}
		} else if restError.ApiError.FirstErrorCode() == "CHANNEL_PARENT_INVALID" {
			message = "Could not find the ticket channel category: it must have been deleted. Ask a server " +
				"administrator to visit the dashboard and assign a valid channel category to this ticket panel."
		} else {
			message = formatDiscordError(restError, eventId)
		}
	} else if errors.Is(err, context.DeadlineExceeded) {
		message = "The operation timed out, please try again."
	} else {
		message = fmt.Sprintf("An error occurred while performing this action.\nError ID: `%s`", eventId)
	}

	container := utils.BuildContainerRaw(r.GetColour(customisation.Red), r.GetMessage(i18n.Error), message, r.ctx.PremiumTier())
	res := command.NewEphemeralMessageResponseWithComponents(utils.Slice(container))

	if includeInviteLink {
		res.Components = append(res.Components,
			component.BuildActionRow(
				component.BuildButton(component.Button{
					Label: r.GetMessage(i18n.MessageJoinSupportServer),
					Style: component.ButtonStyleLink,
					Emoji: utils.BuildEmoji("❓"),
					Url:   utils.Ptr(strings.ReplaceAll(config.Conf.Bot.SupportServerInvite, "\n", "")),
				}),
			),
		)
	}

	return res
}

func formatDiscordError(restError request.RestError, eventId string) string {
	return fmt.Sprintf("An error occurred while performing this action:\n"+
		"```\n"+
		"%s\n"+
		"```\n"+
		"Error ID: `%s`",
		restError.Error(), eventId)
}

func findMissingPermissions(ctx registry.InteractionContext) ([]permission.Permission, error) {
	if permission.HasPermissionRaw(ctx.InteractionMetadata().AppPermissions, permission.Administrator) {
		return nil, nil
	}

	requiredPermissions := append(
		[]permission.Permission{permission.ManageChannels, permission.ManageRoles},
		logic.StandardPermissions[:]...,
	)

	var missingPermissions []permission.Permission
	for _, requiredPermission := range requiredPermissions {
		if !permission.HasPermissionRaw(ctx.InteractionMetadata().AppPermissions, requiredPermission) {
			missingPermissions = append(missingPermissions, requiredPermission)
		}
	}

	return missingPermissions, nil
}
