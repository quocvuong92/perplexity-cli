package display

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/glamour"
)

// renderer is the markdown renderer instance
var (
	renderer     *glamour.TermRenderer
	rendererOnce sync.Once
	rendererErr  error
)

// InitRenderer initializes the markdown renderer
func InitRenderer() error {
	rendererOnce.Do(func() {
		r, err := glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(100),
		)
		if err != nil {
			rendererErr = err
			return
		}
		renderer = r
	})
	return rendererErr
}

// ShowUsage displays token usage statistics in markdown format
func ShowUsage(usage map[string]int) {
	fmt.Println("## Tokens")
	fmt.Println()
	fmt.Println("| Type | Count |")
	fmt.Println("|------|-------|")
	fmt.Printf("| Prompt | %d |\n", usage["prompt_tokens"])
	fmt.Printf("| Completion | %d |\n", usage["completion_tokens"])
	fmt.Printf("| **Total** | **%d** |\n", usage["total_tokens"])
	fmt.Println()
}

// ShowCitations displays the citations list in markdown format
func ShowCitations(citations []string) {
	fmt.Println("## Citations")
	fmt.Println()
	for i, citation := range citations {
		fmt.Printf("%d. %s\n", i+1, citation)
	}
	fmt.Println()
}

// ShowContent displays the main content response
func ShowContent(content string) {
	fmt.Println(strings.TrimSpace(content))
}

// ShowContentRendered displays markdown content with terminal rendering
func ShowContentRendered(content string) {
	if renderer == nil {
		ShowContent(content)
		return
	}
	rendered, err := renderer.Render(content)
	if err != nil {
		ShowContent(content)
		return
	}
	// glamour output already includes trailing newline, use Print to avoid double newline
	fmt.Print(strings.TrimSuffix(rendered, "\n"))
}

// ShowStreamRenderWarning displays a warning when render is used with stream mode
func ShowStreamRenderWarning() {
	fmt.Fprintln(os.Stderr, "Note: --render with --stream buffers output until complete (no real-time streaming)")
}

// ShowError displays an error message
func ShowError(message string) {
	fmt.Printf("Error: %s\n", message)
}

// ShowKeyRotation displays a message when API key is rotated
func ShowKeyRotation(fromIndex, toIndex int, totalKeys int) {
	fmt.Fprintf(os.Stderr, "Note: API key %d/%d failed, switching to key %d/%d\n", fromIndex, totalKeys, toIndex, totalKeys)
}
