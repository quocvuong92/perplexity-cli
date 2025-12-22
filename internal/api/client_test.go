package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/quocvuong92/perplexity-cli/internal/config"
)

func TestNewClient(t *testing.T) {
	cfg := &config.Config{
		APIURL:  "https://api.example.com",
		APIKey:  "test-key",
		Timeout: 30 * time.Second,
	}

	client := NewClient(cfg)
	if client == nil {
		t.Fatal("NewClient() returned nil")
	}
	if client.config != cfg {
		t.Error("Client config not set correctly")
	}
	if client.httpClient == nil {
		t.Error("HTTP client not initialized")
	}
}

func TestChatResponseGetContent(t *testing.T) {
	tests := []struct {
		name     string
		response ChatResponse
		want     string
	}{
		{
			name: "content from message",
			response: ChatResponse{
				Choices: []StreamChoice{
					{Message: Message{Content: "Hello from message"}},
				},
			},
			want: "Hello from message",
		},
		{
			name: "content from delta",
			response: ChatResponse{
				Choices: []StreamChoice{
					{Delta: Delta{Content: "Hello from delta"}},
				},
			},
			want: "Hello from delta",
		},
		{
			name: "message takes precedence",
			response: ChatResponse{
				Choices: []StreamChoice{
					{
						Message: Message{Content: "From message"},
						Delta:   Delta{Content: "From delta"},
					},
				},
			},
			want: "From message",
		},
		{
			name:     "empty choices",
			response: ChatResponse{Choices: []StreamChoice{}},
			want:     "",
		},
		{
			name:     "nil choices",
			response: ChatResponse{},
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.response.GetContent()
			if got != tt.want {
				t.Errorf("GetContent() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestChatResponseGetUsageMap(t *testing.T) {
	resp := ChatResponse{
		Usage: Usage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
	}

	usage := resp.GetUsageMap()

	if usage["prompt_tokens"] != 100 {
		t.Errorf("prompt_tokens = %d, want 100", usage["prompt_tokens"])
	}
	if usage["completion_tokens"] != 50 {
		t.Errorf("completion_tokens = %d, want 50", usage["completion_tokens"])
	}
	if usage["total_tokens"] != 150 {
		t.Errorf("total_tokens = %d, want 150", usage["total_tokens"])
	}
}

func TestAPIError(t *testing.T) {
	err := &APIError{
		StatusCode: 401,
		Message:    "Unauthorized",
	}

	if err.Error() != "Unauthorized" {
		t.Errorf("Error() = %q, want %q", err.Error(), "Unauthorized")
	}
}

func TestClientShouldRotateKey(t *testing.T) {
	client := &Client{config: &config.Config{}}

	tests := []struct {
		statusCode int
		errorMsg   string
		want       bool
	}{
		{401, "", true},
		{403, "", true},
		{429, "", true},
		{402, "", false},
		{500, "", false},
		{200, "insufficient credit", true},
		{200, "quota exceeded", true},
		{200, "rate limit exceeded", true},
		{200, "normal error", false},
		{400, "INSUFFICIENT CREDIT", true}, // Case insensitive
	}

	for _, tt := range tests {
		t.Run(tt.errorMsg, func(t *testing.T) {
			got := client.shouldRotateKey(tt.statusCode, tt.errorMsg)
			if got != tt.want {
				t.Errorf("shouldRotateKey(%d, %q) = %v, want %v", tt.statusCode, tt.errorMsg, got, tt.want)
			}
		})
	}
}

func TestQueryNonStreaming(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Invalid Authorization header")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Invalid Content-Type header")
		}

		// Parse request body
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}
		if req.Model != "sonar-pro" {
			t.Errorf("Model = %q, want %q", req.Model, "sonar-pro")
		}
		if req.Stream {
			t.Error("Stream should be false for non-streaming")
		}

		// Send response
		resp := ChatResponse{
			Choices: []StreamChoice{
				{Message: Message{Role: "assistant", Content: "Hello, world!"}},
			},
			Citations: []string{"https://example.com"},
			Usage:     Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{
		APIURL:  server.URL,
		APIKey:  "test-key",
		APIKeys: []string{"test-key"},
		Model:   "sonar-pro",
		Timeout: 10 * time.Second,
	}

	client := NewClient(cfg)
	resp, err := client.Query("Test query")

	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if resp.GetContent() != "Hello, world!" {
		t.Errorf("Content = %q, want %q", resp.GetContent(), "Hello, world!")
	}
	if len(resp.Citations) != 1 {
		t.Errorf("Citations count = %d, want 1", len(resp.Citations))
	}
}

func TestQueryWithContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(100 * time.Millisecond)

		resp := ChatResponse{
			Choices: []StreamChoice{
				{Message: Message{Content: "Response"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{
		APIURL:  server.URL,
		APIKey:  "test-key",
		APIKeys: []string{"test-key"},
		Model:   "sonar-pro",
		Timeout: 10 * time.Second,
	}

	client := NewClient(cfg)

	// Test with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.QueryContext(ctx, "Test")
	if err == nil {
		t.Error("Expected error for cancelled context")
	}
}

func TestQueryAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error: struct {
				Message string `json:"message"`
			}{Message: "Invalid API key"},
		})
	}))
	defer server.Close()

	cfg := &config.Config{
		APIURL:  server.URL,
		APIKey:  "bad-key",
		APIKeys: []string{"bad-key"},
		Model:   "sonar-pro",
		Timeout: 10 * time.Second,
	}

	client := NewClient(cfg)
	_, err := client.Query("Test")

	if err == nil {
		t.Fatal("Expected error for 401 response")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("Expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 401 {
		t.Errorf("StatusCode = %d, want 401", apiErr.StatusCode)
	}
}

