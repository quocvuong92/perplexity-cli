package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/elk-language/go-prompt"
	"github.com/google/uuid"

	"github.com/quocvuong92/perplexity-cli/internal/api"
	"github.com/quocvuong92/perplexity-cli/internal/config"
	"github.com/quocvuong92/perplexity-cli/internal/display"
	"github.com/quocvuong92/perplexity-cli/internal/history"
	"github.com/quocvuong92/perplexity-cli/internal/retry"
	"github.com/quocvuong92/perplexity-cli/internal/validation"
)

// ANSI color codes for banner
const (
	colorReset  = "\033[0m"
	colorCyan   = "\033[36m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
)

// showBanner displays the ASCII art banner for interactive mode
func showBanner(model string) {
	fmt.Println()
	fmt.Printf("        %s%s██████╗ ███████╗██████╗ ██████╗ ██╗     ███████╗██╗  ██╗██╗████████╗██╗   ██╗%s\n", colorBold, colorCyan, colorReset)
	fmt.Printf("        %s%s██╔══██╗██╔════╝██╔══██╗██╔══██╗██║     ██╔════╝╚██╗██╔╝██║╚══██╔══╝╚██╗ ██╔╝%s\n", colorBold, colorCyan, colorReset)
	fmt.Printf("        %s%s██████╔╝█████╗  ██████╔╝██████╔╝██║     █████╗   ╚███╔╝ ██║   ██║    ╚████╔╝%s\n", colorBold, colorBlue, colorReset)
	fmt.Printf("        %s%s██╔═══╝ ██╔══╝  ██╔══██╗██╔═══╝ ██║     ██╔══╝   ██╔██╗ ██║   ██║     ╚██╔╝%s\n", colorBold, colorBlue, colorReset)
	fmt.Printf("        %s%s██║     ███████╗██║  ██║██║     ███████╗███████╗██╔╝ ██╗██║   ██║      ██║%s\n", colorBold, colorPurple, colorReset)
	fmt.Printf("        %s%s╚═╝     ╚══════╝╚═╝  ╚═╝╚═╝     ╚══════╝╚══════╝╚═╝  ╚═╝╚═╝   ╚═╝      ╚═╝%s\n", colorBold, colorPurple, colorReset)
	fmt.Println()

	// Tip box like Kiro (aligned with banner width - 78 chars)
	fmt.Printf("        %s╭────────────────────────────────── Tips ───────────────────────────────────╮%s\n", colorDim, colorReset)
	fmt.Printf("        %s│                                                                           │%s\n", colorDim, colorReset)
	fmt.Printf("        %s│        Type /help for commands, use Ctrl+D to quit the session            │%s\n", colorDim, colorReset)
	fmt.Printf("        %s│                End a line with \\ for multiline input                      │%s\n", colorDim, colorReset)
	fmt.Printf("        %s│                                                                           │%s\n", colorDim, colorReset)
	fmt.Printf("        %s╰───────────────────────────────────────────────────────────────────────────╯%s\n", colorDim, colorReset)
	fmt.Println()

	// Model info at bottom
	fmt.Printf("%sModel: %s%s%s\n", colorDim, colorReset, model, colorReset)
	fmt.Println()
}

// InterruptibleContext manages a cancellable context for operations.
// It allows Ctrl+C to cancel the current operation instead of exiting the CLI.
type InterruptibleContext struct {
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
	active bool
}

// NewInterruptibleContext creates a new interruptible context manager.
func NewInterruptibleContext() *InterruptibleContext {
	return &InterruptibleContext{}
}

// Start begins an interruptible operation, returning a context that will be
// cancelled if Ctrl+C is pressed during the operation.
func (ic *InterruptibleContext) Start() context.Context {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	ic.ctx, ic.cancel = context.WithCancel(context.Background())
	ic.active = true

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGINT)

	go func() {
		select {
		case <-sigChan:
			ic.mu.Lock()
			if ic.active {
				fmt.Fprintf(os.Stderr, "\nOperation cancelled\n")
				ic.cancel()
			}
			ic.mu.Unlock()
		case <-ic.ctx.Done():
		}
		signal.Stop(sigChan)
		close(sigChan)
	}()

	return ic.ctx
}

// Stop ends the interruptible operation and cleans up.
func (ic *InterruptibleContext) Stop() {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	ic.active = false
	if ic.cancel != nil {
		ic.cancel()
	}
}

// InteractiveSession holds the state for interactive mode
type InteractiveSession struct {
	app            *App
	client         *api.Client
	messages       []api.Message
	messagesMu     sync.RWMutex // Protects messages slice
	exitFlag       bool
	inputBuffer    []string
	history        *history.History
	conversationID string
	interruptCtx   *InterruptibleContext
	lastUserInput  string
	lastResponse   string
}

