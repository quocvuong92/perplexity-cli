package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/quocvuong92/perplexity-cli/internal/config"
	"github.com/quocvuong92/perplexity-cli/internal/ratelimit"
	"github.com/quocvuong92/perplexity-cli/internal/retry"
)

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content,omitempty"`
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

// Error implements the error interface
func (e *APIError) Error() string {
	return e.Message
}

// Client is the Perplexity API client
type Client struct {
	httpClient    *http.Client
	config        *config.Config
	retryConfig   retry.Config
	rateLimiter   *ratelimit.Limiter
	onKeyRotation func(fromIndex, toIndex int, totalKeys int) // Callback when key is rotated
	onRetry       func(info retry.RetryInfo)                  // Callback when retrying
}

// NewClient creates a new API client
func NewClient(cfg *config.Config) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		config:      cfg,
		retryConfig: retry.DefaultConfig(),
		rateLimiter: ratelimit.NewLimiter(cfg.RateLimit),
	}
}

// SetKeyRotationCallback sets a callback function to be called when key rotation occurs
func (c *Client) SetKeyRotationCallback(callback func(fromIndex, toIndex int, totalKeys int)) {
	c.onKeyRotation = callback
}

// SetRetryCallback sets a callback function to be called before each retry attempt
func (c *Client) SetRetryCallback(callback func(info retry.RetryInfo)) {
	c.onRetry = callback
}

// SetRetryConfig sets the retry configuration
func (c *Client) SetRetryConfig(cfg retry.Config) {
	c.retryConfig = cfg
}

// SetBaseURL sets the API URL (useful for testing with mock servers)
func (c *Client) SetBaseURL(url string) {
	c.config.APIURL = url
}

