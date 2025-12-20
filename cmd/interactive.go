package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/elk-language/go-prompt"
	istrings "github.com/elk-language/go-prompt/strings"
	"github.com/google/uuid"

	"github.com/quocvuong92/perplexity-cli/internal/api"
	"github.com/quocvuong92/perplexity-cli/internal/config"
	"github.com/quocvuong92/perplexity-cli/internal/display"
	"github.com/quocvuong92/perplexity-cli/internal/history"
)

// InteractiveSession holds the state for interactive mode
type InteractiveSession struct {
	app            *App
	client         *api.Client
	messages       []api.Message
	exitFlag       bool
	inputBuffer    []string // Buffer for multiline input
	history        *history.History
	conversationID string
}

// runInteractive starts the interactive chat mode
func (app *App) runInteractive() {
	fmt.Println("Perplexity CLI - Interactive Mode")
	fmt.Printf("Model: %s\n", app.cfg.Model)
	fmt.Println("Type /help for commands, Ctrl+C or Ctrl+D to quit")
	fmt.Println("Commands auto-complete as you type")
	fmt.Println("End a line with \\ for multiline input")
	fmt.Println()

	// Initialize history
	hist := history.NewHistory()
	if err := hist.Load(); err != nil {
		// History load failed, continue without it
		fmt.Fprintf(os.Stderr, "Note: Could not load history: %v\n", err)
	}

	client := api.NewClient(app.cfg)

	session := &InteractiveSession{
		app:    app,
		client: client,
		messages: []api.Message{
			{Role: "system", Content: config.DefaultSystemMessage},
		},
		exitFlag:       false,
		history:        hist,
		conversationID: uuid.New().String(),
	}

	session.client.SetKeyRotationCallback(func(fromIndex, toIndex int, totalKeys int) {
		display.ShowKeyRotation(fromIndex, toIndex, totalKeys)
	})

	p := prompt.New(
		session.executor,
		prompt.WithCompleter(session.completer),
		prompt.WithPrefix("> "),
		prompt.WithTitle("Perplexity CLI"),
		prompt.WithPrefixTextColor(prompt.Green),
		// Suggestion box styling - better contrast and visibility
		prompt.WithSuggestionBGColor(prompt.DarkBlue),
		prompt.WithSuggestionTextColor(prompt.White),
		prompt.WithSelectedSuggestionBGColor(prompt.Cyan),
		prompt.WithSelectedSuggestionTextColor(prompt.Black),
		prompt.WithDescriptionBGColor(prompt.DarkBlue),
		prompt.WithDescriptionTextColor(prompt.LightGray),
		prompt.WithSelectedDescriptionBGColor(prompt.Cyan),
		prompt.WithSelectedDescriptionTextColor(prompt.Black),
		prompt.WithScrollbarBGColor(prompt.DarkGray),
		prompt.WithScrollbarThumbColor(prompt.White),
		// Show more suggestions at once
		prompt.WithMaxSuggestion(15),
		prompt.WithCompletionOnDown(),
		prompt.WithExitChecker(func(in string, breakline bool) bool {
			return session.exitFlag
		}),
		prompt.WithKeyBind(prompt.KeyBind{
			Key: prompt.ControlC,
			Fn: func(p *prompt.Prompt) bool {
				fmt.Println("\nGoodbye!")
				session.saveHistory()
				session.exitFlag = true
				return false
			},
		}),
		prompt.WithKeyBind(prompt.KeyBind{
			Key: prompt.ControlD,
			Fn: func(p *prompt.Prompt) bool {
				if p.Buffer().Text() == "" {
					fmt.Println("Goodbye!")
					session.saveHistory()
					session.exitFlag = true
				}
				return false
			},
		}),
	)

	p.Run()
}

// saveHistory persists the current conversation to the history file.
// Only saves if there are messages beyond the initial system prompt.
func (s *InteractiveSession) saveHistory() {
	if s.history == nil {
		return
	}
	// Only save if there are messages beyond the system prompt
	if len(s.messages) > 1 {
		// Convert api.Message to history.Message
		historyMessages := make([]history.Message, len(s.messages))
		for i, msg := range s.messages {
			historyMessages[i] = history.Message{
				Role:    msg.Role,
				Content: msg.Content,
			}
		}
		s.history.AddConversation(
			s.conversationID,
			s.app.cfg.Model,
			historyMessages,
		)
		if err := s.history.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not save history: %v\n", err)
		}
	}
}

