package cmd

import (
	"fmt"
	"strings"

	"github.com/c-bata/go-prompt"

	"github.com/quocvuong92/perplexity-cli/internal/api"
	"github.com/quocvuong92/perplexity-cli/internal/config"
	"github.com/quocvuong92/perplexity-cli/internal/display"
)

// InteractiveSession holds the state for interactive mode
type InteractiveSession struct {
	client   *api.Client
	messages []api.Message
	exitFlag bool
}

// runInteractive starts the interactive chat mode
func runInteractive() {
	fmt.Println("Perplexity CLI - Interactive Mode")
	fmt.Printf("Model: %s\n", cfg.Model)
	fmt.Println("Type /help for commands, Ctrl+C or Ctrl+D to quit")
	fmt.Println("Commands auto-complete as you type")
	fmt.Println()

	session := &InteractiveSession{
		client: api.NewClient(cfg),
		messages: []api.Message{
			{Role: "system", Content: config.DefaultSystemMessage},
		},
		exitFlag: false,
	}

	session.client.SetKeyRotationCallback(func(fromIndex, toIndex int, totalKeys int) {
		display.ShowKeyRotation(fromIndex, toIndex, totalKeys)
	})

	p := prompt.New(
		session.executor,
		session.completer,
		prompt.OptionPrefix("> "),
		prompt.OptionTitle("Perplexity CLI"),
		prompt.OptionPrefixTextColor(prompt.Green),
		prompt.OptionSuggestionBGColor(prompt.DarkGray),
		prompt.OptionSuggestionTextColor(prompt.White),
		prompt.OptionSelectedSuggestionBGColor(prompt.LightGray),
		prompt.OptionSelectedSuggestionTextColor(prompt.Black),
		prompt.OptionDescriptionBGColor(prompt.DarkGray),
		prompt.OptionDescriptionTextColor(prompt.White),
		prompt.OptionSelectedDescriptionBGColor(prompt.LightGray),
		prompt.OptionSelectedDescriptionTextColor(prompt.Black),
		prompt.OptionMaxSuggestion(10),
		prompt.OptionCompletionOnDown(),
		prompt.OptionAddKeyBind(prompt.KeyBind{
			Key: prompt.ControlC,
			Fn: func(buf *prompt.Buffer) {
				fmt.Println("\nGoodbye!")
				panic("exit")
			},
		}),
		prompt.OptionAddKeyBind(prompt.KeyBind{
			Key: prompt.ControlD,
			Fn: func(buf *prompt.Buffer) {
				if buf.Text() == "" {
					fmt.Println("Goodbye!")
					panic("exit")
				}
			},
		}),
	)

	// Recover from panic used for exit
	defer func() {
		if r := recover(); r != nil {
			if r != "exit" {
				// Re-panic if it's not our exit signal
				panic(r)
			}
		}
	}()

	p.Run()
}

// executor handles the execution of each input line
func (s *InteractiveSession) executor(input string) {
	// Check if we should exit
	if s.exitFlag {
		panic("exit")
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return
	}

	// Handle commands
	if strings.HasPrefix(input, "/") {
		if handleCommand(input, &s.messages) {
			fmt.Println("Goodbye!")
			s.exitFlag = true
			panic("exit")
		}
		return
	}

	// Regular chat
	s.messages = append(s.messages, api.Message{Role: "user", Content: input})
	fmt.Println()
	response, citations, err := sendInteractiveMessage(s.client, s.messages)
	if err != nil {
		display.ShowError(err.Error())
		s.messages = s.messages[:len(s.messages)-1]
		return
	}
	if response != "" {
		s.messages = append(s.messages, api.Message{Role: "assistant", Content: response})
	}
	if cfg.Citations && len(citations) > 0 {
		fmt.Println()
		display.ShowCitations(citations)
	}
	fmt.Println()
}

