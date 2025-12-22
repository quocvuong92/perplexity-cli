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
)

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
				fmt.Fprintf(os.Stderr, "\n⚠️  Operation cancelled\n")
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
	fmt.Println("Perplexity CLI - Interactive Mode")
	fmt.Printf("Model: %s\n", app.cfg.Model)
	fmt.Println("Type /help for commands, Ctrl+C or Ctrl+D to quit")
	fmt.Println("Commands auto-complete as you type")
	fmt.Println("End a line with \\ for multiline input")
	fmt.Println()

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
	if len(s.messages) > 1 {
		historyMessages := make([]history.Message, len(s.messages))
		for i, msg := range s.messages {
			historyMessages[i] = history.Message{
				Role:    msg.Role,
				Content: msg.Content,
			}
		}
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
	}
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

	// Regular chat
	s.lastUserInput = input
	s.messages = append(s.messages, api.Message{Role: "user", Content: input})
	fmt.Println()

	response, citations, err := s.sendInteractiveMessage()
	if err != nil {
		if err == context.Canceled {
			s.messages = s.messages[:len(s.messages)-1]
			return
		}
		display.ShowError(err.Error())
		s.messages = s.messages[:len(s.messages)-1]
		return
	}

	if response == "" {
		response = config.FailedResponsePlaceholder
	}
	s.lastResponse = response
	s.messages = append(s.messages, api.Message{Role: "assistant", Content: response})

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

	if s.app.cfg.Stream {
		var fullContent strings.Builder
		var citations []string
		firstChunk := true

		sp := display.NewSpinner("Thinking...")
		sp.Start()

		err := s.client.QueryStreamWithHistoryContext(ctx, s.messages,
			func(content string) {
				if firstChunk {
					firstChunk = false
					if s.app.cfg.Render {
						sp.UpdateMessage("Receiving...")
					} else {
						sp.Stop()
					}
				}
				fullContent.WriteString(content)
				if !s.app.cfg.Render {
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

		if s.app.cfg.Render {
			display.ShowContentRendered(fullContent.String())
			return fullContent.String(), citations, nil
		}
		fmt.Println()
		return fullContent.String(), citations, nil
	}

	// Non-streaming
	sp := display.NewSpinner("Thinking...")
	sp.Start()

	resp, err := s.client.QueryWithHistoryContext(ctx, s.messages)
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
