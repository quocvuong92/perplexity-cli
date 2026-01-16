package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/quocvuong92/perplexity-cli/internal/api"
	"github.com/quocvuong92/perplexity-cli/internal/config"
	"github.com/quocvuong92/perplexity-cli/internal/display"
	"github.com/quocvuong92/perplexity-cli/internal/logging"
	"github.com/quocvuong92/perplexity-cli/internal/retry"
	"github.com/quocvuong92/perplexity-cli/internal/validation"
)

var Version = "dev"

// App holds the application state
type App struct {
	cfg        *config.Config
	client     *api.Client
	verbose    bool
	listModels bool
	noColor    bool
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
	rootCmd.Flags().BoolVar(&app.noColor, "no-color", false, "Disable colored output")
	rootCmd.Version = Version

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func (app *App) run(cmd *cobra.Command, args []string) {
	// Initialize structured logging
	if app.verbose {
		logging.Init(logging.Config{
			Level:   logging.LevelDebug,
			Output:  os.Stderr,
			Verbose: true,
		})
	} else {
		logging.Init(logging.Config{
			Output: io.Discard,
		})
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
			logging.Warn("Failed to initialize renderer", logging.Err(err))
		}
	}

	// Interactive mode
	if app.cfg.Interactive {
		app.runInteractive(app.shouldUseColor())
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

	// Validate and sanitize the query
	query = validation.SanitizePrompt(query)
	result := validation.ValidatePrompt(query)
	if !result.Valid {
		display.ShowError(result.Error.Error())
		os.Exit(1)
	}
	query = result.Cleaned

	logging.Debug("Processing query",
		logging.String("query", query),
		logging.String("model", app.cfg.Model),
		logging.Bool("stream", app.cfg.Stream),
	)

	app.client = api.NewClient(app.cfg)

	// Set up key rotation callback to notify user
	app.client.SetKeyRotationCallback(func(fromIndex, toIndex int, totalKeys int) {
		display.ShowKeyRotation(fromIndex, toIndex, totalKeys)
	})

	// Set up retry callback to notify user of network retries
	app.client.SetRetryCallback(func(info retry.RetryInfo) {
		display.ShowRetry(info.Attempt+1, info.MaxRetries, info.NextBackoff)
	})

	logging.Debug("Sending request to API")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Fprintln(os.Stderr, "\nInterrupted")
		cancel()
	}()

	if app.cfg.Stream {
		app.runStream(ctx, query)
	} else {
		app.runNormal(ctx, query)
	}
}

// shouldUseColor determines if colored output should be used
func (app *App) shouldUseColor() bool {
	// Explicit --no-color flag takes precedence
	if app.noColor {
		return false
	}

	// Check NO_COLOR environment variable (https://no-color.org/)
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	// Check if stdout is a TTY
	if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) == 0 {
		return false
	}

	return true
}
