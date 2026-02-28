package context

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	permcache "github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/common/premium"
	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/objects/channel/embed"
	"github.com/TicketsBot-cloud/gdl/objects/guild/emoji"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/gdl/permission"
	"github.com/TicketsBot-cloud/gdl/rest/request"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/logic"
	"github.com/TicketsBot-cloud/worker/bot/permissionwrapper"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/config"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type Replyable struct {
	ctx         registry.CommandContext
	colourCodes map[customisation.Colour]int
}

type rateLimitResponse struct {
	Message    string  `json:"message"`
	RetryAfter float64 `json:"retry_after"`
	Global     bool    `json:"global"`
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

func (r *Replyable) buildEmbed(colour customisation.Colour, title, content i18n.MessageId, fields []embed.EmbedField, format ...interface{}) *embed.Embed {
	return utils.BuildEmbed(r.ctx, colour, title, content, fields, format...)
}

func (r *Replyable) buildEmbedRaw(colour customisation.Colour, title, content string, fields ...embed.EmbedField) *embed.Embed {
	return utils.BuildEmbedRaw(r.GetColour(colour), title, content, fields, r.ctx.PremiumTier())
}

func (r *Replyable) Reply(colour customisation.Colour, title, content i18n.MessageId, format ...interface{}) {
	embed := r.buildEmbed(colour, title, content, nil, format...)
	_, _ = r.ctx.ReplyWith(command.NewEphemeralEmbedMessageResponse(embed))
}

func (r *Replyable) ReplyPermanent(colour customisation.Colour, title, content i18n.MessageId, format ...interface{}) {
	embed := r.buildEmbed(colour, title, content, nil, format...)
	_, _ = r.ctx.ReplyWith(command.NewEmbedMessageResponse(embed))
}

func (r *Replyable) ReplyWithEmbed(embed *embed.Embed) {
	_, _ = r.ctx.ReplyWith(command.NewEphemeralEmbedMessageResponse(embed))
}

func (r *Replyable) ReplyWithEmbedAndComponents(embed *embed.Embed, components []component.Component) {
	_, _ = r.ctx.ReplyWith(command.NewEphemeralEmbedMessageResponseWithComponents(embed, components))
}

func (r *Replyable) ReplyWithEmbedPermanent(embed *embed.Embed) {
	_, _ = r.ctx.ReplyWith(command.NewEmbedMessageResponse(embed))
}

func (r *Replyable) ReplyWithFields(colour customisation.Colour, title, content i18n.MessageId, fields []embed.EmbedField, format ...interface{}) {
	embed := r.buildEmbed(colour, title, content, fields, format...)
	_, _ = r.ctx.ReplyWith(command.NewEphemeralEmbedMessageResponse(embed))
}

func (r *Replyable) ReplyWithFieldsPermanent(colour customisation.Colour, title, content i18n.MessageId, fields []embed.EmbedField, format ...interface{}) {
	embed := r.buildEmbed(colour, title, content, fields, format...)
	_, _ = r.ctx.ReplyWith(command.NewEmbedMessageResponse(embed))
}

func (r *Replyable) ReplyRaw(colour customisation.Colour, title, content string) {
	embed := r.buildEmbedRaw(colour, title, content)
	_, _ = r.ctx.ReplyWith(command.NewEphemeralEmbedMessageResponse(embed))
}

func (r *Replyable) ReplyRawWithComponents(colour customisation.Colour, title, content string, components ...component.Component) {
	embed := r.buildEmbedRaw(colour, title, content)
	_, _ = r.ctx.ReplyWith(command.NewEphemeralEmbedMessageResponseWithComponents(embed, components))
}

func (r *Replyable) ReplyRawPermanent(colour customisation.Colour, title, content string) {
	embed := r.buildEmbedRaw(colour, title, content)
	_, _ = r.ctx.ReplyWith(command.NewEmbedMessageResponse(embed))
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
	var imageUrl *string

	var restError request.RestError
	if errors.As(err, &restError) {
		if restError.ApiError.Code == 10003 { // Unknown channel
			message = r.GetMessage(i18n.MessageErrorUnknownChannel, config.Conf.Bot.DashboardUrl)
		} else if restError.ApiError.Code == 10004 { // Unknown guild
			message = r.GetMessage(i18n.MessageErrorUnknownGuild)
		} else if restError.ApiError.Code == 10007 { // Unknown member
			message = r.GetMessage(i18n.MessageErrorUnknownMember)
		} else if restError.ApiError.Code == 10008 { // Unknown message
			message = r.GetMessage(i18n.MessageErrorUnknownMessage)
		} else if restError.ApiError.Code == 10011 { // Unknown role
			message = r.GetMessage(i18n.MessageErrorUnknownRole, config.Conf.Bot.DashboardUrl)
		} else if restError.ApiError.Code == 10013 { // Unknown user
			message = r.GetMessage(i18n.MessageErrorUnknownUser)
		} else if restError.ApiError.Code == 10059 { // Unknown category
			message = r.GetMessage(i18n.MessageErrorUnknownCategory, config.Conf.Bot.DashboardUrl)
		} else if restError.ApiError.Code == 10062 { // Unknown interaction
			message = r.GetMessage(i18n.MessageErrorUnknownInteraction)
		} else if restError.ApiError.Code == 30007 { // Maximum number of webhooks reached
			message = r.GetMessage(i18n.MessageErrorMaxWebhooks)
		} else if restError.ApiError.Code == 30013 { // Maximum number of guild channels reached
			message = r.GetMessage(i18n.MessageErrorMaxChannels)
		} else if restError.ApiError.Code == 40060 { // Interaction has already been acknowledged
			message = r.GetMessage(i18n.MessageErrorInteractionAcknowledged)
		} else if restError.ApiError.Code == 50001 || restError.ApiError.Code == 50013 { // Missing permissions / Missing access
			interactionCtx, ok := r.ctx.(registry.InteractionContext)
			docsUrl := fmt.Sprintf("%s/miscellaneous/permissions-explained", config.Conf.Bot.DocsUrl)
			if ok {
				missingPermissions, err := findMissingPermissions(interactionCtx)
				if err == nil {
					if len(missingPermissions) > 0 {
						message = r.GetMessage(i18n.MessageErrorMissingPermissionsTitle) + ":\n"
						for _, perm := range missingPermissions {
							message += fmt.Sprintf("* `%s`\n", perm.String())
						}

						message += "\n" + r.GetMessage(i18n.MessageErrorMissingPermissionsBody, docsUrl)
					} else {
						message = r.GetMessage(i18n.MessageErrorMissingAccess, docsUrl)
					}
				} else {
					sentry.ErrorWithContext(err, r.ctx.ToErrorContext())
					message = r.GetMessage(i18n.MessageErrorMissingAccess, docsUrl)
				}
			} else {
				message = r.GetMessage(i18n.MessageErrorMissingAccess, docsUrl)
			}
		} else if restError.ApiError.Code == 50035 { // Invalid Form Body
			// Check for specific form validation errors
			if restError.ApiError.FirstErrorCode() == "BASE_TYPE_BAD_LENGTH" {
				message = r.GetMessage(i18n.MessageErrorInvalidLength, config.Conf.Bot.DashboardUrl)
			} else if restError.ApiError.FirstErrorCode() == "BASE_TYPE_REQUIRED" {
				message = r.GetMessage(i18n.MessageErrorRequiredField, config.Conf.Bot.DashboardUrl)
			} else if restError.ApiError.FirstErrorCode() == "CHANNEL_INVALID_TYPE" {
				message = r.GetMessage(i18n.MessageErrorInvalidChannelType, config.Conf.Bot.DashboardUrl)
			} else if restError.ApiError.FirstErrorCode() == "CHANNEL_PARENT_INVALID" {
				message = r.GetMessage(i18n.MessageErrorInvalidCategory, config.Conf.Bot.DashboardUrl)
			} else if restError.ApiError.FirstErrorCode() == "NUMBER_TYPE_COERCE" {
				message = r.GetMessage(i18n.MessageErrorInvalidId, config.Conf.Bot.DashboardUrl)
			} else if restError.ApiError.FirstErrorCode() == "STRING_TYPE_REGEX" {
				message = r.GetMessage(i18n.MessageErrorInvalidCharacters)
			} else if restError.ApiError.FirstErrorCode() == "UNION_TYPE_CHOICES" {
				message = r.GetMessage(i18n.MessageErrorInvalidChoice, config.Conf.Bot.DashboardUrl)
			} else {
				message = r.GetMessage(i18n.MessageErrorInvalidForm) + "\n\n" +
					r.formatDiscordError(restError, eventId)
			}
		} else if restError.ApiError.Code == 160005 { // Thread is locked
			message = r.GetMessage(i18n.MessageErrorThreadLocked)
		} else if restError.ApiError.Code == 160006 { // Maximum number of active threads reached
			message = r.GetMessage(i18n.MessageErrorMaxActiveThreads)
		} else if restError.ApiError.Code == 160007 { // Maximum number of active announcement threads reached
			message = r.GetMessage(i18n.MessageErrorMaxActiveAnnouncementThreads)
		} else if restError.StatusCode == http.StatusTooManyRequests {
			// Rate limit error - parse raw response to extract retry_after and global flag
			var rateLimit rateLimitResponse
			if err := json.Unmarshal(restError.Raw, &rateLimit); err == nil && rateLimit.RetryAfter > 0 {
				timestamp := formatTimestamp(rateLimit.RetryAfter)

				if rateLimit.Global {
					message = r.GetMessage(i18n.MessageErrorRateLimitedGlobal, timestamp)
				} else {
					message = r.GetMessage(i18n.MessageErrorRateLimited, timestamp)
				}
			} else {
				// Fallback if parsing fails or retry_after is not available
				message = r.GetMessage(i18n.MessageErrorRateLimited, "soon")
			}
		} else {
			message = r.formatDiscordError(restError, eventId)
		}
	} else if errors.Is(err, context.DeadlineExceeded) {
		message = r.GetMessage(i18n.MessageErrorTimeout)
	} else {
		message = r.GetMessage(i18n.MessageErrorGeneral) + "\n" + r.GetMessage(i18n.MessageErrorId) + ": `" + eventId + "`"
	}

	embed := r.buildEmbedRaw(customisation.Red, r.GetMessage(i18n.Error), message)
	if imageUrl != nil {
		embed.SetImage(*imageUrl)
	}

	res := command.NewEphemeralEmbedMessageResponse(embed)

	if includeInviteLink {
		res.Components = []component.Component{
			component.BuildActionRow(
				component.BuildButton(component.Button{
					Label: r.GetMessage(i18n.MessageJoinSupportServer),
					Style: component.ButtonStyleLink,
					Emoji: utils.BuildEmoji("❓"),
					Url:   utils.Ptr(strings.ReplaceAll(config.Conf.Bot.SupportServerInvite, "\n", "")),
				}),
			),
		}
	}

	return res
}

func (r *Replyable) formatDiscordError(restError request.RestError, eventId string) string {
	return r.GetMessage(i18n.MessageErrorGeneral) + ":\n```\n" +
		restError.Error() + "\n```\n" +
		r.GetMessage(i18n.MessageErrorId) + ": `" + eventId + "`"
}

func findMissingPermissions(ctx registry.InteractionContext) ([]permission.Permission, error) {
	if permission.HasPermissionRaw(ctx.InteractionMetadata().AppPermissions, permission.Administrator) {
		return nil, nil
	}

	var useThreads bool
	var targetChannelId uint64

	settings, err := ctx.Settings()
	if err == nil {
		useThreads = settings.UseThreads
	}

	var panel *database.Panel
	if btnCtx, ok := ctx.(*ButtonContext); ok {
		p, panelExists, err := dbclient.Client.Panel.GetByCustomId(context.Background(), ctx.GuildId(), btnCtx.InteractionData.CustomId)
		if err == nil && panelExists {
			panel = &p
			// Panel can enable threads if global setting is disabled
			if !useThreads {
				useThreads = panel.UseThreads
			}
		}
	}

	if useThreads {
		// Thread mode - check permissions in the current channel
		targetChannelId = ctx.ChannelId()
	} else {
		// Channel mode - check permissions in the ticket category
		if panel != nil && panel.TargetCategory != 0 {
			// Use panel's target category
			targetChannelId = panel.TargetCategory
		} else {
			// Fall back to guild default category
			targetChannelId, _ = dbclient.Client.ChannelCategory.Get(context.Background(), ctx.GuildId())
		}
	}

	// Build required permissions based on mode
	var requiredPermissions []permission.Permission
	if useThreads {
		// Thread mode permissions
		requiredPermissions = append(
			[]permission.Permission{
				permission.CreatePrivateThreads,
				permission.SendMessagesInThreads,
				permission.ManageThreads,
			},
			logic.StandardPermissions[:]...,
		)
	} else {
		// Channel mode permissions
		requiredPermissions = append(
			[]permission.Permission{
				permission.ManageChannels,
			},
			logic.StandardPermissions[:]...,
		)
	}

	if targetChannelId != 0 {
		return permissionwrapper.GetMissingPermissionsChannel(ctx.Worker(), ctx.GuildId(), ctx.Worker().BotId, targetChannelId, requiredPermissions...), nil
	}

	// If no target channel, just return guild-level missing permissions
	return permissionwrapper.GetMissingPermissions(ctx.Worker(), ctx.GuildId(), ctx.Worker().BotId, requiredPermissions...), nil
}

func formatTimestamp(seconds float64) string {
	futureTime := time.Now().Add(time.Duration(seconds) * time.Second)
	return fmt.Sprintf("<t:%d:R>", futureTime.Unix())
}
