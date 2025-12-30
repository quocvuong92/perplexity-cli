package validation

import (
	"strings"
	"testing"
)

func TestValidatePrompt(t *testing.T) {
	tests := []struct {
		name      string
		prompt    string
		wantValid bool
		wantError error
	}{
		{
			name:      "valid prompt",
			prompt:    "What is the weather today?",
			wantValid: true,
		},
		{
			name:      "valid prompt with whitespace",
			prompt:    "  Hello world  ",
			wantValid: true,
		},
		{
			name:      "empty prompt",
			prompt:    "",
			wantValid: false,
			wantError: ErrEmptyPrompt,
		},
		{
			name:      "whitespace only",
			prompt:    "   \t\n  ",
			wantValid: false,
			wantError: ErrEmptyPrompt,
		},
		{
			name:      "valid multiline prompt",
			prompt:    "Line 1\nLine 2\nLine 3",
			wantValid: true,
		},
		{
			name:      "unicode prompt",
			prompt:    "你好世界 こんにちは 🌍",
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidatePrompt(tt.prompt)
			if result.Valid != tt.wantValid {
				t.Errorf("ValidatePrompt(%q).Valid = %v, want %v", tt.prompt, result.Valid, tt.wantValid)
			}
			if tt.wantError != nil && result.Error == nil {
				t.Errorf("ValidatePrompt(%q).Error = nil, want error containing %v", tt.prompt, tt.wantError)
			}
		})
	}
}

func TestValidatePromptTooLong(t *testing.T) {
	// Create a prompt that exceeds the maximum length
	longPrompt := strings.Repeat("a", MaxPromptLength+1)
	result := ValidatePrompt(longPrompt)

	if result.Valid {
		t.Error("ValidatePrompt should reject prompts exceeding MaxPromptLength")
	}

	if result.Error == nil {
		t.Error("ValidatePrompt should return an error for long prompts")
	}
}

func TestValidatePromptCleaned(t *testing.T) {
	result := ValidatePrompt("  hello world  ")
	if !result.Valid {
		t.Fatal("Expected valid result")
	}
	if result.Cleaned != "hello world" {
		t.Errorf("Expected cleaned prompt 'hello world', got %q", result.Cleaned)
	}
}

func TestValidateAPIKey(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		wantValid   bool
		wantWarning bool
	}{
		{
			name:        "valid key with pplx prefix",
			key:         "pplx-abcdefghij1234567890",
			wantValid:   true,
			wantWarning: false,
		},
		{
			name:        "valid key without pplx prefix",
			key:         "sk-abcdefghij1234567890abcd",
			wantValid:   true,
			wantWarning: true, // Should warn about missing prefix
		},
		{
			name:      "empty key",
			key:       "",
			wantValid: false,
		},
		{
			name:      "key too short",
			key:       "abc123",
			wantValid: false,
		},
		{
			name:      "key with invalid characters",
			key:       "pplx-abc@def#ghi$jkl!mnop",
			wantValid: false,
		},
		{
			name:      "key with spaces",
			key:       "pplx-abc def ghi jkl mnop",
			wantValid: false,
		},
		{
			name:        "key with hyphens and underscores",
			key:         "pplx-abc_def-ghi_jkl-mnop",
			wantValid:   true,
			wantWarning: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateAPIKey(tt.key)
			if result.Valid != tt.wantValid {
				t.Errorf("ValidateAPIKey(%q).Valid = %v, want %v (error: %v)", tt.key, result.Valid, tt.wantValid, result.Error)
			}
			if tt.wantWarning && result.Warning == "" {
				t.Errorf("ValidateAPIKey(%q) expected warning but got none", tt.key)
			}
			if !tt.wantWarning && result.Warning != "" {
				t.Errorf("ValidateAPIKey(%q) unexpected warning: %s", tt.key, result.Warning)
			}
		})
	}
}

func TestValidateAPIKeyTooLong(t *testing.T) {
	longKey := "pplx-" + strings.Repeat("a", MaxAPIKeyLength+1)
	result := ValidateAPIKey(longKey)

	if result.Valid {
		t.Error("ValidateAPIKey should reject keys exceeding MaxAPIKeyLength")
	}
}

func TestValidateAPIKeys(t *testing.T) {
	tests := []struct {
		name      string
		keys      []string
		wantError bool
	}{
		{
			name:      "empty keys",
			keys:      []string{},
			wantError: true,
		},
		{
			name:      "all valid keys",
			keys:      []string{"pplx-abcdefghij1234567890", "pplx-zyxwvutsrq0987654321"},
			wantError: false,
		},
		{
			name:      "one invalid key",
			keys:      []string{"pplx-abcdefghij1234567890", "short"},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateAPIKeys(tt.keys)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateAPIKeys() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestSanitizePrompt(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		output string
	}{
		{
			name:   "normal text",
			input:  "Hello world",
			output: "Hello world",
		},
		{
			name:   "with newlines",
			input:  "Line 1\nLine 2",
			output: "Line 1\nLine 2",
		},
		{
			name:   "with tabs",
			input:  "Column1\tColumn2",
			output: "Column1\tColumn2",
		},
		{
			name:   "with null bytes",
			input:  "Hello\x00World",
			output: "HelloWorld",
		},
		{
			name:   "with control characters",
			input:  "Hello\x01\x02\x03World",
			output: "HelloWorld",
		},
		{
			name:   "unicode preserved",
			input:  "日本語 🎉 emoji",
			output: "日本語 🎉 emoji",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizePrompt(tt.input)
			if result != tt.output {
				t.Errorf("SanitizePrompt(%q) = %q, want %q", tt.input, result, tt.output)
			}
		})
	}
}

func TestIsValidAPIKeyChar(t *testing.T) {
	validChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_"
	for _, c := range validChars {
		if !isValidAPIKeyChar(c) {
			t.Errorf("isValidAPIKeyChar(%q) = false, expected true", c)
		}
	}

	invalidChars := "!@#$%^&*()+=[]{}|;':\",./<>? "
	for _, c := range invalidChars {
		if isValidAPIKeyChar(c) {
			t.Errorf("isValidAPIKeyChar(%q) = true, expected false", c)
		}
	}
}
