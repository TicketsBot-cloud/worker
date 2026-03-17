package admin

import (
	"fmt"
	"strconv"
	"time"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/redis"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type AdminRecacheCommand struct {
}

func (AdminRecacheCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:            "recache",
		Description:     i18n.HelpAdmin,
		Type:            interaction.ApplicationCommandTypeChatInput,
		PermissionLevel: permission.Everyone,
		Category:        command.Settings,
		HelperOnly:      true,
		Arguments: command.Arguments(
			command.NewOptionalArgument("guildid", "ID of the guild to recache", interaction.OptionTypeString, i18n.MessageInvalidArgument),
		),
		Timeout: time.Second * 10,
	}
}

func (c AdminRecacheCommand) GetExecutor() interface{} {
	return c.Execute
}

func (AdminRecacheCommand) Execute(ctx registry.CommandContext, providedGuildId *string) {
	var guildId uint64
	if providedGuildId != nil {
		var err error
		guildId, err = strconv.ParseUint(*providedGuildId, 10, 64)
		if err != nil {
			ctx.HandleError(err)
			return
		}
	} else {
		guildId = ctx.GuildId()
	}

	if onCooldown, cooldownTime := redis.GetRecacheCooldown(guildId); onCooldown {
		ctx.ReplyWith(command.NewMessageResponseWithComponents([]component.Component{
			utils.BuildContainerWithComponents(
				ctx,
				customisation.Red,
				"Admin - Recache",
				[]component.Component{
					component.BuildTextDisplay(component.TextDisplay{
						Content: fmt.Sprintf("Recache for this guild is on cooldown. Please wait until it is available again.\n\n**Cooldown ends** <t:%d:R>", cooldownTime.Unix()),
					}),
				},
			)}))
		return
	}

	currentTime := time.Now()

	// purge cache
	ctx.Worker().Cache.DeleteGuild(ctx, guildId)
	ctx.Worker().Cache.DeleteGuildChannels(ctx, guildId)
	ctx.Worker().Cache.DeleteGuildRoles(ctx, guildId)

	// re-cache
	worker, err := utils.WorkerForGuild(ctx, ctx.Worker(), guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	guild, err := worker.GetGuild(guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	guildChannels, err := worker.GetGuildChannels(guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if err := redis.SetRecacheCooldown(guildId, time.Second*30); err != nil {
		ctx.HandleError(err)
		return
	}

	ctx.ReplyWith(command.NewMessageResponseWithComponents([]component.Component{
		utils.BuildContainerWithComponents(
			ctx,
			customisation.Orange,
			"Admin - Recache",
			[]component.Component{
				component.BuildTextDisplay(component.TextDisplay{
					Content: fmt.Sprintf("**%s** has been recached successfully.\n\n**Guild ID:** %d\n**Time Taken:** %s", guild.Name, guildId, time.Since(currentTime).Round(time.Millisecond)),
				}),
				component.BuildSeparator(component.Separator{}),
				component.BuildTextDisplay(component.TextDisplay{
					Content: fmt.Sprintf("### Cache Stats\n**Channels:** `%d`\n**Roles:** `%d`", len(guildChannels), len(guild.Roles)),
				}),
			},
		),
	}))
}