// executor handles the execution of each input line
func (s *InteractiveSession) executor(input string) {
	// Check if we should exit
	if s.exitFlag {
		return
	}

	// Handle multiline input with backslash continuation
	if strings.HasSuffix(input, "\\") {
		// Remove the trailing backslash and add to buffer
		line := strings.TrimSuffix(input, "\\")
		s.inputBuffer = append(s.inputBuffer, line)
		fmt.Print("... ") // Show continuation prompt
		return
	}

	// If we have buffered lines, combine them with current input
	if len(s.inputBuffer) > 0 {
		s.inputBuffer = append(s.inputBuffer, input)
		input = strings.Join(s.inputBuffer, "\n")
		s.inputBuffer = nil // Clear the buffer
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return
	}

	// Handle commands (only if not in multiline mode - first line determines if it's a command)
	if strings.HasPrefix(input, "/") {
		if s.handleCommand(input) {
			s.exitFlag = true
		}
		return
	}

	// Regular chat
	s.messages = append(s.messages, api.Message{Role: "user", Content: input})
	fmt.Println()
	response, citations, err := s.sendInteractiveMessage()
	if err != nil {
		display.ShowError(err.Error())
		s.messages = s.messages[:len(s.messages)-1]
		return
	}
	// Always append assistant response to maintain alternating user/assistant pattern
	// Use placeholder if response is empty to satisfy API requirements
	if response == "" {
		response = "I apologize, but I couldn't generate a response."
	}
	s.messages = append(s.messages, api.Message{Role: "assistant", Content: response})
	if s.app.cfg.Citations && len(citations) > 0 {
		fmt.Println()
		display.ShowCitations(citations)
	}
	fmt.Println()
}

