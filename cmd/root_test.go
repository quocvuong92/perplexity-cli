package cmd

import (
	"testing"

	"github.com/quocvuong92/perplexity-cli/internal/config"
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
