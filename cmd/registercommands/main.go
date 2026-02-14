package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"strings"

	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/rest"
	"github.com/TicketsBot-cloud/worker/bot/command/manager"
	"github.com/TicketsBot-cloud/worker/i18n"
)

var (
	Token               = flag.String("token", "", "Bot token to create commands for")
	GuildId             = flag.Uint64("guild", 0, "Guild to create the commands for")
	AdminCommandGuildId = flag.Uint64("admin-guild", 0, "Guild to create the admin commands in")
	MergeGuildCommands  = flag.Bool("merge", true, "Merge new commands with existing ones instead of overwriting")
	DevMode             = flag.Bool("dev", false, "Enable dev mode")
)

func main() {
	devMode := false
	flag.Parse()
	if *Token == "" {
		panic("no token")
	}

	if DevMode != nil {
		devMode = *DevMode
	}

	applicationId := must(getApplicationId(*Token))

	i18n.Init()

	commandManager := new(manager.CommandManager)
	commandManager.RegisterCommands()

	data, adminCommands := commandManager.BuildCreatePayload(devMode, false, AdminCommandGuildId)

	// Register commands globally or for a specific guild
	if *GuildId == 0 {
		must(rest.ModifyGlobalCommands(context.Background(), *Token, nil, applicationId, data))
	} else {
		must(rest.ModifyGuildCommands(context.Background(), *Token, nil, applicationId, *GuildId, data))
	}

	// Handle admin commands for a specific guild, merging if requested
	if *AdminCommandGuildId != 0 {
		if *MergeGuildCommands {
			existingCmds := must(rest.GetGuildCommands(context.Background(), *Token, nil, applicationId, *AdminCommandGuildId))
			for _, cmd := range existingCmds {
				var found bool
				for _, newCmd := range adminCommands {
					if cmd.Name == newCmd.Name {
						found = true
						break
					}
				}
				if !found {
					adminCommands = append(adminCommands, rest.CreateCommandData{
						Id:          cmd.Id,
						Name:        cmd.Name,
						Description: cmd.Description,
						Options:     cmd.Options,
						Type:        interaction.ApplicationCommandTypeChatInput,
						Contexts:    cmd.Contexts,
					})
				}
			}
		}
		must(rest.ModifyGuildCommands(context.Background(), *Token, nil, applicationId, *AdminCommandGuildId, adminCommands))
	}

	// Output all global commands as JSON
	cmds := must(rest.GetGlobalCommands(context.Background(), *Token, nil, applicationId))
	marshalled := must(json.MarshalIndent(cmds, "", "    "))
	fmt.Println(string(marshalled))
}

// getApplicationId fetches the application ID using the bot token
func getApplicationId(token string) (uint64, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) < 1 {
		return 0, fmt.Errorf("invalid token format")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return 0, fmt.Errorf("failed to base64 decode token: %w", err)
	}
	var id uint64
	_, err = fmt.Sscanf(string(decoded), "%d", &id)
	if err != nil {
		return 0, fmt.Errorf("failed to parse application id: %w", err)
	}
	return id, nil
}

func must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}

	return t
}