// completer provides auto-completion suggestions for slash commands.
// It provides context-aware suggestions based on what the user is typing.
func (s *InteractiveSession) completer(d prompt.Document) ([]prompt.Suggest, istrings.RuneNumber, istrings.RuneNumber) {
	text := d.TextBeforeCursor()
	endIndex := d.CurrentRuneIndex()
	w := d.GetWordBeforeCursor()
	startIndex := endIndex - istrings.RuneCountInString(w)

	// Only show suggestions when input starts with "/"
	if !strings.HasPrefix(text, "/") {
		return []prompt.Suggest{}, startIndex, endIndex
	}

	// Context-aware suggestions based on command being typed
	textLower := strings.ToLower(text)

	// /model <name> - suggest available models
	if strings.HasPrefix(textLower, "/model ") || strings.HasPrefix(textLower, "/m ") {
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
	if strings.HasPrefix(textLower, "/citations ") {
		suggestions := []prompt.Suggest{
			{Text: "on", Description: "Enable citations display"},
			{Text: "off", Description: "Disable citations display"},
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
		{Text: "/citations", Description: "Toggle citations display (current: " + citationsStatus + ")"},
		{Text: "/clear", Description: "Clear conversation history"},
		{Text: "/help", Description: "Show all available commands"},
		{Text: "/exit", Description: "Exit interactive mode"},

		// History commands
		{Text: "/history", Description: "Show recent conversations"},
		{Text: "/resume", Description: "Resume last conversation"},

		// Aliases
		{Text: "/q", Description: "Exit (alias)"},
		{Text: "/c", Description: "Clear (alias)"},
		{Text: "/h", Description: "Help (alias)"},
		{Text: "/m", Description: "Model (alias)"},
	}

	return prompt.FilterHasPrefix(suggestions, w, true), startIndex, endIndex
}

// handleCommand processes slash commands in interactive mode.
// Returns true if the session should exit, false otherwise.
func (s *InteractiveSession) handleCommand(input string) bool {
	parts := strings.SplitN(input, " ", 2)
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/exit", "/quit", "/q":
		fmt.Println("Goodbye!")
		s.saveHistory()
		return true

	case "/clear", "/c":
		s.messages = []api.Message{
			{Role: "system", Content: config.DefaultSystemMessage},
		}
		// Start a new conversation ID when clearing
		s.conversationID = uuid.New().String()
		fmt.Println("Conversation cleared.")

	case "/help", "/h":
		fmt.Println("\nCommands:")
		fmt.Printf("  %-24s %s\n", "/exit, /quit, /q", "Exit interactive mode")
		fmt.Printf("  %-24s %s\n", "/clear, /c", "Clear conversation history")
		fmt.Printf("  %-24s %s\n", "/citations [on|off]", "Toggle or set citations display")
		fmt.Printf("  %-24s %s\n", "/history", "Show recent conversations")
		fmt.Printf("  %-24s %s\n", "/resume", "Resume last conversation")
		fmt.Printf("  %-24s %s\n", "/model <name>, /m <name>", "Switch model")
		fmt.Printf("  %-24s %s\n", "/model, /m", "Show current model")
		fmt.Printf("  %-24s %s\n", "/help, /h", "Show this help")
		fmt.Println()

	case "/citations":
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
			// Toggle
			s.app.cfg.Citations = !s.app.cfg.Citations
			if s.app.cfg.Citations {
				fmt.Println("Citations display enabled.")
			} else {
				fmt.Println("Citations display disabled.")
			}
		}

	case "/history":
		if s.history == nil {
			fmt.Println("History not available.")
		} else {
			conversations := s.history.GetRecentConversations(10)
			if len(conversations) == 0 {
				fmt.Println("No conversation history.")
			} else {
				fmt.Println("\nRecent conversations:")
				for i, conv := range conversations {
					msgCount := len(conv.Messages) - 1 // Exclude system message
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
			}
		}

	case "/resume":
		if s.history == nil {
			fmt.Println("History not available.")
		} else {
			lastConv := s.history.GetLastConversation()
			if lastConv == nil {
				fmt.Println("No conversation to resume.")
			} else {
				// Convert history.Message to api.Message
				s.messages = make([]api.Message, len(lastConv.Messages))
				for i, msg := range lastConv.Messages {
					s.messages[i] = api.Message{
						Role:    msg.Role,
						Content: msg.Content,
					}
				}
				s.conversationID = lastConv.ID
				msgCount := len(lastConv.Messages) - 1
				if msgCount < 0 {
					msgCount = 0
				}
				fmt.Printf("Resumed conversation from %s (%d messages)\n",
					lastConv.UpdatedAt.Format("2006-01-02 15:04"),
					msgCount,
				)
			}
		}

	case "/model", "/m":
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

	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		fmt.Println("Type /help for available commands")
	}

	return false
}

// sendInteractiveMessage sends a message in interactive mode and returns the response
func (s *InteractiveSession) sendInteractiveMessage() (string, []string, error) {
	if s.app.cfg.Stream {
		var fullContent strings.Builder
		var citations []string
		firstChunk := true

		sp := display.NewSpinner("Thinking...")
		sp.Start()

		err := s.client.QueryStreamWithHistory(s.messages,
			func(content string) {
				if firstChunk {
					firstChunk = false
					if s.app.cfg.Render {
						sp.UpdateMessage("Receiving...")
					} else {
						sp.Stop()
					}
				}
				if s.app.cfg.Render {
					fullContent.WriteString(content)
				} else {
					fmt.Print(content)
				}
			},
			func(resp *api.ChatResponse) {
				if resp != nil {
					citations = resp.Citations
				}
			},
		)

		sp.Stop()

		if err != nil {
			return "", nil, err
		}

		if s.app.cfg.Render {
			display.ShowContentRendered(fullContent.String())
			return fullContent.String(), citations, nil
		}
		fmt.Println()
		return fullContent.String(), citations, nil
	}

	// Non-streaming
	sp := display.NewSpinner("Thinking...")
	sp.Start()

	resp, err := s.client.QueryWithHistory(s.messages)
	sp.Stop()

	if err != nil {
		return "", nil, err
	}

	content := resp.GetContent()
	if s.app.cfg.Render {
		display.ShowContentRendered(content)
	} else {
		display.ShowContent(content)
	}

	return content, resp.Citations, nil
}