// completer provides auto-suggestions for commands
func (s *InteractiveSession) completer(d prompt.Document) []prompt.Suggest {
	// Only show suggestions when input starts with "/"
	text := d.TextBeforeCursor()
	if !strings.HasPrefix(text, "/") {
		return []prompt.Suggest{}
	}

	suggestions := []prompt.Suggest{
		{Text: "/quit", Description: "Exit interactive mode"},
		{Text: "/q", Description: "Exit interactive mode (short)"},
		{Text: "/clear", Description: "Clear conversation history"},
		{Text: "/c", Description: "Clear conversation history (short)"},
		{Text: "/help", Description: "Show available commands"},
		{Text: "/h", Description: "Show available commands (short)"},
		{Text: "/model", Description: "Show/switch model"},
		{Text: "/m", Description: "Show/switch model (short)"},
	}

	return prompt.FilterHasPrefix(suggestions, d.GetWordBeforeCursor(), true)
}

// handleCommand processes slash commands in interactive mode
func handleCommand(input string, messages *[]api.Message) bool {
	parts := strings.SplitN(input, " ", 2)
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/quit", "/q":
		fmt.Println("Goodbye!")
		return true

	case "/clear", "/c":
		*messages = []api.Message{
			{Role: "system", Content: config.DefaultSystemMessage},
		}
		fmt.Println("Conversation cleared.")

	case "/help", "/h":
		fmt.Println("\nCommands:")
		fmt.Printf("  %-24s %s\n", "/quit, /q", "Exit interactive mode")
		fmt.Printf("  %-24s %s\n", "/clear, /c", "Clear conversation history")
		fmt.Printf("  %-24s %s\n", "/model <name>, /m <name>", "Switch model")
		fmt.Printf("  %-24s %s\n", "/model, /m", "Show current model")
		fmt.Printf("  %-24s %s\n", "/help, /h", "Show this help")
		fmt.Println()

	case "/model", "/m":
		if len(parts) > 1 {
			newModel := strings.TrimSpace(parts[1])
			if newModel == "" {
				fmt.Printf("Current model: %s\n", cfg.Model)
				fmt.Printf("Available: %s\n", config.GetAvailableModelsString())
			} else if !config.ValidateModel(newModel) {
				fmt.Printf("Invalid model: %s\n", newModel)
				fmt.Printf("Available: %s\n", config.GetAvailableModelsString())
			} else {
				cfg.Model = newModel
				fmt.Printf("Switched to model: %s\n", cfg.Model)
			}
		} else {
			fmt.Printf("Current model: %s\n", cfg.Model)
			fmt.Printf("Available: %s\n", config.GetAvailableModelsString())
		}

	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		fmt.Println("Type /help for available commands")
	}

	return false
}

// sendInteractiveMessage sends a message in interactive mode and returns the response
func sendInteractiveMessage(client *api.Client, messages []api.Message) (string, []string, error) {
	if cfg.Stream {
		var fullContent strings.Builder
		var citations []string
		firstChunk := true

		sp := display.NewSpinner("Thinking...")
		sp.Start()

		err := client.QueryStreamWithHistory(messages,
			func(content string) {
				if firstChunk {
					firstChunk = false
					if cfg.Render {
						sp.UpdateMessage("Receiving...")
					} else {
						sp.Stop()
					}
				}
				if cfg.Render {
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

		if cfg.Render {
			display.ShowContentRendered(fullContent.String())
			return fullContent.String(), citations, nil
		}
		fmt.Println()
		return fullContent.String(), citations, nil
	}

	// Non-streaming
	sp := display.NewSpinner("Thinking...")
	sp.Start()

	resp, err := client.QueryWithHistory(messages)
	sp.Stop()

	if err != nil {
		return "", nil, err
	}

	content := resp.GetContent()
	if cfg.Render {
		display.ShowContentRendered(content)
	} else {
		display.ShowContent(content)
	}

	return content, resp.Citations, nil
}
