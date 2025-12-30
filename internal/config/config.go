package config

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/quocvuong92/perplexity-cli/internal/validation"
)

// AvailableModels lists all supported Perplexity models
var AvailableModels = []string{
	"sonar-reasoning-pro",
	"sonar-reasoning",
	"sonar-pro",
	"sonar",
	"sonar-deep-research",
}

// DefaultModel is the default model to use
const DefaultModel = "sonar-pro"

// DefaultSystemMessage is the default system prompt
const DefaultSystemMessage = "Be precise and concise."

// FailedResponsePlaceholder is used when API returns empty response
const FailedResponsePlaceholder = "I apologize, but I couldn't generate a response."

// DefaultAPIURL is the Perplexity API endpoint
const DefaultAPIURL = "https://api.perplexity.ai/chat/completions"

// DefaultTimeout is the default HTTP client timeout
const DefaultTimeout = 120 * time.Second

// Environment variable names
const (
	EnvAPIKeys = "PERPLEXITY_API_KEYS" // Comma-separated list of API keys
	EnvAPIKey  = "PERPLEXITY_API_KEY"  // Single API key (fallback)
	EnvTimeout = "PERPLEXITY_TIMEOUT"  // Timeout in seconds
)

// Config holds the application configuration
type Config struct {
	APIURL          string
	APIKey          string   // Current active API key
	APIKeys         []string // All available API keys
	CurrentKeyIndex int      // Index of current key in APIKeys
	startKeyIndex   int      // Starting index for rotation cycle detection (-1 = not tracking)
	Model           string
	Timeout         time.Duration // HTTP client timeout
	Usage           bool
	Citations       bool
	Stream          bool
	Render          bool   // Render markdown output with colors/formatting
	Interactive     bool   // Interactive chat mode
	OutputFile      string // Output file path for saving response
}

// ErrAPIKeyNotFound is returned when no API key is available
var ErrAPIKeyNotFound = errors.New("API key not found. Set PERPLEXITY_API_KEYS or PERPLEXITY_API_KEY environment variable, or use --api-key flag")

// ErrNoAvailableKeys is returned when all keys are exhausted
var ErrNoAvailableKeys = errors.New("all API keys exhausted")

// ErrInvalidModel is returned when an invalid model is specified
var ErrInvalidModel = errors.New("invalid model specified")

// Error codes that should trigger key rotation
// 401: Unauthorized (invalid/revoked key)
// 403: Forbidden (key doesn't have permission)
// 429: Too Many Requests (rate limited)
// Note: 402 (Payment Required) is not included as it typically requires user action
var RotatableErrorCodes = []int{401, 403, 429}

// Error message patterns that indicate credit exhaustion
// These are specific phrases to avoid false positives
var CreditExhaustedPatterns = []string{
	"insufficient credit",
	"credit exhausted",
	"credit limit",
	"out of credit",
	"no credit",
	"balance exhausted",
	"insufficient balance",
	"quota exceeded",
	"quota limit",
	"rate limit exceeded",
	"account blocked",
	"key blocked",
	"api key blocked",
}

// ValidateModel checks if the given model is valid
func ValidateModel(model string) bool {
	return slices.Contains(AvailableModels, model)
}

// GetAvailableModelsString returns a formatted string of available models
func GetAvailableModelsString() string {
	return strings.Join(AvailableModels, ", ")
}

// GetAPIKeysFromEnv retrieves API keys from environment variables
// First tries PERPLEXITY_API_KEYS (comma-separated), then falls back to PERPLEXITY_API_KEY
func GetAPIKeysFromEnv() []string {
	// Try PERPLEXITY_API_KEYS first (comma-separated)
	if keysEnv := os.Getenv(EnvAPIKeys); keysEnv != "" {
		keys := strings.Split(keysEnv, ",")
		var result []string
		for _, key := range keys {
			key = strings.TrimSpace(key)
			if key != "" {
				result = append(result, key)
			}
		}
		if len(result) > 0 {
			return result
		}
	}

	// Fall back to PERPLEXITY_API_KEY
	if key := os.Getenv(EnvAPIKey); key != "" {
		return []string{strings.TrimSpace(key)}
	}

	return nil
}

// NewConfig creates a new Config with defaults
func NewConfig() *Config {
	return &Config{
		APIURL:        DefaultAPIURL,
		Model:         DefaultModel,
		Timeout:       DefaultTimeout,
		startKeyIndex: -1,
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Load timeout from environment if not already set to non-default
	if c.Timeout == DefaultTimeout {
		if timeoutStr := os.Getenv(EnvTimeout); timeoutStr != "" {
			if seconds, err := strconv.Atoi(timeoutStr); err == nil && seconds > 0 {
				c.Timeout = time.Duration(seconds) * time.Second
			}
		}
	}

	// If API key is provided via flag, use it directly (single key mode)
	if c.APIKey != "" {
		// Validate the API key format
		result := validation.ValidateAPIKey(c.APIKey)
		if !result.Valid {
			return fmt.Errorf("invalid API key: %w", result.Error)
		}
		c.APIKeys = []string{c.APIKey}
		c.CurrentKeyIndex = 0
		if !ValidateModel(c.Model) {
			return fmt.Errorf("%w: %s. Available models: %s", ErrInvalidModel, c.Model, GetAvailableModelsString())
		}
		return nil
	}

	// Load keys from environment
	c.APIKeys = GetAPIKeysFromEnv()
	if len(c.APIKeys) == 0 {
		return ErrAPIKeyNotFound
	}

	// Validate all API keys
	for i, key := range c.APIKeys {
		result := validation.ValidateAPIKey(key)
		if !result.Valid {
			return fmt.Errorf("invalid API key %d: %w", i+1, result.Error)
		}
	}

	// Random starting key for load balancing across multiple keys
	c.CurrentKeyIndex = rand.IntN(len(c.APIKeys))
	c.APIKey = c.APIKeys[c.CurrentKeyIndex]

	if !ValidateModel(c.Model) {
		return fmt.Errorf("%w: %s. Available models: %s", ErrInvalidModel, c.Model, GetAvailableModelsString())
	}

	return nil
}

// RotateKey moves to the next available API key, wrapping around to try all keys
// Returns the new key or error if all keys have been tried
func (c *Config) RotateKey() (string, error) {
	if len(c.APIKeys) <= 1 {
		return "", ErrNoAvailableKeys
	}

	// Track starting position to detect full cycle
	if c.startKeyIndex < 0 {
		c.startKeyIndex = c.CurrentKeyIndex
	}

	nextIndex := (c.CurrentKeyIndex + 1) % len(c.APIKeys)

	// If we've cycled back to start, all keys have been tried
	if nextIndex == c.startKeyIndex {
		c.startKeyIndex = -1 // Reset for next rotation cycle
		return "", ErrNoAvailableKeys
	}

	c.CurrentKeyIndex = nextIndex
	c.APIKey = c.APIKeys[nextIndex]
	return c.APIKey, nil
}

// ResetKeyRotation resets the key rotation tracking (call after successful request)
func (c *Config) ResetKeyRotation() {
	c.startKeyIndex = -1
}

// GetKeyCount returns the total number of configured keys
func (c *Config) GetKeyCount() int {
	return len(c.APIKeys)
}
