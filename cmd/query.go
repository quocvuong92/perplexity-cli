package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/quocvuong92/perplexity-cli/internal/api"
	"github.com/quocvuong92/perplexity-cli/internal/display"
)

// runNormal executes a single query in non-streaming mode
func (app *App) runNormal(ctx context.Context, query string) {
	sp := display.NewSpinner("Waiting for response...")
	sp.Start()

	resp, err := app.client.QueryContext(ctx, query)
	sp.Stop()

	if err != nil {
		if ctx.Err() != nil {
			return
		}
		msg, hint := display.FormatNetworkError(err)
		display.ShowFriendlyError(msg, hint)
		return
	}

	content := resp.GetContent()

	if app.cfg.Render {
		display.ShowContentRendered(content)
	} else {
		display.ShowContent(content)
	}

	if app.cfg.Citations && len(resp.Citations) > 0 {
		display.ShowCitations(resp.Citations)
	}

	if app.cfg.Usage {
		display.ShowUsage(resp.GetUsageMap())
	}

	// Save to file if output flag is set
	if app.cfg.OutputFile != "" {
		if err := os.WriteFile(app.cfg.OutputFile, []byte(content), 0600); err != nil {
			display.ShowError(fmt.Sprintf("Failed to save output: %v", err))
		} else {
			fmt.Fprintf(os.Stderr, "Response saved to %s\n", app.cfg.OutputFile)
		}
	}
}

// runStream executes a single query in streaming mode
func (app *App) runStream(ctx context.Context, query string) {
	var finalResp *api.ChatResponse
	var fullContent strings.Builder
	firstChunk := true

	sp := display.NewSpinner("Waiting for response...")
	sp.Start()

	err := app.client.QueryStreamContext(ctx, query,
		func(content string) {
			if firstChunk {
				firstChunk = false
				sp.Stop()
			}

			fullContent.WriteString(content)
			fmt.Print(content)
		},
		func(resp *api.ChatResponse) {
			finalResp = resp
		},
	)

	sp.Stop()

	if err != nil {
		if ctx.Err() != nil {
			fmt.Println()
			return
		}
		msg, hint := display.FormatNetworkError(err)
		display.ShowFriendlyError(msg, hint)
		return
	}

	if app.cfg.Render {
		fmt.Println("\n---")
		// Render collected content
		display.ShowContentRendered(fullContent.String())
	} else {
		fmt.Println() // newline after streaming content
	}

	if finalResp != nil {
		if app.cfg.Citations && len(finalResp.Citations) > 0 {
			fmt.Println()
			display.ShowCitations(finalResp.Citations)
		}

		if app.cfg.Usage {
			fmt.Println()
			display.ShowUsage(finalResp.GetUsageMap())
		}
	}

	// Save to file if output flag is set
	if app.cfg.OutputFile != "" {
		if err := os.WriteFile(app.cfg.OutputFile, []byte(fullContent.String()), 0600); err != nil {
			display.ShowError(fmt.Sprintf("Failed to save output: %v", err))
		} else {
			fmt.Fprintf(os.Stderr, "Response saved to %s\n", app.cfg.OutputFile)
		}
	}
}
