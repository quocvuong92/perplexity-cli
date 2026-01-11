package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/quocvuong92/perplexity-cli/internal/api"
	"github.com/quocvuong92/perplexity-cli/internal/config"
	"github.com/quocvuong92/perplexity-cli/internal/display"
	"github.com/quocvuong92/perplexity-cli/internal/history"
)

// handleCommand processes slash commands in interactive mode.
// Returns true if the session should exit, false otherwise.
func (s *InteractiveSession) handleCommand(input string) bool {
	parts := strings.SplitN(input, " ", 2)
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/exit", "/quit", "/q":
		return s.cmdExit()
	case "/clear", "/c":
		return s.cmdClear()
	case "/retry", "/r":
		return s.cmdRetry()
	case "/export":
		return s.cmdExport(parts)
	case "/help", "/h":
		return s.cmdHelp()
	case "/citations":
		return s.cmdCitations(parts)
	case "/history":
		return s.cmdHistory()
	case "/search":
		return s.cmdSearch(parts)
	case "/delete":
		return s.cmdDelete(parts)
	case "/system":
		return s.cmdSystem(parts)
	case "/copy":
		return s.cmdCopy()
	case "/resume":
		return s.cmdResume(parts)
	case "/model", "/m":
		return s.cmdModel(parts)
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		fmt.Println("Type /help for available commands")
	}

	return false
}

func (s *InteractiveSession) cmdExit() bool {
	fmt.Println("Goodbye!")
	s.saveHistory()
	return true
}

func (s *InteractiveSession) cmdClear() bool {
	s.setMessages([]api.Message{
		{Role: "system", Content: config.DefaultSystemMessage},
	})
	s.conversationID = uuid.New().String()
	s.lastUserInput = ""
	s.lastResponse = ""
	fmt.Println("Conversation cleared.")
	return false
}

func (s *InteractiveSession) cmdRetry() bool {
	if s.lastUserInput == "" {
		fmt.Println("No previous message to retry.")
		return false
	}

	// Remove the last assistant response if it exists
	s.messagesMu.Lock()
	if len(s.messages) > 0 && s.messages[len(s.messages)-1].Role == "assistant" {
		s.messages = s.messages[:len(s.messages)-1]
	}
	// Remove the last user message if it exists
	if len(s.messages) > 0 && s.messages[len(s.messages)-1].Role == "user" {
		s.messages = s.messages[:len(s.messages)-1]
	}
	s.messagesMu.Unlock()

	// Resend the last user input
	fmt.Printf("Retrying: %s\n", s.lastUserInput)
	s.appendMessage(api.Message{Role: "user", Content: s.lastUserInput})
	fmt.Println()

	response, citations, err := s.sendInteractiveMessage()
	if err != nil {
		if err == context.Canceled {
			s.removeLastMessage()
			return false
		}
		msg, hint := display.FormatNetworkError(err)
		display.ShowFriendlyError(msg, hint)
		s.removeLastMessage()
		return false
	}

	if response == "" {
		response = config.FailedResponsePlaceholder
	}
	s.lastResponse = response
	s.appendMessage(api.Message{Role: "assistant", Content: response})

	if s.app.cfg.Citations && len(citations) > 0 {
		fmt.Println()
		display.ShowCitations(citations)
	}
	fmt.Println()
	return false
}

func (s *InteractiveSession) cmdExport(parts []string) bool {
	messages := s.getMessages()
	if len(messages) <= 1 {
		fmt.Println("No conversation to export.")
		return false
	}

	filename := fmt.Sprintf("conversation-%s.md", time.Now().Format("2006-01-02-150405"))
	if len(parts) > 1 {
		filename = strings.TrimSpace(parts[1])
		if !strings.HasSuffix(filename, ".md") {
			filename += ".md"
		}
	}

	var content strings.Builder
	content.WriteString("# Conversation Export\n\n")
	content.WriteString(fmt.Sprintf("**Date:** %s\n", time.Now().Format("2006-01-02 15:04:05")))
	content.WriteString(fmt.Sprintf("**Model:** %s\n\n", s.app.cfg.Model))
	content.WriteString("---\n\n")

	for _, msg := range messages {
		if msg.Role == "system" {
			continue
		}
		if msg.Role == "user" {
			content.WriteString("## You\n\n")
			content.WriteString(msg.Content)
			content.WriteString("\n\n")
		} else if msg.Role == "assistant" {
			content.WriteString("## Assistant\n\n")
			content.WriteString(msg.Content)
			content.WriteString("\n\n")
		}
	}

	if err := os.WriteFile(filename, []byte(content.String()), 0600); err != nil {
		display.ShowError(fmt.Sprintf("Failed to export conversation: %v", err))
	} else {
		fmt.Printf("Conversation exported to %s\n", filename)
	}
	return false
}

