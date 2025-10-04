package general

import (
	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

const (
	gdprIntroText = "Select the type of GDPR request you would like to make:"
	
	transcriptSectionTitle = "### **Transcript Deletion Options**\n-# _Delete transcripts from servers you own_"
	messageSectionTitle = "### **Message Deletion Options**\n-# _Remove your messages from ticket transcripts_"
	
	warningText = "### **Important Information**\n" +
		"* Deletion is **permanent** and cannot be undone\n" +
		"* You can only delete transcripts from servers **you** own\n" +
		"* Only **your** messages can be deleted from transcripts\n" +
		"* The request may take up to **30 days** to complete\n" +
		"* Your requst will be processed in a **queue**"
	
	resourcesText = "### **Resources**\n" +
		"[What is GDPR?](https://gdpr.eu/what-is-gdpr/)\n" +
		"[Right to Erasure](https://gdpr-info.eu/art-17-gdpr/)\n" +
		"[Right of Access](https://gdpr-info.eu/art-15-gdpr/)"
)

type gdprButton struct {
	Label    string
	CustomID string
}

var (
	transcriptButtons = []gdprButton{
		{"All transcripts from a server", "gdpr_all_transcripts"},
		{"Specific transcripts", "gdpr_specific_transcripts"},
	}
	
	messageButtons = []gdprButton{
		{"All messages from account", "gdpr_all_messages"},
		{"Messages in specific tickets", "gdpr_specific_messages"},
	}
)

type GDPRCommand struct{}

func (GDPRCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:            "gdpr",
		Description:     i18n.HelpGdpr,
		Type:            interaction.ApplicationCommandTypeChatInput,
		PermissionLevel: permission.Everyone,
		Category:        command.General,
		Contexts:        []interaction.InteractionContextType{interaction.InteractionContextBotDM},
	}
}

func (c GDPRCommand) GetExecutor() interface{} {
	return c.Execute
}

func (GDPRCommand) Execute(ctx registry.CommandContext) {
	components := buildGDPRComponents(ctx)
	if _, err := ctx.ReplyWith(command.NewMessageResponseWithComponents(components)); err != nil {
		ctx.HandleError(err)
	}
}

func buildGDPRComponents(ctx registry.CommandContext) []component.Component {
	innerComponents := []component.Component{
		buildTextSection(gdprIntroText),
		component.BuildSeparator(component.Separator{}),
		buildTextSection(transcriptSectionTitle),
		buildButtonRow(transcriptButtons),
		component.BuildSeparator(component.Separator{}),
		buildTextSection(messageSectionTitle),
		buildButtonRow(messageButtons),
		component.BuildSeparator(component.Separator{}),
		buildTextSection(warningText),
		component.BuildSeparator(component.Separator{}),
		buildTextSection(resourcesText),
	}

	container := utils.BuildContainerWithComponents(ctx, customisation.Green, "GDPR Data Request", innerComponents)
	return []component.Component{container}
}

func buildTextSection(content string) component.Component {
	return component.BuildTextDisplay(component.TextDisplay{Content: content})
}

func buildButtonRow(buttons []gdprButton) component.Component {
	buttonComponents := make([]component.Component, len(buttons))
	for i, btn := range buttons {
		buttonComponents[i] = component.BuildButton(component.Button{
			Label:    btn.Label,
			CustomId: btn.CustomID,
			Style:    component.ButtonStylePrimary,
		})
	}
	return component.BuildActionRow(buttonComponents...)
}
