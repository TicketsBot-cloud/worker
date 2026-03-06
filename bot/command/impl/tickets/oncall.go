package tickets

import (
	"errors"
	"fmt"
	"time"

	permcache "github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/member"
	"github.com/TicketsBot-cloud/gdl/rest/request"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/logic"
	"github.com/TicketsBot-cloud/worker/config"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type OnCallCommand struct {
}

func (OnCallCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:             "on-call",
		Description:      i18n.HelpOnCall,
		Type:             interaction.ApplicationCommandTypeChatInput,
		PermissionLevel:  permcache.Support,
		Category:         command.Tickets,
		DefaultEphemeral: true,
		Timeout:          time.Second * 8,
	}
}

func (c OnCallCommand) GetExecutor() interface{} {
	return c.Execute
}

func (OnCallCommand) Execute(ctx registry.CommandContext) {
	settings, err := ctx.Settings()
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if !settings.UseThreads {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageOnCallChannelMode, "/on-call", fmt.Sprintf("%s/features/thread-mode", config.Conf.Bot.DocsUrl))
		return
	}

	member, err := ctx.Member()
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Reflects *new* state
	onCall, err := dbclient.Client.OnCall.Toggle(ctx, ctx.GuildId(), ctx.UserId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	defaultTeam, teamIds, err := logic.GetMemberTeamsWithMember(ctx, ctx.GuildId(), ctx.UserId(), member)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	teams, err := dbclient.Client.SupportTeam.GetMulti(ctx, ctx.GuildId(), teamIds)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	metadata, err := dbclient.Client.GuildMetadata.Get(ctx, ctx.GuildId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if onCall { // *new* value
		if defaultTeam {
			if err := assignOnCallRole(ctx, member, metadata.OnCallRole, nil, 0); err != nil {
				ctx.HandleError(err)
				return
			}
		}

		for i, teamId := range teamIds {
			if i >= 5 { // Don't get caught up adding roles forever
				break
			}

			team, ok := teams[teamId]
			if !ok {
				continue
			}

			if err := assignOnCallRole(ctx, member, team.OnCallRole, &team, 0); err != nil {
				ctx.HandleError(err)
				return
			}
		}

		// TODO: Add assigning roles progress message
		ctx.Reply(customisation.Green, i18n.Success, i18n.MessageOnCallSuccess)
	} else {
		if defaultTeam && metadata.OnCallRole != nil {
			auditReason := fmt.Sprintf("Removed on-call role from %s", member.User.Username)
			reasonCtx := request.WithAuditReason(ctx, auditReason)
			if err := ctx.Worker().RemoveGuildMemberRole(reasonCtx, ctx.GuildId(), ctx.UserId(), *metadata.OnCallRole); err != nil {
				// If role was deleted, clear it from database and continue
				if restErr, ok := err.(request.RestError); ok && restErr.ApiError.Code == 10011 {
					if err := dbclient.Client.GuildMetadata.SetOnCallRole(ctx, ctx.GuildId(), nil); err != nil {
						ctx.HandleError(err)
						return
					}
				} else {
					ctx.HandleError(err)
					return
				}
			}
		}

		for i, teamId := range teamIds {
			if i >= 5 { // Don't get caught up adding roles forever
				break
			}

			team, ok := teams[teamId]
			if !ok {
				continue
			}

			if team.OnCallRole == nil {
				continue
			}

			reasonCtx2 := request.WithAuditReason(ctx, fmt.Sprintf("Removed team on-call role from %s", member.User.Username))
			if err := ctx.Worker().RemoveGuildMemberRole(reasonCtx2, ctx.GuildId(), ctx.UserId(), *team.OnCallRole); err != nil {
				// If role was deleted, clear it from database and continue
				if restErr, ok := err.(request.RestError); ok && restErr.ApiError.Code == 10011 {
					if err := dbclient.Client.SupportTeam.SetOnCallRole(ctx, team.Id, nil); err != nil {
						ctx.HandleError(err)
						return
					}
				} else {
					ctx.HandleError(err)
					return
				}
			}
		}

		ctx.Reply(customisation.Green, i18n.Success, i18n.MessageOnCallRemoveSuccess)
	}
}

// Attempt counter to prevent infinite loop
func assignOnCallRole(ctx registry.CommandContext, member member.Member, roleId *uint64, team *database.SupportTeam, attempt int) error {
	if attempt >= 2 {
		return errors.New("reached retry limit")
	}

	// Create role if it does not exist  yet
	if roleId == nil {
		tmp, err := logic.CreateOnCallRole(ctx, ctx, team)
		if err != nil {
			return err
		}

		roleId = &tmp
	}

	reasonCtx3 := request.WithAuditReason(ctx, fmt.Sprintf("Added on-call role to %s", member.User.Username))
	if err := ctx.Worker().AddGuildMemberRole(reasonCtx3, ctx.GuildId(), ctx.UserId(), *roleId); err != nil {
		// If role was deleted, recreate it
		if err, ok := err.(request.RestError); ok && err.ApiError.Code == 10011 {
			if team == nil {
				if err := dbclient.Client.GuildMetadata.SetOnCallRole(ctx, ctx.GuildId(), nil); err != nil {
					return err
				}
			} else {
				if err := dbclient.Client.SupportTeam.SetOnCallRole(ctx, team.Id, nil); err != nil {
					return err
				}
			}

			return assignOnCallRole(ctx, member, nil, team, attempt+1)
		} else {
			return err
		}
	}

	return nil
}