// shouldRotateKey checks if the error indicates we should try another key
func (c *Client) shouldRotateKey(statusCode int, errorMsg string) bool {
	// Check status codes that indicate key issues
	if slices.Contains(config.RotatableErrorCodes, statusCode) {
		return true
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
	return c.QueryContext(context.Background(), message)
}

// QueryContext sends a query to the Perplexity API with context support (non-streaming)
func (c *Client) QueryContext(ctx context.Context, message string) (*ChatResponse, error) {
	return c.queryWithRetry(ctx, message)
}

// queryWithRetry performs the query with automatic key rotation on failure
func (c *Client) queryWithRetry(ctx context.Context, message string) (*ChatResponse, error) {
	// If only one key, no retry needed
	if c.config.GetKeyCount() <= 1 {
		return c.doQuery(ctx, message)
	}

	for {
		resp, err := c.doQuery(ctx, message)
		if err == nil {
			c.config.ResetKeyRotation()
			return resp, nil
		}

		// Check if context was cancelled
		if ctx.Err() != nil {
			return nil, ctx.Err()
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
func (c *Client) doQuery(ctx context.Context, message string) (*ChatResponse, error) {
	messages := []Message{
		{Role: "system", Content: config.DefaultSystemMessage},
		{Role: "user", Content: message},
	}
	return c.doQueryWithHistory(ctx, messages)
}

// QueryStream sends a streaming query to the Perplexity API
func (c *Client) QueryStream(message string, onChunk func(content string), onDone func(resp *ChatResponse)) error {
	return c.QueryStreamContext(context.Background(), message, onChunk, onDone)
}

// QueryStreamContext sends a streaming query to the Perplexity API with context support
func (c *Client) QueryStreamContext(ctx context.Context, message string, onChunk func(content string), onDone func(resp *ChatResponse)) error {
	return c.queryStreamWithRetry(ctx, message, onChunk, onDone)
}

// queryStreamWithRetry performs the streaming query with automatic key rotation on failure
// Note: Key rotation only happens before streaming starts (on HTTP errors).
// Once streaming begins successfully, mid-stream errors are not retried to avoid duplicate content.
func (c *Client) queryStreamWithRetry(ctx context.Context, message string, onChunk func(content string), onDone func(resp *ChatResponse)) error {
	// If only one key, no retry needed
	if c.config.GetKeyCount() <= 1 {
		return c.doQueryStream(ctx, message, onChunk, onDone)
	}

	for {
		err := c.doQueryStream(ctx, message, onChunk, onDone)
		if err == nil {
			c.config.ResetKeyRotation()
			return nil
		}

		// Check if context was cancelled
		if ctx.Err() != nil {
			return ctx.Err()
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
func (c *Client) doQueryStream(ctx context.Context, message string, onChunk func(content string), onDone func(resp *ChatResponse)) error {
	messages := []Message{
		{Role: "system", Content: config.DefaultSystemMessage},
		{Role: "user", Content: message},
	}
	return c.doQueryStreamWithHistory(ctx, messages, onChunk, onDone)
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

// QueryWithHistory sends a query with message history (for interactive mode)
func (c *Client) QueryWithHistory(messages []Message) (*ChatResponse, error) {
	return c.QueryWithHistoryContext(context.Background(), messages)
}

// QueryWithHistoryContext sends a query with message history and context support
func (c *Client) QueryWithHistoryContext(ctx context.Context, messages []Message) (*ChatResponse, error) {
	return c.queryWithHistoryRetry(ctx, messages)
}

func (c *Client) queryWithHistoryRetry(ctx context.Context, messages []Message) (*ChatResponse, error) {
	if c.config.GetKeyCount() <= 1 {
		return c.doQueryWithHistory(ctx, messages)
	}

	for {
		resp, err := c.doQueryWithHistory(ctx, messages)
		if err == nil {
			c.config.ResetKeyRotation()
			return resp, nil
		}

		// Check if context was cancelled
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		apiErr, ok := err.(*APIError)
		if !ok || !c.shouldRotateKey(apiErr.StatusCode, apiErr.Message) {
			return nil, err
		}

		if rotateErr := c.rotateKey(); rotateErr != nil {
			return nil, fmt.Errorf("%v (no more API keys available)", err)
		}
	}
}

func (c *Client) doQueryWithHistory(ctx context.Context, messages []Message) (*ChatResponse, error) {
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	reqBody := ChatRequest{
		Model:    c.config.Model,
		Messages: messages,
		Stream:   false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var chatResp *ChatResponse

	err = retry.Do(ctx, c.retryConfig, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.APIURL, bytes.NewBuffer(jsonData))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to send request: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
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

		var parsed ChatResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		chatResp = &parsed
		return nil
	}, c.onRetry)

	if err != nil {
		return nil, err
	}

	return chatResp, nil
}

// QueryStreamWithHistory sends a streaming query with message history (for interactive mode)
func (c *Client) QueryStreamWithHistory(messages []Message, onChunk func(content string), onDone func(resp *ChatResponse)) error {
	return c.QueryStreamWithHistoryContext(context.Background(), messages, onChunk, onDone)
}

// QueryStreamWithHistoryContext sends a streaming query with message history and context support
func (c *Client) QueryStreamWithHistoryContext(ctx context.Context, messages []Message, onChunk func(content string), onDone func(resp *ChatResponse)) error {
	return c.queryStreamWithHistoryRetry(ctx, messages, onChunk, onDone)
}

func (c *Client) queryStreamWithHistoryRetry(ctx context.Context, messages []Message, onChunk func(content string), onDone func(resp *ChatResponse)) error {
	if c.config.GetKeyCount() <= 1 {
		return c.doQueryStreamWithHistory(ctx, messages, onChunk, onDone)
	}

	for {
		err := c.doQueryStreamWithHistory(ctx, messages, onChunk, onDone)
		if err == nil {
			c.config.ResetKeyRotation()
			return nil
		}

		// Check if context was cancelled
		if ctx.Err() != nil {
			return ctx.Err()
		}

		apiErr, ok := err.(*APIError)
		if !ok || !c.shouldRotateKey(apiErr.StatusCode, apiErr.Message) {
			return err
		}

		if rotateErr := c.rotateKey(); rotateErr != nil {
			return fmt.Errorf("%v (no more API keys available)", err)
		}
	}
}

func (c *Client) doQueryStreamWithHistory(ctx context.Context, messages []Message, onChunk func(content string), onDone func(resp *ChatResponse)) error {
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return err
	}

	reqBody := ChatRequest{
		Model:    c.config.Model,
		Messages: messages,
		Stream:   true,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Use retry logic for the initial connection
	var resp *http.Response
	err = retry.Do(ctx, c.retryConfig, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.APIURL, bytes.NewBuffer(jsonData))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

		resp, err = c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to send request: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
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

		return nil
	}, c.onRetry)

	if err != nil {
		return err
	}
	defer func() {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
	}()

	var finalResp *ChatResponse
	reader := bufio.NewReader(resp.Body)

	for {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

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

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			onChunk(chunk.Choices[0].Delta.Content)
		}

		if len(chunk.Citations) > 0 || chunk.Usage.TotalTokens > 0 {
			finalResp = &chunk
		}
	}

	if onDone != nil && finalResp != nil {
		onDone(finalResp)
	}

	return nil
}
