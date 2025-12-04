package cmd

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/spf13/cobra"

	"github.com/quocvuong92/perplexity-api/internal/api"
	"github.com/quocvuong92/perplexity-api/internal/config"
	"github.com/quocvuong92/perplexity-api/internal/display"
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
and receive answers directly from the terminal.`,
	Args: cobra.ExactArgs(1),
	Run:  run,
}

func init() {
	cfg = config.NewConfig()

	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable debug mode")
	rootCmd.Flags().BoolVarP(&cfg.Usage, "usage", "u", false, "Show token usage statistics")
	rootCmd.Flags().BoolVarP(&cfg.Citations, "citations", "c", false, "Show citations")
	rootCmd.Flags().BoolVarP(&cfg.Glow, "glow", "g", false, "Use Glow-compatible formatting")
	rootCmd.Flags().StringVarP(&cfg.APIKey, "api-key", "a", "", "API key (defaults to PERPLEXITY_API_KEY env var)")
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

	if err := cfg.Validate(); err != nil {
		display.ShowError(err.Error())
		os.Exit(1)
	}

	client := api.NewClient(cfg)
	log.Printf("Sending request to API...")

	resp, err := client.Query(query)
	if err != nil {
		display.ShowError(err.Error())
		os.Exit(1)
	}

	if cfg.Citations && len(resp.Citations) > 0 {
		display.ShowCitations(resp.Citations, cfg.Glow)
	}

	if cfg.Usage {
		display.ShowUsage(resp.GetUsageMap(), cfg.Glow)
	}

	display.ShowContent(resp.GetContent(), cfg.Glow)
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
