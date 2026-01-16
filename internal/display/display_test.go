package display

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func captureStdout(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func captureStderr(f func()) string {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	f()

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestShowContent(t *testing.T) {
	output := captureStdout(func() {
		ShowContent("  Hello World  ")
	})

	// Should trim whitespace
	if output != "Hello World\n" {
		t.Errorf("ShowContent() output = %q, want %q", output, "Hello World\n")
	}
}

func TestShowCitations(t *testing.T) {
	citations := []string{
		"https://example.com/1",
		"https://example.com/2",
	}

	output := captureStdout(func() {
		ShowCitations(citations)
	})

	if !strings.Contains(output, "## Citations") {
		t.Error("ShowCitations() should contain header")
	}
	if !strings.Contains(output, "1. https://example.com/1") {
		t.Error("ShowCitations() should contain first citation")
	}
	if !strings.Contains(output, "2. https://example.com/2") {
		t.Error("ShowCitations() should contain second citation")
	}
}

func TestShowUsage(t *testing.T) {
	usage := map[string]int{
		"prompt_tokens":     100,
		"completion_tokens": 50,
		"total_tokens":      150,
	}

	output := captureStdout(func() {
		ShowUsage(usage)
	})

	if !strings.Contains(output, "## Tokens") {
		t.Error("ShowUsage() should contain header")
	}
	if !strings.Contains(output, "| Prompt | 100 |") {
		t.Error("ShowUsage() should contain prompt tokens")
	}
	if !strings.Contains(output, "| Completion | 50 |") {
		t.Error("ShowUsage() should contain completion tokens")
	}
	if !strings.Contains(output, "| **Total** | **150** |") {
		t.Error("ShowUsage() should contain total tokens")
	}
}

func TestShowError(t *testing.T) {
	output := captureStderr(func() {
		ShowError("Something went wrong")
	})

	if output != "Error: Something went wrong\n" {
		t.Errorf("ShowError() output = %q, want %q", output, "Error: Something went wrong\n")
	}
}

func TestShowKeyRotation(t *testing.T) {
	output := captureStderr(func() {
		ShowKeyRotation(1, 2, 3)
	})

	if !strings.Contains(output, "key 1/3 failed") {
		t.Error("ShowKeyRotation() should mention failed key")
	}
	if !strings.Contains(output, "switching to key 2/3") {
		t.Error("ShowKeyRotation() should mention new key")
	}
}

func TestShowModels(t *testing.T) {
	models := []string{"model-a", "model-b", "model-c"}
	currentModel := "model-b"

	output := captureStdout(func() {
		ShowModels(models, currentModel)
	})

	if !strings.Contains(output, "Available models") {
		t.Error("ShowModels() should contain header")
	}
	if !strings.Contains(output, "model-a") {
		t.Error("ShowModels() should list model-a")
	}
	if !strings.Contains(output, "* model-b (current)") {
		t.Error("ShowModels() should mark current model")
	}
	if !strings.Contains(output, "model-c") {
		t.Error("ShowModels() should list model-c")
	}
}

func TestInitRenderer(t *testing.T) {
	// Should not error
	err := InitRenderer()
	if err != nil {
		t.Errorf("InitRenderer() error = %v", err)
	}

	// Second call should also succeed (using sync.Once)
	err = InitRenderer()
	if err != nil {
		t.Errorf("InitRenderer() second call error = %v", err)
	}
}

func TestShowContentRendered(t *testing.T) {
	// Initialize renderer first
	InitRenderer()

	output := captureStdout(func() {
		ShowContentRendered("**Bold text**")
	})

	// Should produce some output (rendered or fallback)
	if output == "" {
		t.Error("ShowContentRendered() should produce output")
	}
}

func TestShowContentRenderedFallback(t *testing.T) {
	// Test fallback when renderer is nil
	oldRenderer := renderer
	renderer = nil
	defer func() { renderer = oldRenderer }()

	output := captureStdout(func() {
		ShowContentRendered("Plain text")
	})

	if !strings.Contains(output, "Plain text") {
		t.Error("ShowContentRendered() fallback should show plain text")
	}
}

func TestNewSpinner(t *testing.T) {
	sp := NewSpinner("Loading...")

	if sp == nil {
		t.Fatal("NewSpinner() returned nil")
	}
	if sp.message != "Loading..." {
		t.Errorf("Spinner message = %q, want %q", sp.message, "Loading...")
	}
	if sp.s == nil {
		t.Error("Spinner internal spinner is nil")
	}
}

func TestSpinnerStartStop(t *testing.T) {
	sp := NewSpinner("Test")

	// Should not panic
	sp.Start()

	// Give it a moment to run
	// time.Sleep(50 * time.Millisecond)

	sp.Stop()

	// Double stop should not panic
	sp.Stop()
}

func TestSpinnerUpdateMessage(t *testing.T) {
	sp := NewSpinner("Initial")
	sp.Start()

	sp.UpdateMessage("Updated")

	if sp.message != "Updated" {
		t.Errorf("After UpdateMessage, message = %q, want %q", sp.message, "Updated")
	}

	sp.Stop()

	// UpdateMessage after stop should not panic
	sp.UpdateMessage("After stop")
}

func TestSpinnerImmediateStop(t *testing.T) {
	// Test that Start followed immediately by Stop doesn't panic or race
	for i := 0; i < 100; i++ {
		sp := NewSpinner("Test")
		sp.Start()
		sp.Stop() // Immediate stop should not race
	}
}

func TestSpinnerRaceCondition(t *testing.T) {
	// Run with -race flag to detect race conditions
	sp := NewSpinner("Test")

	done := make(chan struct{})
	go func() {
		sp.Start()
		close(done)
	}()

	// Try to stop while start might still be setting up
	<-done
	sp.Stop()
}
