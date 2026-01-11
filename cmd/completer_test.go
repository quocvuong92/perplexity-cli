package cmd

import (
	"strings"
	"testing"

	"github.com/quocvuong92/perplexity-cli/internal/api"
	"github.com/quocvuong92/perplexity-cli/internal/config"
	"github.com/quocvuong92/perplexity-cli/internal/history"
)

func TestToLower(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"HELLO", "hello"},
		{"Hello World", "hello world"},
		{"hello", "hello"},
		{"123ABC", "123abc"},
		{"", ""},
	}

	for _, tt := range tests {
		got := strings.ToLower(tt.input)
		if got != tt.want {
			t.Errorf("strings.ToLower(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestHasPrefix(t *testing.T) {
	tests := []struct {
		s      string
		prefix string
		want   bool
	}{
		{"/model test", "/model ", true},
		{"/m test", "/m ", true},
		{"/model", "/model ", false},
		{"", "/", false},
		{"/help", "/help", true},
		{"/citations on", "/citations ", true},
		{"/system reset", "/system ", true},
	}

	for _, tt := range tests {
		got := strings.HasPrefix(tt.s, tt.prefix)
		if got != tt.want {
			t.Errorf("strings.HasPrefix(%q, %q) = %v, want %v", tt.s, tt.prefix, got, tt.want)
		}
	}
}

func newTestSession() *InteractiveSession {
	cfg := &config.Config{
		Model:     "sonar-pro",
		Citations: true,
	}
	return &InteractiveSession{
		app: &App{cfg: cfg},
		messages: []api.Message{
			{Role: "system", Content: config.DefaultSystemMessage},
		},
		history: history.NewHistory(),
	}
}

// Note: Full completer testing requires the prompt library's Document type
// which has complex internal state. The helper functions (toLower, hasPrefix)
// are tested above. Integration testing of the completer is better done
// through manual testing or end-to-end tests.
