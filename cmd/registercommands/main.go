package main

import (
	"context"
	"encoding/json"
	"net/http"
	"flag"
	"fmt"
	"io"

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
)

func main() {
	flag.Parse()
	if *Token == "" {
		panic("no token")
	}

	applicationId := must(getApplicationId(*Token))

	i18n.Init()

	commandManager := new(manager.CommandManager)
	commandManager.RegisterCommands()

	data, adminCommands := commandManager.BuildCreatePayload(false, AdminCommandGuildId)

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
	req, err := http.NewRequest("GET", "https://discord.com/api/v10/oauth2/applications/@me", nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bot "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("failed to get application id: %s", string(body))
	}

	var data struct {
		Id string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, err
	}

	var id uint64
	_, err = fmt.Sscanf(data.Id, "%d", &id)
	if err != nil {
		return 0, err
	}

	return id, nil
}

func must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}

	return t
}
