package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/quocvuong92/perplexity-cli/internal/config"
	"github.com/quocvuong92/perplexity-cli/internal/logging"
)

func TestNewApp(t *testing.T) {
	app := NewApp()

	if app == nil {
		t.Fatal("NewApp should not return nil")
	}
	if app.cfg == nil {
		t.Fatal("NewApp should create config")
	}
	if app.verbose {
		t.Error("NewApp should have verbose=false by default")
	}
	if app.listModels {
		t.Error("NewApp should have listModels=false by default")
	}
}

func TestAppConfigDefaults(t *testing.T) {
	app := NewApp()

	if app.cfg.Model != config.DefaultModel {
		t.Errorf("Default model should be %s, got %s", config.DefaultModel, app.cfg.Model)
	}
}

func TestAppStruct(t *testing.T) {
	cfg := &config.Config{
		Model:     "sonar",
		Citations: true,
		Usage:     false,
	}

	app := &App{
		cfg:        cfg,
		verbose:    true,
		listModels: false,
	}

	if app.cfg.Model != "sonar" {
		t.Errorf("Expected model 'sonar', got %s", app.cfg.Model)
	}
	if !app.cfg.Citations {
		t.Error("Citations should be true")
	}
	if !app.verbose {
		t.Error("Verbose should be true")
	}
}

func TestShouldUseColor(t *testing.T) {
	tests := []struct {
		name     string
		noColor  bool
		envSet   bool
		expected bool
	}{
		{"flag set", true, false, false},
		{"env set", false, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{noColor: tt.noColor}

			if tt.envSet {
				os.Setenv("NO_COLOR", "1")
				defer os.Unsetenv("NO_COLOR")
			}

			result := app.shouldUseColor()
			if tt.noColor && result {
				t.Error("--no-color should disable colors")
			}
			if tt.envSet && result {
				t.Error("NO_COLOR env should disable colors")
			}
		})
	}
}

func TestListModelsFlag(t *testing.T) {
	// Reset logger for clean test
	logging.ResetForTesting()

	app := NewApp()
	app.listModels = true

	// Capture stdout
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	os.Stdout = w

	cmd := &cobra.Command{}
	app.run(cmd, []string{})

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)

	output := buf.String()
	if !strings.Contains(output, "sonar-pro") {
		t.Error("--list-models should show available models")
	}
}

func TestVerboseFlag(t *testing.T) {
	// Reset logger for clean test
	logging.ResetForTesting()

	app := NewApp()
	app.verbose = true
	app.listModels = true // Use list-models to avoid needing API key

	cmd := &cobra.Command{}

	// Capture stdout (list-models outputs there)
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	os.Stdout = w

	app.run(cmd, []string{})

	w.Close()
	os.Stdout = old
	io.Copy(io.Discard, r)

	// If we get here without panic, verbose initialization worked
}
