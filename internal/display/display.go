package display

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/briandowns/spinner"
	"github.com/charmbracelet/glamour"
)

// renderer is the markdown renderer instance
var (
	renderer     *glamour.TermRenderer
	rendererOnce sync.Once
	rendererErr  error
)

// Spinner wraps the spinner with elapsed time display
type Spinner struct {
	s         *spinner.Spinner
	startTime time.Time
	message   string
	stopChan  chan struct{}
	wg        sync.WaitGroup
	stopped   bool
	mu        sync.Mutex
}

// NewSpinner creates a new spinner with the given message
func NewSpinner(message string) *Spinner {
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = fmt.Sprintf(" %s (0.0s)", message)
	s.Writer = os.Stderr
	return &Spinner{
		s:        s,
		message:  message,
		stopChan: make(chan struct{}),
	}
}

// Start begins the spinner animation
func (sp *Spinner) Start() {
	sp.mu.Lock()
	sp.startTime = time.Now()
	sp.mu.Unlock()

	sp.s.Start()

	// Update elapsed time in background
	sp.wg.Add(1)
	go func() {
		defer sp.wg.Done()
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-sp.stopChan:
				return
			case <-ticker.C:
				sp.mu.Lock()
				if sp.stopped {
					sp.mu.Unlock()
					return
				}
				elapsed := time.Since(sp.startTime).Seconds()
				message := sp.message
				sp.mu.Unlock()
				sp.s.Suffix = fmt.Sprintf(" %s (%.1fs)", message, elapsed)
			}
		}
	}()
}

// Stop stops the spinner and clears the line
func (sp *Spinner) Stop() {
	sp.mu.Lock()
	if sp.stopped {
		sp.mu.Unlock()
		return
	}
	sp.stopped = true
	sp.mu.Unlock()

	close(sp.stopChan)
	sp.wg.Wait()
	sp.s.Stop()
}

// UpdateMessage updates the spinner message while keeping it running
func (sp *Spinner) UpdateMessage(message string) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	if sp.stopped {
		return
	}
	sp.message = message
	elapsed := time.Since(sp.startTime).Seconds()
	sp.s.Suffix = fmt.Sprintf(" %s (%.1fs)", message, elapsed)
}

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

// ShowError displays an error message
func ShowError(message string) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", message)
}

// ShowFriendlyError displays an error with a user-friendly message and optional hint
func ShowFriendlyError(message, hint string) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", message)
	if hint != "" {
		fmt.Fprintf(os.Stderr, "Hint: %s\n", hint)
	}
}

// FormatNetworkError returns a user-friendly message for common network errors
func FormatNetworkError(err error) (message string, hint string) {
	if err == nil {
		return "", ""
	}

	errStr := err.Error()

	// Check for common network error patterns
	switch {
	case strings.Contains(errStr, "connection refused"):
		return "Could not connect to the API server",
			"Check your internet connection and firewall settings"

	case strings.Contains(errStr, "no such host") || strings.Contains(errStr, "dns"):
		return "Could not resolve API server address",
			"Check your internet connection and DNS settings"

	case strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded"):
		return "Request timed out",
			"The server took too long to respond. Try again or check your connection"

	case strings.Contains(errStr, "certificate") || strings.Contains(errStr, "x509"):
		return "SSL/TLS certificate error",
			"There may be a problem with your network security settings or a proxy"

	case strings.Contains(errStr, "network is unreachable"):
		return "Network is unreachable",
			"Check your internet connection"

	case strings.Contains(errStr, "connection reset"):
		return "Connection was reset",
			"The server closed the connection unexpectedly. Try again"

	case strings.Contains(errStr, "EOF") || strings.Contains(errStr, "unexpected EOF"):
		return "Connection closed unexpectedly",
			"The server closed the connection. Try again"

	default:
		return errStr, ""
	}
}

// ShowKeyRotation displays a message when API key is rotated
func ShowKeyRotation(fromIndex, toIndex int, totalKeys int) {
	fmt.Fprintf(os.Stderr, "Note: API key %d/%d failed, switching to key %d/%d\n", fromIndex, totalKeys, toIndex, totalKeys)
}

// ShowRetry displays a message when retrying due to network error
func ShowRetry(attempt, maxRetries int, nextBackoff time.Duration) {
	fmt.Fprintf(os.Stderr, "Note: Network error, retrying (%d/%d) in %v...\n", attempt, maxRetries, nextBackoff.Round(time.Millisecond))
}

// ShowModels displays available models
func ShowModels(models []string, currentModel string) {
	fmt.Println("Available models:")
	for _, m := range models {
		if m == currentModel {
			fmt.Printf("  * %s (current)\n", m)
		} else {
			fmt.Printf("    %s\n", m)
		}
	}
}
