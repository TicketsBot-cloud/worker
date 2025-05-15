package settings

import (
	"fmt"
	"time"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/common/premium"
	"github.com/TicketsBot-cloud/gdl/objects/channel/embed"
	"github.com/TicketsBot-cloud/gdl/objects/guild/emoji"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/config"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type PremiumCommand struct {
}

func (PremiumCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:             "premium",
		Description:      i18n.HelpPremium,
		Type:             interaction.ApplicationCommandTypeChatInput,
		PermissionLevel:  permission.Admin,
		Category:         command.Settings,
		DefaultEphemeral: true,
		Timeout:          time.Second * 5,
	}
}

func (c PremiumCommand) GetExecutor() interface{} {
	return c.Execute
}

func (PremiumCommand) Execute(ctx registry.CommandContext) {
	premiumTier := ctx.PremiumTier()

	// Tell user if premium is already active
	if premiumTier > premium.None {
		// Re-enable panels
		if err := dbclient.Client.Panel.EnableAll(ctx, ctx.GuildId()); err != nil {
			ctx.HandleError(err)
			return
		}

		var content i18n.MessageId
		if premiumTier == premium.Whitelabel {
			content = i18n.MessagePremiumLinkAlreadyActivatedWhitelabel
		} else {
			content = i18n.MessagePremiumLinkAlreadyActivated
		}

		buttons := []component.Component{
			component.BuildButton(component.Button{
				Label:    ctx.GetMessage(i18n.MessagePremiumUseKeyAnyway),
				CustomId: "open_premium_key_modal",
				Style:    component.ButtonStyleSecondary,
				Emoji:    utils.BuildEmoji("🔑"),
			}),
		}

		// Check for patreon, and show server selector button if necessary
		legacyEntitlement, err := dbclient.Client.LegacyPremiumEntitlements.GetUserTier(ctx, ctx.UserId(), premium.PatreonGracePeriod)
		if err != nil {
			ctx.HandleError(err)
			return
		}

		if legacyEntitlement != nil && !legacyEntitlement.IsLegacy {
			// make it first button
			buttons = append([]component.Component{
				component.BuildButton(component.Button{
					Label: ctx.GetMessage(i18n.MessagePremiumOpenServerSelector),
					Style: component.ButtonStyleLink,
					Emoji: utils.BuildEmoji("🔗"),
					Url:   utils.Ptr(fmt.Sprintf("%s/premium/select-servers", config.Conf.Bot.DashboardUrl)),
				}),
			}, buttons...)
		}

		ctx.ReplyWith(command.NewEphemeralEmbedMessageResponseWithComponents(
			utils.BuildEmbed(ctx, customisation.Green, i18n.TitlePremium, content, nil),
			utils.Slice(component.BuildActionRow(buttons...)),
		))

	} else {
		var patreonEmoji, discordEmoji, keyEmoji *emoji.Emoji
		if !ctx.Worker().IsWhitelabel {
			patreonEmoji = customisation.EmojiPatreon.BuildEmoji()
			discordEmoji = customisation.EmojiDiscord.BuildEmoji()
			keyEmoji = utils.BuildEmoji("🔑")
		}

		fields := utils.Slice(embed.EmbedField{
			Name:   ctx.GetMessage(i18n.MessagePremiumAlreadyPurchasedTitle),
			Value:  ctx.GetMessage(i18n.MessagePremiumAlreadyPurchasedDescription),
			Inline: false,
		})

		ctx.ReplyWith(command.NewEphemeralEmbedMessageResponseWithComponents(
			utils.BuildEmbed(ctx, customisation.Green, i18n.TitlePremium, i18n.MessagePremiumAbout, fields),
			utils.Slice(
				component.BuildActionRow(
					component.BuildSelectMenu(component.SelectMenu{
						CustomId: "premium_purchase_method",
						Options: utils.Slice(
							component.SelectOption{
								Label:       "Patreon", // Don't translate
								Value:       "patreon",
								Description: ctx.GetMessage(i18n.MessagePremiumMethodSelectorPatreon),
								Emoji:       patreonEmoji,
							},
							component.SelectOption{
								Label:       "Discord",
								Value:       "discord",
								Description: ctx.GetMessage(i18n.MessagePremiumMethodSelectorDiscord),
								Emoji:       discordEmoji,
							},
							component.SelectOption{
								Label:       ctx.GetMessage(i18n.MessagePremiumGiveawayKey),
								Value:       "key",
								Description: ctx.GetMessage(i18n.MessagePremiumMethodSelectorKey),
								Emoji:       keyEmoji,
							},
						),
						Placeholder: ctx.GetMessage(i18n.MessagePremiumMethodSelector),
						Disabled:    false,
					}),
				),
				component.BuildActionRow(
					component.BuildButton(component.Button{
						Label: ctx.GetMessage(i18n.Website),
						Style: component.ButtonStyleLink,
						Emoji: utils.BuildEmoji("🔗"),
						Url:   utils.Ptr(fmt.Sprintf("%s/premium", config.Conf.Bot.FrontpageUrl)),
					}),
				),
			),
		))
	}
}
