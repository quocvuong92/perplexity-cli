package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/quocvuong92/perplexity-cli/internal/api"
	"github.com/quocvuong92/perplexity-cli/internal/config"
)

func createMockServer(t *testing.T, response *api.ChatResponse) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
}

func createMockStreamServer(t *testing.T, chunks []string, finalResponse *api.ChatResponse) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("Streaming not supported")
		}

		for _, chunk := range chunks {
			resp := &api.ChatResponse{
				Choices: []api.StreamChoice{
					{Delta: api.Delta{Content: chunk}},
				},
			}
			data, _ := json.Marshal(resp)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}

		if finalResponse != nil {
			data, _ := json.Marshal(finalResponse)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}

		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
}

func TestRunNormal(t *testing.T) {
	mockResponse := &api.ChatResponse{
		Choices: []api.StreamChoice{
			{
				Message: api.Message{
					Role:    "assistant",
					Content: "This is a test response",
				},
			},
		},
		Citations: []string{"https://example.com"},
		Usage: api.Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}

	server := createMockServer(t, mockResponse)
	defer server.Close()

	cfg := &config.Config{
		APIKey:    "test-key",
		Model:     "sonar-pro",
		Citations: true,
		Usage:     true,
		Render:    false,
		Stream:    false,
	}

	app := &App{cfg: cfg}
	app.client = api.NewClient(cfg)
	app.client.SetBaseURL(server.URL)

	output := captureOutput(func() {
		app.runNormal(context.Background(), "test query")
	})

	if !strings.Contains(output, "This is a test response") {
		t.Error("Output should contain response content")
	}
}

func TestRunNormalWithOutputFile(t *testing.T) {
	mockResponse := &api.ChatResponse{
		Choices: []api.StreamChoice{
			{
				Message: api.Message{
					Role:    "assistant",
					Content: "File output test",
				},
			},
		},
	}

	server := createMockServer(t, mockResponse)
	defer server.Close()

	tempFile := "test-output-normal.txt"
	defer os.Remove(tempFile)

	cfg := &config.Config{
		APIKey:     "test-key",
		Model:      "sonar-pro",
		OutputFile: tempFile,
		Render:     false,
	}

	app := &App{cfg: cfg}
	app.client = api.NewClient(cfg)
	app.client.SetBaseURL(server.URL)

	captureOutput(func() {
		app.runNormal(context.Background(), "test query")
	})

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if string(content) != "File output test" {
		t.Errorf("Expected 'File output test', got %q", string(content))
	}
}

func TestRunNormalWithRender(t *testing.T) {
	mockResponse := &api.ChatResponse{
		Choices: []api.StreamChoice{
			{
				Message: api.Message{
					Role:    "assistant",
					Content: "# Heading\n\nSome **bold** text",
				},
			},
		},
	}

	server := createMockServer(t, mockResponse)
	defer server.Close()

	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "sonar-pro",
		Render: true,
	}

	app := &App{cfg: cfg}
	app.client = api.NewClient(cfg)
	app.client.SetBaseURL(server.URL)

	output := captureOutput(func() {
		app.runNormal(context.Background(), "test query")
	})

	if output == "" {
		t.Error("Should produce rendered output")
	}
}

func TestRunStream(t *testing.T) {
	chunks := []string{"Hello ", "World ", "!"}
	finalResponse := &api.ChatResponse{
		Citations: []string{"https://example.com"},
		Usage: api.Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}

	server := createMockStreamServer(t, chunks, finalResponse)
	defer server.Close()

	cfg := &config.Config{
		APIKey:    "test-key",
		Model:     "sonar-pro",
		Stream:    true,
		Citations: true,
		Usage:     true,
		Render:    false,
	}

	app := &App{cfg: cfg}
	app.client = api.NewClient(cfg)
	app.client.SetBaseURL(server.URL)

	output := captureOutput(func() {
		app.runStream(context.Background(), "test query")
	})

	if !strings.Contains(output, "Hello") {
		t.Error("Output should contain streamed content")
	}
}

