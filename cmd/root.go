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

var (
	cfg     *config.Config
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "perplexity [query]",
	Short: "A CLI client for the Perplexity API",
	Long: `Perplexity CLI is a simple and convenient command-line client
for the Perplexity API, allowing users to quickly ask questions
and receive answers directly from the terminal.

Output is in markdown format for easy copying.`,
	Args: cobra.ExactArgs(1),
	Run:  run,
}

func init() {
	cfg = config.NewConfig()

	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable debug mode")
	rootCmd.Flags().BoolVarP(&cfg.Usage, "usage", "u", false, "Show token usage statistics")
	rootCmd.Flags().BoolVarP(&cfg.Citations, "citations", "c", false, "Show citations")
	rootCmd.Flags().BoolVarP(&cfg.Stream, "stream", "s", false, "Stream output in real-time")
	rootCmd.Flags().BoolVarP(&cfg.Render, "render", "r", false, "Render markdown with colors and formatting")
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

	query := args[0]
	log.Printf("Query: %s", query)
	log.Printf("Model: %s", cfg.Model)
	log.Printf("Stream: %v", cfg.Stream)

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
	resp, err := client.Query(query)
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

	err := client.QueryStream(query,
		func(content string) {
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
