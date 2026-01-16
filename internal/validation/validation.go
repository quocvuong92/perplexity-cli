package validation

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Validation errors
var (
	ErrEmptyPrompt        = errors.New("prompt cannot be empty")
	ErrPromptTooLong      = errors.New("prompt exceeds maximum length")
	ErrInvalidAPIKey      = errors.New("invalid API key format")
	ErrAPIKeyTooShort     = errors.New("API key is too short")
	ErrAPIKeyInvalidChars = errors.New("API key contains invalid characters")
)

// Limits for validation
const (
	// MaxPromptLength is the maximum allowed prompt length in characters.
	// This is a reasonable limit to prevent abuse while accommodating long prompts.
	// Perplexity API has token limits (~127k for sonar-pro), but tokens != characters.
	// 100k characters is approximately 25-50k tokens depending on language.
	MaxPromptLength = 100000

	// MinAPIKeyLength is the minimum expected API key length.
	// Perplexity API keys (pplx-xxx) are typically 40+ characters.
	// 20 is a conservative minimum to catch obvious mistakes.
	MinAPIKeyLength = 20

	// MaxAPIKeyLength is the maximum expected API key length.
	// Most API keys are under 100 characters. 256 provides headroom
	// for future key format changes while catching paste errors.
	MaxAPIKeyLength = 256
)

// PromptResult contains the result of prompt validation
type PromptResult struct {
	Valid   bool
	Error   error
	Cleaned string // The cleaned/trimmed prompt
}

// ValidatePrompt validates a user prompt
func ValidatePrompt(prompt string) PromptResult {
	// Trim whitespace
	cleaned := strings.TrimSpace(prompt)

	// Check for empty prompt
	if cleaned == "" {
		return PromptResult{
			Valid: false,
			Error: ErrEmptyPrompt,
		}
	}

	// Check length
	charCount := utf8.RuneCountInString(cleaned)
	if charCount > MaxPromptLength {
		return PromptResult{
			Valid: false,
			Error: fmt.Errorf("%w: %d characters (max: %d)", ErrPromptTooLong, charCount, MaxPromptLength),
		}
	}

	return PromptResult{
		Valid:   true,
		Cleaned: cleaned,
	}
}

// APIKeyResult contains the result of API key validation
type APIKeyResult struct {
	Valid   bool
	Error   error
	Warning string // Non-fatal warning message
}

// ValidateAPIKey validates an API key format
func ValidateAPIKey(key string) APIKeyResult {
	// Trim whitespace
	key = strings.TrimSpace(key)

	// Check for empty key
	if key == "" {
		return APIKeyResult{
			Valid: false,
			Error: ErrInvalidAPIKey,
		}
	}

	// Check minimum length
	if len(key) < MinAPIKeyLength {
		return APIKeyResult{
			Valid: false,
			Error: fmt.Errorf("%w: got %d characters, minimum is %d", ErrAPIKeyTooShort, len(key), MinAPIKeyLength),
		}
	}

	// Check maximum length
	if len(key) > MaxAPIKeyLength {
		return APIKeyResult{
			Valid: false,
			Error: fmt.Errorf("%w: got %d characters, maximum is %d", ErrInvalidAPIKey, len(key), MaxAPIKeyLength),
		}
	}

	// Check for valid characters (alphanumeric, hyphens, underscores)
	for i, r := range key {
		if !isValidAPIKeyChar(r) {
			return APIKeyResult{
				Valid: false,
				Error: fmt.Errorf("%w: invalid character at position %d", ErrAPIKeyInvalidChars, i),
			}
		}
	}

	// Check for common prefixes (Perplexity keys typically start with "pplx-")
	var warning string
	if !strings.HasPrefix(key, "pplx-") {
		warning = "API key does not start with 'pplx-' prefix; verify it's a valid Perplexity API key"
	}

	return APIKeyResult{
		Valid:   true,
		Warning: warning,
	}
}

// ValidateAPIKeys validates multiple API keys
func ValidateAPIKeys(keys []string) ([]APIKeyResult, error) {
	if len(keys) == 0 {
		return nil, errors.New("no API keys provided")
	}

	results := make([]APIKeyResult, len(keys))
	var firstError error

	for i, key := range keys {
		results[i] = ValidateAPIKey(key)
		if !results[i].Valid && firstError == nil {
			firstError = fmt.Errorf("API key %d: %w", i+1, results[i].Error)
		}
	}

	return results, firstError
}

// isValidAPIKeyChar checks if a character is valid for an API key
func isValidAPIKeyChar(r rune) bool {
	// Allow alphanumeric characters
	if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
		return true
	}
	// Allow common separators in API keys
	if r == '-' || r == '_' {
		return true
	}
	return false
}

// SanitizePrompt removes potentially problematic characters from a prompt
// while preserving the meaning. This is a light sanitization.
func SanitizePrompt(prompt string) string {
	// Remove null bytes and other control characters (except newlines and tabs)
	var builder strings.Builder
	builder.Grow(len(prompt))

	for _, r := range prompt {
		// Keep printable characters, newlines, and tabs
		if r >= 32 || r == '\n' || r == '\t' || r == '\r' {
			builder.WriteRune(r)
		}
	}

	return builder.String()
}