func TestRunStreamWithOutputFile(t *testing.T) {
	chunks := []string{"Streamed ", "content ", "here"}

	server := createMockStreamServer(t, chunks, nil)
	defer server.Close()

	tempFile := "test-output-stream.txt"
	defer os.Remove(tempFile)

	cfg := &config.Config{
		APIKey:     "test-key",
		Model:      "sonar-pro",
		Stream:     true,
		OutputFile: tempFile,
		Render:     false,
	}

	app := &App{cfg: cfg}
	app.client = api.NewClient(cfg)
	app.client.SetBaseURL(server.URL)

	captureOutput(func() {
		app.runStream(context.Background(), "test query")
	})

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if !strings.Contains(string(content), "Streamed") {
		t.Error("File should contain streamed content")
	}
}

func TestRunStreamWithRender(t *testing.T) {
	chunks := []string{"# Title\n", "**Bold** text"}

	server := createMockStreamServer(t, chunks, nil)
	defer server.Close()

	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "sonar-pro",
		Stream: true,
		Render: true,
	}

	app := &App{cfg: cfg}
	app.client = api.NewClient(cfg)
	app.client.SetBaseURL(server.URL)

	output := captureOutput(func() {
		app.runStream(context.Background(), "test query")
	})

	if output == "" {
		t.Error("Should produce rendered output")
	}
}

func TestRunNormalError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": {"message": "Internal server error"}}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "sonar-pro",
		Render: false,
	}

	app := &App{cfg: cfg}
	app.client = api.NewClient(cfg)
	app.client.SetBaseURL(server.URL)

	output := captureOutput(func() {
		app.runNormal(context.Background(), "test query")
	})

	if !strings.Contains(output, "Error") && !strings.Contains(output, "error") {
		t.Errorf("Should show error message, got: %s", output)
	}
}

func TestRunStreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": {"message": "Unauthorized"}}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "sonar-pro",
		Stream: true,
		Render: false,
	}

	app := &App{cfg: cfg}
	app.client = api.NewClient(cfg)
	app.client.SetBaseURL(server.URL)

	output := captureOutput(func() {
		app.runStream(context.Background(), "test query")
	})

	if !strings.Contains(output, "Error") && !strings.Contains(output, "error") {
		t.Errorf("Should show error message, got: %s", output)
	}
}

func TestRunNormalNoCitations(t *testing.T) {
	mockResponse := &api.ChatResponse{
		Choices: []api.StreamChoice{
			{
				Message: api.Message{
					Role:    "assistant",
					Content: "Response without citations",
				},
			},
		},
		Citations: []string{},
	}

	server := createMockServer(t, mockResponse)
	defer server.Close()

	cfg := &config.Config{
		APIKey:    "test-key",
		Model:     "sonar-pro",
		Citations: false,
		Render:    false,
	}

	app := &App{cfg: cfg}
	app.client = api.NewClient(cfg)
	app.client.SetBaseURL(server.URL)

	output := captureOutput(func() {
		app.runNormal(context.Background(), "test query")
	})

	if strings.Contains(output, "Sources") {
		t.Error("Should not show citations when disabled")
	}
}

func TestRunStreamNoCitations(t *testing.T) {
	chunks := []string{"Response"}
	finalResponse := &api.ChatResponse{
		Citations: []string{},
	}

	server := createMockStreamServer(t, chunks, finalResponse)
	defer server.Close()

	cfg := &config.Config{
		APIKey:    "test-key",
		Model:     "sonar-pro",
		Stream:    true,
		Citations: false,
		Render:    false,
	}

	app := &App{cfg: cfg}
	app.client = api.NewClient(cfg)
	app.client.SetBaseURL(server.URL)

	output := captureOutput(func() {
		app.runStream(context.Background(), "test query")
	})

	if strings.Contains(output, "Sources") {
		t.Error("Should not show citations when disabled")
	}
}