// runInteractive starts the interactive chat mode
func (app *App) runInteractive() {
	showBanner(app.cfg.Model)

	hist := history.NewHistory()
	if err := hist.Load(); err != nil {
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
		interruptCtx:   NewInterruptibleContext(),
	}

	session.client.SetKeyRotationCallback(func(fromIndex, toIndex int, totalKeys int) {
		display.ShowKeyRotation(fromIndex, toIndex, totalKeys)
	})

	session.client.SetRetryCallback(func(info retry.RetryInfo) {
		display.ShowRetry(info.Attempt+1, info.MaxRetries, info.NextBackoff)
	})

	p := prompt.New(
		session.executor,
		prompt.WithCompleter(session.completer),
		prompt.WithPrefix("> "),
		prompt.WithTitle("Perplexity CLI"),
		prompt.WithPrefixTextColor(prompt.Green),
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
func (s *InteractiveSession) saveHistory() {
	if s.history == nil {
		return
	}

	s.messagesMu.RLock()
	msgCount := len(s.messages)
	if msgCount > 1 {
		historyMessages := make([]history.Message, msgCount)
		for i, msg := range s.messages {
			historyMessages[i] = history.Message{
				Role:    msg.Role,
				Content: msg.Content,
			}
		}
		s.messagesMu.RUnlock()

		if !s.history.UpdateConversation(s.conversationID, historyMessages) {
			s.history.AddConversation(
				s.conversationID,
				s.app.cfg.Model,
				historyMessages,
			)
		}
		if err := s.history.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not save history: %v\n", err)
		}
	} else {
		s.messagesMu.RUnlock()
	}
}

// appendMessage safely appends a message to the messages slice
func (s *InteractiveSession) appendMessage(msg api.Message) {
	s.messagesMu.Lock()
	s.messages = append(s.messages, msg)
	s.messagesMu.Unlock()
}

// removeLastMessage safely removes the last message from the messages slice
func (s *InteractiveSession) removeLastMessage() {
	s.messagesMu.Lock()
	if len(s.messages) > 0 {
		s.messages = s.messages[:len(s.messages)-1]
	}
	s.messagesMu.Unlock()
}

// getMessages returns a copy of the messages slice for safe iteration
func (s *InteractiveSession) getMessages() []api.Message {
	s.messagesMu.RLock()
	defer s.messagesMu.RUnlock()
	msgs := make([]api.Message, len(s.messages))
	copy(msgs, s.messages)
	return msgs
}

// getMessageCount returns the current message count
func (s *InteractiveSession) getMessageCount() int {
	s.messagesMu.RLock()
	defer s.messagesMu.RUnlock()
	return len(s.messages)
}

// setMessages safely replaces the entire messages slice
func (s *InteractiveSession) setMessages(msgs []api.Message) {
	s.messagesMu.Lock()
	s.messages = msgs
	s.messagesMu.Unlock()
}

// executor handles the execution of each input line
func (s *InteractiveSession) executor(input string) {
	if s.exitFlag {
		return
	}

	// Handle multiline input
	if strings.HasSuffix(input, "\\") {
		line := strings.TrimSuffix(input, "\\")
		s.inputBuffer = append(s.inputBuffer, line)
		fmt.Print("... ")
		return
	}

	if len(s.inputBuffer) > 0 {
		s.inputBuffer = append(s.inputBuffer, input)
		input = strings.Join(s.inputBuffer, "\n")
		s.inputBuffer = nil
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return
	}

	// Handle commands
	if strings.HasPrefix(input, "/") {
		if s.handleCommand(input) {
			s.exitFlag = true
		}
		return
	}

	// Validate and sanitize the input
	input = validation.SanitizePrompt(input)
	result := validation.ValidatePrompt(input)
	if !result.Valid {
		display.ShowError(result.Error.Error())
		return
	}
	input = result.Cleaned

	// Regular chat
	s.lastUserInput = input
	s.appendMessage(api.Message{Role: "user", Content: input})
	fmt.Println()

	response, citations, err := s.sendInteractiveMessage()
	if err != nil {
		if err == context.Canceled {
			s.removeLastMessage()
			return
		}
		msg, hint := display.FormatNetworkError(err)
		display.ShowFriendlyError(msg, hint)

		// On network error, we keep the user message but add a placeholder response
		// so that roles continue to alternate for future requests/retries.
		s.lastResponse = config.FailedResponsePlaceholder
		s.appendMessage(api.Message{Role: "assistant", Content: s.lastResponse})
		return
	}

	if response == "" {
		response = config.FailedResponsePlaceholder
	}
	s.lastResponse = response
	s.appendMessage(api.Message{Role: "assistant", Content: response})

	if s.app.cfg.Citations && len(citations) > 0 {
		fmt.Println()
		display.ShowCitations(citations)
	}
	fmt.Println()
}

// sendInteractiveMessage sends a message and returns the response
func (s *InteractiveSession) sendInteractiveMessage() (string, []string, error) {
	ctx := s.interruptCtx.Start()
	defer s.interruptCtx.Stop()

	// Get a copy of messages for thread-safe access
	messages := s.getMessages()

	if s.app.cfg.Stream {
		var fullContent strings.Builder
		var citations []string
		firstChunk := true

		sp := display.NewSpinner("Thinking...")
		sp.Start()

		err := s.client.QueryStreamWithHistoryContext(ctx, messages,
			func(content string) {
				if firstChunk {
					firstChunk = false
					sp.Stop()
				}
				fullContent.WriteString(content)
				fmt.Print(content)
			},
			func(resp *api.ChatResponse) {
				if resp != nil {
					citations = resp.Citations
				}
			},
		)

		if err != nil {
			return "", nil, err
		}

		if s.app.cfg.Render {
			fmt.Println("\n---")
			display.ShowContentRendered(fullContent.String())
			return fullContent.String(), citations, nil
		}
		fmt.Println()
		return fullContent.String(), citations, nil
	}

	// Non-streaming
	sp := display.NewSpinner("Thinking...")
	sp.Start()

	resp, err := s.client.QueryWithHistoryContext(ctx, messages)
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
