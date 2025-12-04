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

// EnvAPIKey is the environment variable name for the API key
const EnvAPIKey = "PERPLEXITY_API_KEY"

// Config holds the application configuration
type Config struct {
	APIURL    string
	APIKey    string
	Model     string
	Usage     bool
	Citations bool
	Verbose   bool
}

// ErrAPIKeyNotFound is returned when no API key is available
var ErrAPIKeyNotFound = errors.New("API key not found. Set PERPLEXITY_API_KEY environment variable or use --api-key flag")

// ErrInvalidModel is returned when an invalid model is specified
var ErrInvalidModel = errors.New("invalid model specified")

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

// GetAPIKeyFromEnv retrieves the API key from environment variable
func GetAPIKeyFromEnv() string {
	return os.Getenv(EnvAPIKey)
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
	if c.APIKey == "" {
		c.APIKey = GetAPIKeyFromEnv()
		if c.APIKey == "" {
			return ErrAPIKeyNotFound
		}
	}

	if !ValidateModel(c.Model) {
		return fmt.Errorf("%w: %s. Available models: %s", ErrInvalidModel, c.Model, GetAvailableModelsString())
	}

	return nil
}
