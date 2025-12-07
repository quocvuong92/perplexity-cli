package cmd

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/chzyer/readline"
	"github.com/spf13/cobra"

	"github.com/quocvuong92/perplexity-cli/internal/api"
	"github.com/quocvuong92/perplexity-cli/internal/config"
	"github.com/quocvuong92/perplexity-cli/internal/display"
)

var (
	cfg     *config.Config
	verbose bool
)

// Command items for autocomplete
var commandItems = []readline.PrefixCompleterInterface{
	readline.PcItem("/exit"),
	readline.PcItem("/quit"),
	readline.PcItem("/q"),
	readline.PcItem("/clear"),
	readline.PcItem("/c"),
	readline.PcItem("/help"),
	readline.PcItem("/h"),
	readline.PcItem("/model"),
}

var rootCmd = &cobra.Command{
	Use:   "perplexity [query]",
	Short: "A CLI client for the Perplexity API",
	Long: `Perplexity CLI is a simple and convenient command-line client
for the Perplexity API, allowing users to quickly ask questions
and receive answers directly from the terminal.

Output is in markdown format for easy copying.`,
	Args: cobra.MaximumNArgs(1),
	Run:  run,
}

func init() {
	cfg = config.NewConfig()

	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable debug mode")
	rootCmd.Flags().BoolVarP(&cfg.Usage, "usage", "u", false, "Show token usage statistics")
	rootCmd.Flags().BoolVarP(&cfg.Citations, "citations", "c", false, "Show citations")
	rootCmd.Flags().BoolVarP(&cfg.Stream, "stream", "s", false, "Stream output in real-time")
	rootCmd.Flags().BoolVarP(&cfg.Render, "render", "r", false, "Render markdown with colors and formatting")
	rootCmd.Flags().BoolVarP(&cfg.Interactive, "interactive", "i", false, "Interactive chat mode")
	rootCmd.Flags().StringVarP(&cfg.APIKey, "api-key", "a", "", "API key (defaults to PERPLEXITY_API_KEYS or PERPLEXITY_API_KEY env var)")
	rootCmd.Flags().StringVarP(&cfg.Model, "model", "m", config.DefaultModel,
		fmt.Sprintf("Model to use. Available: %s", config.GetAvailableModelsString()))
}

func run(cmd *cobra.Command, args []string) {
	if verbose {
		log.SetOutput(os.Stderr)
		log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	} else {
		log.SetOutput(io.Discard)
	}

	if err := cfg.Validate(); err != nil {
		display.ShowError(err.Error())
		os.Exit(1)
	}

	// Initialize markdown renderer if render flag is set
	if cfg.Render {
		if err := display.InitRenderer(); err != nil {
			log.Printf("Failed to initialize renderer: %v", err)
		}
	}

	// Interactive mode
	if cfg.Interactive {
		runInteractive()
		return
	}

	// Require query if not interactive mode
	if len(args) == 0 {
		_ = cmd.Help()
		os.Exit(1)
	}

	query := args[0]
	log.Printf("Query: %s", query)
	log.Printf("Model: %s", cfg.Model)
	log.Printf("Stream: %v", cfg.Stream)

	client := api.NewClient(cfg)

	// Set up key rotation callback to notify user
	client.SetKeyRotationCallback(func(fromIndex, toIndex int, totalKeys int) {
		display.ShowKeyRotation(fromIndex, toIndex, totalKeys)
	})

	log.Printf("Sending request to API...")

	if cfg.Stream {
		runStream(client, query)
	} else {
		runNormal(client, query)
	}
}

func runNormal(client *api.Client, query string) {
	sp := display.NewSpinner("Waiting for response...")
	sp.Start()

	resp, err := client.Query(query)
	sp.Stop()

	if err != nil {
		display.ShowError(err.Error())
		os.Exit(1)
	}

	if cfg.Render {
		display.ShowContentRendered(resp.GetContent())
	} else {
		display.ShowContent(resp.GetContent())
	}

	if cfg.Citations && len(resp.Citations) > 0 {
		display.ShowCitations(resp.Citations)
	}

	if cfg.Usage {
		display.ShowUsage(resp.GetUsageMap())
	}
}

