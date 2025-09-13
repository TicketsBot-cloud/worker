package utils

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNoEscape(t *testing.T) {
	input := "hello world"
	require.Equal(t, input, EscapeMarkdown(input))
}

func TestEscapeBold(t *testing.T) {
	input := "hello **world**"
	require.Equal(t, "hello \\*\\*world\\*\\*", EscapeMarkdown(input))
}

func TestEscapeMulti(t *testing.T) {
	input := "hello __**world**__"
	require.Equal(t, "hello \\_\\_\\*\\*world\\*\\*\\_\\_", EscapeMarkdown(input))
}

func TestEscapeLink(t *testing.T) {
	input := "hello https://google.com/some_path_here **hello world**"
	expected := "hello https://google.com/some_path_here \\*\\*hello world\\*\\*"
	require.Equal(t, expected, EscapeMarkdown(input))
}

func TestHttpsIncomplete(t *testing.T) {
	input := "hello https:/"
	require.Equal(t, input, EscapeMarkdown(input))
}

func TestHttpIncomplete(t *testing.T) {
	input := "hello http:/"
	require.Equal(t, input, EscapeMarkdown(input))
}

func TestChannelMention(t *testing.T) {
	input := "Check out <#123456789> for more info"
	expected := "Check out <#123456789> for more info"
	require.Equal(t, expected, EscapeMarkdown(input))
}

func TestUserMention(t *testing.T) {
	input := "Hello <@987654321>, welcome!"
	expected := "Hello <@987654321>, welcome!"
	require.Equal(t, expected, EscapeMarkdown(input))
}

func TestUserMentionWithNickname(t *testing.T) {
	input := "Hello <@!987654321>, welcome!"
	expected := "Hello <@!987654321>, welcome!"
	require.Equal(t, expected, EscapeMarkdown(input))
}

func TestRoleMention(t *testing.T) {
	input := "Calling all <@&456789123> members"
	expected := "Calling all <@&456789123> members"
	require.Equal(t, expected, EscapeMarkdown(input))
}

func TestMixedMentionsAndMarkdown(t *testing.T) {
	input := "**Important**: Check <#123456789> and ask <@987654321> about *details*"
	expected := "\\*\\*Important\\*\\*: Check <#123456789> and ask <@987654321> about \\*details\\*"
	require.Equal(t, expected, EscapeMarkdown(input))
}

func TestHashOutsideMention(t *testing.T) {
	input := "This is #not-a-mention but <#123456789> is"
	expected := "This is \\#not-a-mention but <#123456789> is"
	require.Equal(t, expected, EscapeMarkdown(input))
}

func TestMultipleMentions(t *testing.T) {
	input := "Join <#123456789> or <#987654321> and ping <@456789123>"
	expected := "Join <#123456789> or <#987654321> and ping <@456789123>"
	require.Equal(t, expected, EscapeMarkdown(input))
}
