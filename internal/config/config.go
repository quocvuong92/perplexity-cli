package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// AvailableModels lists all supported Perplexity models
var AvailableModels = []string{
	"sonar-reasoning-pro",
	"sonar-reasoning",
	"sonar-pro",
	"sonar",
}

// DefaultModel is the default model to use
const DefaultModel = "sonar-pro"

// DefaultAPIURL is the Perplexity API endpoint
const DefaultAPIURL = "https://api.perplexity.ai/chat/completions"

// Environment variable names
const (
	EnvAPIKeys = "PERPLEXITY_API_KEYS" // Comma-separated list of API keys
	EnvAPIKey  = "PERPLEXITY_API_KEY"  // Single API key (fallback)
)

// Config holds the application configuration
type Config struct {
	APIURL          string
	APIKey          string   // Current active API key
	APIKeys         []string // All available API keys
	CurrentKeyIndex int      // Index of current key in APIKeys
	Model           string
	Usage           bool
	Citations       bool
	Stream          bool
	Verbose         bool
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
	for _, m := range AvailableModels {
		if m == model {
			return true
		}
	}
	return false
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
		APIURL: DefaultAPIURL,
		Model:  DefaultModel,
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// If API key is provided via flag, use it directly (single key mode)
	if c.APIKey != "" {
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

	c.CurrentKeyIndex = 0
	c.APIKey = c.APIKeys[0]

	if !ValidateModel(c.Model) {
		return fmt.Errorf("%w: %s. Available models: %s", ErrInvalidModel, c.Model, GetAvailableModelsString())
	}

	return nil
}

// RotateKey moves to the next available API key
// Returns the new key or error if no more keys available
func (c *Config) RotateKey() (string, error) {
	if len(c.APIKeys) <= 1 {
		return "", ErrNoAvailableKeys
	}

	nextIndex := c.CurrentKeyIndex + 1
	if nextIndex >= len(c.APIKeys) {
		return "", ErrNoAvailableKeys
	}

	c.CurrentKeyIndex = nextIndex
	c.APIKey = c.APIKeys[nextIndex]
	return c.APIKey, nil
}

// GetKeyCount returns the total number of configured keys
func (c *Config) GetKeyCount() int {
	return len(c.APIKeys)
}

// GetRemainingKeyCount returns the number of keys from current index onwards (including current key)
func (c *Config) GetRemainingKeyCount() int {
	return len(c.APIKeys) - c.CurrentKeyIndex
}

// MaskKey returns a masked version of the API key for display
func MaskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}
