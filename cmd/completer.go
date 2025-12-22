package cmd

import (
	"github.com/elk-language/go-prompt"
	istrings "github.com/elk-language/go-prompt/strings"

	"github.com/quocvuong92/perplexity-cli/internal/config"
)

// completer provides auto-completion suggestions for slash commands.
// It provides context-aware suggestions based on what the user is typing.
func (s *InteractiveSession) completer(d prompt.Document) ([]prompt.Suggest, istrings.RuneNumber, istrings.RuneNumber) {
	text := d.TextBeforeCursor()
	endIndex := d.CurrentRuneIndex()
	w := d.GetWordBeforeCursor()
	startIndex := endIndex - istrings.RuneCountInString(w)

	// Only show suggestions when input starts with "/"
	if len(text) == 0 || text[0] != '/' {
		return []prompt.Suggest{}, startIndex, endIndex
	}

	textLower := toLower(text)

	// /model <name> - suggest available models
	if hasPrefix(textLower, "/model ") || hasPrefix(textLower, "/m ") {
		var suggestions []prompt.Suggest
		for _, model := range config.AvailableModels {
			desc := ""
			if model == s.app.cfg.Model {
				desc = "(current)"
			}
			suggestions = append(suggestions, prompt.Suggest{Text: model, Description: desc})
		}
		return prompt.FilterHasPrefix(suggestions, w, true), startIndex, endIndex
	}

	// /citations - suggest on/off options
	if hasPrefix(textLower, "/citations ") {
		suggestions := []prompt.Suggest{
			{Text: "on", Description: "Enable citations display"},
			{Text: "off", Description: "Disable citations display"},
		}
		return prompt.FilterHasPrefix(suggestions, w, true), startIndex, endIndex
	}

	// /system - suggest reset option
	if hasPrefix(textLower, "/system ") {
		suggestions := []prompt.Suggest{
			{Text: "reset", Description: "Reset to default system prompt"},
		}
		return prompt.FilterHasPrefix(suggestions, w, true), startIndex, endIndex
	}

	// Build citations status for description
	citationsStatus := "off"
	if s.app.cfg.Citations {
		citationsStatus = "on"
	}

	// Main command suggestions
	suggestions := []prompt.Suggest{
		// Most used commands first
		{Text: "/model", Description: "Show/switch model (current: " + s.app.cfg.Model + ")"},
		{Text: "/system", Description: "Show/set system prompt"},
		{Text: "/citations", Description: "Toggle citations display (current: " + citationsStatus + ")"},
		{Text: "/clear", Description: "Clear conversation history"},
		{Text: "/retry", Description: "Retry last message"},
		{Text: "/copy", Description: "Copy last response to clipboard"},
		{Text: "/export", Description: "Export conversation to markdown"},
		{Text: "/help", Description: "Show all available commands"},
		{Text: "/exit", Description: "Exit interactive mode"},

		// History commands
		{Text: "/history", Description: "Show recent conversations"},
		{Text: "/search", Description: "Search conversations by keyword"},
		{Text: "/resume", Description: "Resume conversation by index"},
		{Text: "/delete", Description: "Delete conversation by index"},

		// Aliases
		{Text: "/q", Description: "Exit (alias)"},
		{Text: "/c", Description: "Clear (alias)"},
		{Text: "/r", Description: "Retry (alias)"},
		{Text: "/h", Description: "Help (alias)"},
		{Text: "/m", Description: "Model (alias)"},
	}

	return prompt.FilterHasPrefix(suggestions, w, true), startIndex, endIndex
}

// Helper functions to avoid strings import in this file
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
