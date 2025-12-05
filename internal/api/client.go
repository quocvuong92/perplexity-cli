package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/quocvuong92/perplexity-cli/internal/config"
)

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest represents the API request payload
type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream,omitempty"`
}

// Usage represents token usage statistics
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Delta represents streaming delta content
type Delta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// StreamChoice represents a streaming response choice
type StreamChoice struct {
	Delta        Delta   `json:"delta,omitempty"`
	Message      Message `json:"message,omitempty"`
	FinishReason string  `json:"finish_reason,omitempty"`
}

// ChatResponse represents the API response
type ChatResponse struct {
	Choices   []StreamChoice `json:"choices"`
	Usage     Usage          `json:"usage"`
	Citations []string       `json:"citations"`
}

// ErrorResponse represents an API error
type ErrorResponse struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// APIError represents an error with status code
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return e.Message
}

// Client is the Perplexity API client
type Client struct {
	httpClient    *http.Client
	config        *config.Config
	onKeyRotation func(fromIndex, toIndex int, totalKeys int) // Callback when key is rotated
}

// NewClient creates a new API client
func NewClient(cfg *config.Config) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		config: cfg,
	}
}

// SetKeyRotationCallback sets a callback function to be called when key rotation occurs
func (c *Client) SetKeyRotationCallback(callback func(fromIndex, toIndex int, totalKeys int)) {
	c.onKeyRotation = callback
}

// shouldRotateKey checks if the error indicates we should try another key
func (c *Client) shouldRotateKey(statusCode int, errorMsg string) bool {
	// Check status codes that indicate key issues
	for _, code := range config.RotatableErrorCodes {
		if statusCode == code {
			return true
		}
	}

	// Check error message patterns
	lowerMsg := strings.ToLower(errorMsg)
	for _, pattern := range config.CreditExhaustedPatterns {
		if strings.Contains(lowerMsg, pattern) {
			return true
		}
	}

	return false
}

// rotateKey attempts to switch to the next available API key
func (c *Client) rotateKey() error {
	oldIndex := c.config.CurrentKeyIndex
	_, err := c.config.RotateKey()
	if err != nil {
		return err
	}

	// Call the rotation callback if set
	if c.onKeyRotation != nil {
		c.onKeyRotation(oldIndex+1, c.config.CurrentKeyIndex+1, c.config.GetKeyCount())
	}

	return nil
}

// Query sends a query to the Perplexity API (non-streaming)
func (c *Client) Query(message string) (*ChatResponse, error) {
	return c.queryWithRetry(message)
}

// queryWithRetry performs the query with automatic key rotation on failure
func (c *Client) queryWithRetry(message string) (*ChatResponse, error) {
	// If only one key, no retry needed
	if c.config.GetKeyCount() <= 1 {
		return c.doQuery(message)
	}

	for {
		resp, err := c.doQuery(message)
		if err == nil {
			return resp, nil
		}

		// Check if we should rotate keys
		apiErr, ok := err.(*APIError)
		if !ok || !c.shouldRotateKey(apiErr.StatusCode, apiErr.Message) {
			return nil, err
		}

		// Try to rotate to next key
		if rotateErr := c.rotateKey(); rotateErr != nil {
			return nil, fmt.Errorf("%v (no more API keys available)", err)
		}
	}
}

// doQuery performs a single query attempt
func (c *Client) doQuery(message string) (*ChatResponse, error) {
	reqBody := ChatRequest{
		Model: c.config.Model,
		Messages: []Message{
			{Role: "system", Content: "Be precise and concise."},
			{Role: "user", Content: message},
		},
		Stream: false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.config.APIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		errMsg := fmt.Sprintf("status code %d", resp.StatusCode)
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
			errMsg = errResp.Error.Message
		}
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("API error: %s", errMsg),
		}
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &chatResp, nil
}

// QueryStream sends a streaming query to the Perplexity API
func (c *Client) QueryStream(message string, onChunk func(content string), onDone func(resp *ChatResponse)) error {
	return c.queryStreamWithRetry(message, onChunk, onDone)
}

// queryStreamWithRetry performs the streaming query with automatic key rotation on failure
// Note: Key rotation only happens before streaming starts (on HTTP errors).
// Once streaming begins successfully, mid-stream errors are not retried to avoid duplicate content.
func (c *Client) queryStreamWithRetry(message string, onChunk func(content string), onDone func(resp *ChatResponse)) error {
	// If only one key, no retry needed
	if c.config.GetKeyCount() <= 1 {
		return c.doQueryStream(message, onChunk, onDone)
	}

	for {
		err := c.doQueryStream(message, onChunk, onDone)
		if err == nil {
			return nil
		}

		// Check if we should rotate keys
		// Only APIError (HTTP status errors) trigger rotation
		// Mid-stream errors (io errors, parse errors) don't trigger rotation
		apiErr, ok := err.(*APIError)
		if !ok || !c.shouldRotateKey(apiErr.StatusCode, apiErr.Message) {
			return err
		}

		// Try to rotate to next key
		if rotateErr := c.rotateKey(); rotateErr != nil {
			return fmt.Errorf("%v (no more API keys available)", err)
		}
	}
}

// doQueryStream performs a single streaming query attempt
func (c *Client) doQueryStream(message string, onChunk func(content string), onDone func(resp *ChatResponse)) error {
	reqBody := ChatRequest{
		Model: c.config.Model,
		Messages: []Message{
			{Role: "system", Content: "Be precise and concise."},
			{Role: "user", Content: message},
		},
		Stream: true,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.config.APIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Only return APIError for HTTP status errors (before streaming starts)
	// This allows key rotation only at this stage
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		var errResp ErrorResponse
		errMsg := fmt.Sprintf("status code %d", resp.StatusCode)
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
			errMsg = errResp.Error.Message
		}
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("API error: %s", errMsg),
		}
	}

	// Once we start reading the stream, don't retry on errors
	// to avoid duplicate content being sent to onChunk
	var finalResp *ChatResponse
	reader := bufio.NewReader(resp.Body)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to read stream: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk ChatResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		// Send content chunk
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			onChunk(chunk.Choices[0].Delta.Content)
		}

		// Capture citations and usage from final chunk
		if len(chunk.Citations) > 0 || chunk.Usage.TotalTokens > 0 {
			finalResp = &chunk
		}
	}

	if onDone != nil && finalResp != nil {
		onDone(finalResp)
	}

	return nil
}

// GetContent extracts the content from the response
func (r *ChatResponse) GetContent() string {
	if len(r.Choices) > 0 {
		if r.Choices[0].Message.Content != "" {
			return r.Choices[0].Message.Content
		}
		return r.Choices[0].Delta.Content
	}
	return ""
}

// GetUsageMap returns usage as a map for display
func (r *ChatResponse) GetUsageMap() map[string]int {
	return map[string]int{
		"prompt_tokens":     r.Usage.PromptTokens,
		"completion_tokens": r.Usage.CompletionTokens,
		"total_tokens":      r.Usage.TotalTokens,
	}
}
