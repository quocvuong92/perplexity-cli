package cmd

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/quocvuong92/perplexity-cli/internal/api"
	"github.com/quocvuong92/perplexity-cli/internal/config"
	"github.com/quocvuong92/perplexity-cli/internal/display"
)

// App holds the application state
type App struct {
	cfg        *config.Config
	client     *api.Client
	verbose    bool
	listModels bool
}

// NewApp creates a new App instance with default configuration
func NewApp() *App {
	return &App{
		cfg: config.NewConfig(),
	}
}

// Execute runs the root command
func Execute() {
	app := NewApp()

	rootCmd := &cobra.Command{
		Use:   "perplexity [query]",
		Short: "A CLI client for the Perplexity API",
		Long: `Perplexity CLI is a simple and convenient command-line client
for the Perplexity API, allowing users to quickly ask questions
and receive answers directly from the terminal.

Output is in markdown format for easy copying.`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			app.run(cmd, args)
		},
	}

	rootCmd.Flags().BoolVarP(&app.verbose, "verbose", "v", false, "Enable debug mode")
	rootCmd.Flags().BoolVarP(&app.cfg.Usage, "usage", "u", false, "Show token usage statistics")
	rootCmd.Flags().BoolVarP(&app.cfg.Citations, "citations", "c", false, "Show citations")
	rootCmd.Flags().BoolVarP(&app.cfg.Stream, "stream", "s", false, "Stream output in real-time")
	rootCmd.Flags().BoolVarP(&app.cfg.Render, "render", "r", false, "Render markdown with colors and formatting")
	rootCmd.Flags().BoolVarP(&app.cfg.Interactive, "interactive", "i", false, "Interactive chat mode")
	rootCmd.Flags().StringVarP(&app.cfg.APIKey, "api-key", "a", "", "API key (defaults to PERPLEXITY_API_KEYS or PERPLEXITY_API_KEY env var)")
	rootCmd.Flags().StringVarP(&app.cfg.Model, "model", "m", config.DefaultModel,
		fmt.Sprintf("Model to use. Available: %s", config.GetAvailableModelsString()))
	rootCmd.Flags().StringVarP(&app.cfg.OutputFile, "output", "o", "", "Save response to file")
	rootCmd.Flags().BoolVar(&app.listModels, "list-models", false, "List available models")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func (app *App) run(cmd *cobra.Command, args []string) {
	if app.verbose {
		log.SetOutput(os.Stderr)
		log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	} else {
		log.SetOutput(io.Discard)
	}

	// Handle --list-models flag (doesn't require API key)
	if app.listModels {
		display.ShowModels(config.AvailableModels, app.cfg.Model)
		return
	}

	if err := app.cfg.Validate(); err != nil {
		display.ShowError(err.Error())
		os.Exit(1)
	}

	// Initialize markdown renderer if render flag is set
	if app.cfg.Render {
		if err := display.InitRenderer(); err != nil {
			log.Printf("Failed to initialize renderer: %v", err)
		}
	}

	// Interactive mode
	if app.cfg.Interactive {
		app.runInteractive()
		return
	}

	// Get query from args or stdin (pipe)
	var query string
	if len(args) > 0 {
		query = args[0]
	} else {
		// Check if there's input from pipe
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			// Data is being piped
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				display.ShowError(fmt.Sprintf("Failed to read from stdin: %v", err))
				os.Exit(1)
			}
			query = strings.TrimSpace(string(data))
		}
	}

	// Require query
	if query == "" {
		_ = cmd.Help()
		os.Exit(1)
	}
	log.Printf("Query: %s", query)
	log.Printf("Model: %s", app.cfg.Model)
	log.Printf("Stream: %v", app.cfg.Stream)

	app.client = api.NewClient(app.cfg)

	// Set up key rotation callback to notify user
	app.client.SetKeyRotationCallback(func(fromIndex, toIndex int, totalKeys int) {
		display.ShowKeyRotation(fromIndex, toIndex, totalKeys)
	})

	log.Printf("Sending request to API...")

	if app.cfg.Stream {
		app.runStream(query)
	} else {
		app.runNormal(query)
	}
}
