package cmd

import (
	"sync"
	"testing"
	"time"

	"github.com/quocvuong92/perplexity-cli/internal/api"
	"github.com/quocvuong92/perplexity-cli/internal/config"
	"github.com/quocvuong92/perplexity-cli/internal/history"
)

func TestNewInterruptibleContext(t *testing.T) {
	ic := NewInterruptibleContext()
	if ic == nil {
		t.Fatal("NewInterruptibleContext should not return nil")
	}
	if ic.active {
		t.Error("New InterruptibleContext should not be active")
	}
}

func TestInterruptibleContextStart(t *testing.T) {
	ic := NewInterruptibleContext()

	ctx := ic.Start()
	if ctx == nil {
		t.Fatal("Start should return a non-nil context")
	}
	if !ic.active {
		t.Error("After Start, InterruptibleContext should be active")
	}

	// Clean up
	ic.Stop()
}

func TestInterruptibleContextStop(t *testing.T) {
	ic := NewInterruptibleContext()

	ctx := ic.Start()
	ic.Stop()

	if ic.active {
		t.Error("After Stop, InterruptibleContext should not be active")
	}

	// Context should be cancelled
	select {
	case <-ctx.Done():
		// Expected
	default:
		t.Error("Context should be cancelled after Stop")
	}
}

func TestInterruptibleContextMultipleStartStop(t *testing.T) {
	ic := NewInterruptibleContext()

	// Multiple start/stop cycles
	for i := 0; i < 3; i++ {
		ctx := ic.Start()
		if ctx == nil {
			t.Fatalf("Iteration %d: Start should return non-nil context", i)
		}
		if !ic.active {
			t.Errorf("Iteration %d: Should be active after Start", i)
		}
		ic.Stop()
		if ic.active {
			t.Errorf("Iteration %d: Should not be active after Stop", i)
		}
	}
}

func TestInterruptibleContextConcurrentAccess(t *testing.T) {
	ic := NewInterruptibleContext()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := ic.Start()
			time.Sleep(10 * time.Millisecond)
			_ = ctx
			ic.Stop()
		}()
	}

	// Should not panic or deadlock
	done := make(chan bool)
	go func() {
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for concurrent operations")
	}
}

func TestInteractiveSessionCreation(t *testing.T) {
	cfg := &config.Config{
		Model:     "sonar-pro",
		Citations: true,
	}
	session := &InteractiveSession{
		app: &App{cfg: cfg},
		messages: []api.Message{
			{Role: "system", Content: config.DefaultSystemMessage},
		},
		history:      history.NewHistory(),
		interruptCtx: NewInterruptibleContext(),
	}

	if session.app.cfg.Model != "sonar-pro" {
		t.Errorf("Expected model 'sonar-pro', got %s", session.app.cfg.Model)
	}
	if len(session.messages) != 1 {
		t.Errorf("Expected 1 initial message, got %d", len(session.messages))
	}
	if session.messages[0].Role != "system" {
		t.Error("First message should be system role")
	}
}

func TestSaveHistoryEmptySession(t *testing.T) {
	session := &InteractiveSession{
		app: &App{cfg: &config.Config{}},
		messages: []api.Message{
			{Role: "system", Content: "test"},
		},
		history: nil,
	}

	// Should not panic with nil history
	session.saveHistory()
}

func TestSaveHistoryWithMessages(t *testing.T) {
	hist := history.NewHistory()
	session := &InteractiveSession{
		app: &App{cfg: &config.Config{Model: "sonar"}},
		messages: []api.Message{
			{Role: "system", Content: "test"},
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "hi there"},
		},
		history:        hist,
		conversationID: "test-id-123",
	}

	session.saveHistory()

	// Verify conversation was added
	conv := hist.GetConversation("test-id-123")
	if conv == nil {
		t.Fatal("Conversation should be saved")
	}
	if len(conv.Messages) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(conv.Messages))
	}
}

func TestExecutorEmptyInput(t *testing.T) {
	session := newTestSession()

	// Empty input should do nothing
	initialMsgCount := len(session.messages)
	session.executor("")
	session.executor("   ")

	if len(session.messages) != initialMsgCount {
		t.Error("Empty input should not change messages")
	}
}

func TestExecutorCommand(t *testing.T) {
	session := newTestSession()

	output := captureOutput(func() {
		session.executor("/help")
	})

	if output == "" {
		t.Error("Command should produce output")
	}
}

func TestExecutorMultilineInput(t *testing.T) {
	session := newTestSession()

	// First line with continuation
	session.executor("line 1\\")
	if len(session.inputBuffer) != 1 {
		t.Errorf("Expected 1 line in buffer, got %d", len(session.inputBuffer))
	}

	// Second line with continuation
	session.executor("line 2\\")
	if len(session.inputBuffer) != 2 {
		t.Errorf("Expected 2 lines in buffer, got %d", len(session.inputBuffer))
	}
}

func TestExecutorExitFlag(t *testing.T) {
	session := newTestSession()
	session.exitFlag = true

	// Should return early when exit flag is set
	initialMsgCount := len(session.messages)
	session.executor("test input")

	if len(session.messages) != initialMsgCount {
		t.Error("Executor should return early when exitFlag is true")
	}
}

func TestExecutorExitCommand(t *testing.T) {
	session := newTestSession()

	captureOutput(func() {
		session.executor("/exit")
	})

	if !session.exitFlag {
		t.Error("Exit command should set exitFlag to true")
	}
}