func (s *InteractiveSession) cmdHelp() bool {
	fmt.Println("\nCommands:")
	fmt.Printf("  %-24s %s\n", "/exit, /quit, /q", "Exit interactive mode")
	fmt.Printf("  %-24s %s\n", "/clear, /c", "Clear conversation history")
	fmt.Printf("  %-24s %s\n", "/retry, /r", "Retry last message")
	fmt.Printf("  %-24s %s\n", "/copy", "Copy last response to clipboard")
	fmt.Printf("  %-24s %s\n", "/export [filename]", "Export conversation to markdown file")
	fmt.Printf("  %-24s %s\n", "/system [prompt|reset]", "Show/set system prompt")
	fmt.Printf("  %-24s %s\n", "/citations [on|off]", "Toggle or set citations display")
	fmt.Printf("  %-24s %s\n", "/history", "Show recent conversations")
	fmt.Printf("  %-24s %s\n", "/search <keyword>", "Search conversations by keyword")
	fmt.Printf("  %-24s %s\n", "/resume [n]", "Resume conversation (n=index from /history)")
	fmt.Printf("  %-24s %s\n", "/delete <n>", "Delete conversation (n=index from /history)")
	fmt.Printf("  %-24s %s\n", "/model <name>, /m <name>", "Switch model")
	fmt.Printf("  %-24s %s\n", "/model, /m", "Show current model")
	fmt.Printf("  %-24s %s\n", "/help, /h", "Show this help")
	fmt.Println()
	return false
}

func (s *InteractiveSession) cmdCitations(parts []string) bool {
	if len(parts) > 1 {
		arg := strings.ToLower(strings.TrimSpace(parts[1]))
		switch arg {
		case "on", "true", "1":
			s.app.cfg.Citations = true
			fmt.Println("Citations display enabled.")
		case "off", "false", "0":
			s.app.cfg.Citations = false
			fmt.Println("Citations display disabled.")
		default:
			fmt.Printf("Invalid argument: %s. Use 'on' or 'off'.\n", arg)
		}
	} else {
		s.app.cfg.Citations = !s.app.cfg.Citations
		if s.app.cfg.Citations {
			fmt.Println("Citations display enabled.")
		} else {
			fmt.Println("Citations display disabled.")
		}
	}
	return false
}

func (s *InteractiveSession) cmdHistory() bool {
	if s.history == nil {
		fmt.Println("History not available.")
		return false
	}

	conversations := s.history.GetRecentConversations(10)
	if len(conversations) == 0 {
		fmt.Println("No conversation history.")
		return false
	}

	fmt.Println("\nRecent conversations:")
	for i, conv := range conversations {
		msgCount := len(conv.Messages) - 1
		if msgCount < 0 {
			msgCount = 0
		}
		fmt.Printf("  %d. [%s] %s (%d messages)\n",
			i+1,
			conv.UpdatedAt.Format("2006-01-02 15:04"),
			conv.Model,
			msgCount,
		)
	}
	fmt.Println()
	return false
}

func (s *InteractiveSession) cmdSearch(parts []string) bool {
	if s.history == nil {
		fmt.Println("History not available.")
		return false
	}

	if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
		fmt.Println("Usage: /search <keyword>")
		return false
	}

	keyword := strings.TrimSpace(parts[1])
	results := s.history.SearchConversations(keyword)
	if len(results) == 0 {
		fmt.Printf("No conversations found containing '%s'.\n", keyword)
		return false
	}

	fmt.Printf("\nConversations containing '%s':\n", keyword)
	for i, conv := range results {
		msgCount := len(conv.Messages) - 1
		if msgCount < 0 {
			msgCount = 0
		}
		fmt.Printf("  %d. [%s] %s (%d messages)\n",
			i+1,
			conv.UpdatedAt.Format("2006-01-02 15:04"),
			conv.Model,
			msgCount,
		)
	}
	fmt.Println()
	return false
}

func (s *InteractiveSession) cmdDelete(parts []string) bool {
	if s.history == nil {
		fmt.Println("History not available.")
		return false
	}

	if len(parts) < 2 {
		fmt.Println("Usage: /delete <n> (n=index from /history)")
		return false
	}

	indexStr := strings.TrimSpace(parts[1])
	index := 0
	if _, err := fmt.Sscanf(indexStr, "%d", &index); err != nil {
		display.ShowError(fmt.Sprintf("Invalid index: %s", indexStr))
		return false
	}

	if s.history.DeleteConversation(index) {
		if err := s.history.Save(); err != nil {
			display.ShowError(fmt.Sprintf("Failed to save history: %v", err))
		} else {
			fmt.Printf("Conversation %d deleted.\n", index)
		}
	} else {
		display.ShowError(fmt.Sprintf("Invalid conversation index: %d", index))
	}
	return false
}