func TestQueryStreamBasic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		// Send streaming chunks
		chunks := []string{
			`{"choices":[{"delta":{"content":"Hello"}}]}`,
			`{"choices":[{"delta":{"content":" "}}]}`,
			`{"choices":[{"delta":{"content":"world"}}]}`,
			`{"citations":["https://example.com"],"usage":{"total_tokens":10}}`,
		}

		for _, chunk := range chunks {
			w.Write([]byte("data: " + chunk + "\n\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	cfg := &config.Config{
		APIURL:  server.URL,
		APIKey:  "test-key",
		APIKeys: []string{"test-key"},
		Model:   "sonar-pro",
		Timeout: 10 * time.Second,
	}

	client := NewClient(cfg)

	var content strings.Builder
	var finalResp *ChatResponse

	err := client.QueryStream("Test",
		func(c string) {
			content.WriteString(c)
		},
		func(resp *ChatResponse) {
			finalResp = resp
		},
	)

	if err != nil {
		t.Fatalf("QueryStream() error = %v", err)
	}

	if content.String() != "Hello world" {
		t.Errorf("Content = %q, want %q", content.String(), "Hello world")
	}

	if finalResp == nil {
		t.Error("Final response not received")
	} else if len(finalResp.Citations) != 1 {
		t.Errorf("Citations count = %d, want 1", len(finalResp.Citations))
	}
}

func TestQueryWithHistory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		json.NewDecoder(r.Body).Decode(&req)

		// Verify message history
		if len(req.Messages) != 3 {
			t.Errorf("Messages count = %d, want 3", len(req.Messages))
		}
		if req.Messages[0].Role != "system" {
			t.Errorf("First message role = %q, want 'system'", req.Messages[0].Role)
		}

		resp := ChatResponse{
			Choices: []StreamChoice{
				{Message: Message{Content: "Response"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{
		APIURL:  server.URL,
		APIKey:  "test-key",
		APIKeys: []string{"test-key"},
		Model:   "sonar-pro",
		Timeout: 10 * time.Second,
	}

	client := NewClient(cfg)
	messages := []Message{
		{Role: "system", Content: "Be helpful"},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi!"},
	}

	resp, err := client.QueryWithHistory(messages)
	if err != nil {
		t.Fatalf("QueryWithHistory() error = %v", err)
	}
	if resp.GetContent() != "Response" {
		t.Errorf("Content = %q, want %q", resp.GetContent(), "Response")
	}
}

func TestKeyRotationCallback(t *testing.T) {
	callCount := 0
	var fromIdx, toIdx, total int

	cfg := &config.Config{
		APIKey:          "key1",
		APIKeys:         []string{"key1", "key2"},
		CurrentKeyIndex: 0,
	}
	cfg.ResetKeyRotation()

	client := NewClient(cfg)
	client.SetKeyRotationCallback(func(from, to, totalKeys int) {
		callCount++
		fromIdx = from
		toIdx = to
		total = totalKeys
	})

	// Trigger rotation
	client.rotateKey()

	if callCount != 1 {
		t.Errorf("Callback called %d times, want 1", callCount)
	}
	if fromIdx != 1 || toIdx != 2 || total != 2 {
		t.Errorf("Callback args: from=%d, to=%d, total=%d; want from=1, to=2, total=2", fromIdx, toIdx, total)
	}
}

func TestKeyRotationOnFailure(t *testing.T) {
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		auth := r.Header.Get("Authorization")

		if auth == "Bearer key1" {
			// First key fails
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(ErrorResponse{
				Error: struct {
					Message string `json:"message"`
				}{Message: "Invalid key"},
			})
			return
		}

		if auth == "Bearer key2" {
			// Second key succeeds
			resp := ChatResponse{
				Choices: []StreamChoice{
					{Message: Message{Content: "Success with key2"}},
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		t.Errorf("Unexpected auth header: %s", auth)
	}))
	defer server.Close()

	cfg := &config.Config{
		APIURL:          server.URL,
		APIKey:          "key1",
		APIKeys:         []string{"key1", "key2"},
		CurrentKeyIndex: 0,
		Model:           "sonar-pro",
		Timeout:         10 * time.Second,
	}
	cfg.ResetKeyRotation()

	client := NewClient(cfg)
	resp, err := client.Query("Test")

	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	if resp.GetContent() != "Success with key2" {
		t.Errorf("Content = %q, want %q", resp.GetContent(), "Success with key2")
	}

	if requestCount != 2 {
		t.Errorf("Request count = %d, want 2", requestCount)
	}
}