func runStream(client *api.Client, query string) {
	var finalResp *api.ChatResponse
	var fullContent strings.Builder
	firstChunk := true

	// Show spinner while waiting for first content
	sp := display.NewSpinner("Waiting for response...")
	sp.Start()

	err := client.QueryStream(query,
		func(content string) {
			if firstChunk {
				firstChunk = false
				if cfg.Render {
					// Update spinner message - keep spinning while collecting content
					sp.UpdateMessage("Receiving response...")
				} else {
					// Stop spinner for non-render streaming (show content immediately)
					sp.Stop()
				}
			}

			if cfg.Render {
				// Collect content for rendering at the end
				fullContent.WriteString(content)
			} else {
				fmt.Print(content)
			}
		},
		func(resp *api.ChatResponse) {
			finalResp = resp
		},
	)

	// Stop spinner (either still running in render mode, or already stopped)
	sp.Stop()

	if err != nil {
		display.ShowError(err.Error())
		os.Exit(1)
	}

	if cfg.Render {
		// Render collected content
		display.ShowContentRendered(fullContent.String())
	} else {
		fmt.Println() // newline after streaming content
	}

	if finalResp != nil {
		if cfg.Citations && len(finalResp.Citations) > 0 {
			fmt.Println()
			display.ShowCitations(finalResp.Citations)
		}

		if cfg.Usage {
			fmt.Println()
			display.ShowUsage(finalResp.GetUsageMap())
		}
	}
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runInteractive() {
	fmt.Println("Perplexity CLI - Interactive Mode")
	fmt.Printf("Model: %s\n", cfg.Model)
	fmt.Println("Type /help for commands, Ctrl+C to quit, Tab for autocomplete")
	fmt.Println()

	// Setup readline with autocomplete
	completer := readline.NewPrefixCompleter(commandItems...)

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "> ",
		AutoComplete:    completer,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		display.ShowError(err.Error())
		return
	}
	defer rl.Close()

	client := api.NewClient(cfg)
	client.SetKeyRotationCallback(func(fromIndex, toIndex int, totalKeys int) {
		display.ShowKeyRotation(fromIndex, toIndex, totalKeys)
	})

	messages := []api.Message{
		{Role: "system", Content: "Be precise and concise."},
	}

	for {
		line, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				fmt.Println("Goodbye!")
				return
			} else if err == io.EOF {
				fmt.Println("Goodbye!")
				return
			}
			continue
		}

		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}

		// Handle commands
		if strings.HasPrefix(input, "/") {
			if handleCommand(input, &messages) {
				return
			}
			continue
		}

		// Regular chat
		messages = append(messages, api.Message{Role: "user", Content: input})
		fmt.Println()
		response, citations, err := sendInteractiveMessage(client, messages)
		if err != nil {
			display.ShowError(err.Error())
			messages = messages[:len(messages)-1]
			continue
		}
		messages = append(messages, api.Message{Role: "assistant", Content: response})

		// Show citations if enabled
		if cfg.Citations && len(citations) > 0 {
			fmt.Println()
			display.ShowCitations(citations)
		}
		fmt.Println()
	}
}

func handleCommand(input string, messages *[]api.Message) bool {
	parts := strings.SplitN(input, " ", 2)
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/exit", "/quit", "/q":
		fmt.Println("Goodbye!")
		return true

	case "/clear", "/c":
		*messages = []api.Message{
			{Role: "system", Content: "Be precise and concise."},
		}
		fmt.Println("Conversation cleared.")

	case "/help", "/h":
		fmt.Println(`
Commands:
  /exit, /quit, /q  - Exit interactive mode
  /clear, /c        - Clear conversation history
  /model <name>     - Switch model
  /model            - Show current model
  /help, /h         - Show this help
`)

	case "/model":
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