func (s *InteractiveSession) cmdSystem(parts []string) bool {
	if len(parts) > 1 {
		newPrompt := strings.TrimSpace(parts[1])
		if newPrompt == "" {
			fmt.Println("Usage: /system <prompt> or /system to show current")
		} else if newPrompt == "reset" {
			s.messagesMu.Lock()
			if len(s.messages) > 0 && s.messages[0].Role == "system" {
				s.messages[0].Content = config.DefaultSystemMessage
			}
			s.messagesMu.Unlock()
			fmt.Println("System prompt reset to default.")
		} else {
			s.messagesMu.Lock()
			if len(s.messages) > 0 && s.messages[0].Role == "system" {
				s.messages[0].Content = newPrompt
			}
			s.messagesMu.Unlock()
			fmt.Println("System prompt updated.")
		}
	} else {
		s.messagesMu.RLock()
		if len(s.messages) > 0 && s.messages[0].Role == "system" {
			fmt.Printf("Current system prompt: %s\n", s.messages[0].Content)
		} else {
			fmt.Println("No system prompt set.")
		}
		s.messagesMu.RUnlock()
	}
	return false
}

func (s *InteractiveSession) cmdCopy() bool {
	if s.lastResponse == "" {
		fmt.Println("No response to copy.")
		return false
	}

	if err := copyToClipboard(s.lastResponse); err != nil {
		display.ShowError(fmt.Sprintf("Failed to copy to clipboard: %v", err))
	} else {
		fmt.Println("Response copied to clipboard.")
	}
	return false
}

func (s *InteractiveSession) cmdResume(parts []string) bool {
	if s.history == nil {
		fmt.Println("History not available.")
		return false
	}

	conversations := s.history.GetRecentConversations(10)
	if len(conversations) == 0 {
		fmt.Println("No conversation to resume.")
		return false
	}

	// Determine which conversation to resume
	var conv *history.ConversationEntry
	if len(parts) > 1 {
		indexStr := strings.TrimSpace(parts[1])
		index := 0
		if _, err := fmt.Sscanf(indexStr, "%d", &index); err != nil || index < 1 || index > len(conversations) {
			fmt.Printf("Invalid conversation index: %s (use 1-%d)\n", indexStr, len(conversations))
			return false
		}
		conv = &conversations[index-1]
	} else {
		conv = &conversations[len(conversations)-1]
	}

	// Convert history.Message to api.Message, filtering out failed responses
	newMessages := make([]api.Message, 0, len(conv.Messages))
	for i, msg := range conv.Messages {
		if msg.Role == "assistant" && msg.Content == config.FailedResponsePlaceholder {
			if len(newMessages) > 0 && newMessages[len(newMessages)-1].Role == "user" {
				newMessages = newMessages[:len(newMessages)-1]
			}
			continue
		}
		newMessages = append(newMessages, api.Message{
			Role:    msg.Role,
			Content: conv.Messages[i].Content,
		})
	}
	s.setMessages(newMessages)

	s.conversationID = conv.ID
	msgCount := len(conv.Messages) - 1
	if msgCount < 0 {
		msgCount = 0
	}
	fmt.Printf("Resumed conversation from %s (%d messages)\n\n",
		conv.UpdatedAt.Format("2006-01-02 15:04"),
		msgCount,
	)

	// Display the conversation history
	messages := s.getMessages()
	for _, msg := range messages {
		if msg.Role == "system" {
			continue
		}
		if msg.Role == "user" {
			fmt.Printf("You:\n%s\n\n", msg.Content)
		}
		if msg.Role == "assistant" && msg.Content != "" {
			fmt.Printf("Assistant:\n")
			if s.app.cfg.Render {
				display.ShowContentRendered(msg.Content)
			} else {
				display.ShowContent(msg.Content)
			}
			fmt.Println()
		}
	}

	fmt.Println("--- End of conversation history ---")
	fmt.Println()
	return false
}

func (s *InteractiveSession) cmdModel(parts []string) bool {
	if len(parts) > 1 {
		newModel := strings.TrimSpace(parts[1])
		if newModel == "" {
			fmt.Printf("Current model: %s\n", s.app.cfg.Model)
			fmt.Printf("Available: %s\n", config.GetAvailableModelsString())
		} else if !config.ValidateModel(newModel) {
			fmt.Printf("Invalid model: %s\n", newModel)
			fmt.Printf("Available: %s\n", config.GetAvailableModelsString())
		} else {
			s.app.cfg.Model = newModel
			fmt.Printf("Switched to model: %s\n", s.app.cfg.Model)
		}
	} else {
		fmt.Printf("Current model: %s\n", s.app.cfg.Model)
		fmt.Printf("Available: %s\n", config.GetAvailableModelsString())
	}
	return false
}
