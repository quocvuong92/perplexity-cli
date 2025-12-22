package config

import (
	"os"
	"testing"
	"time"
)

func TestValidateModel(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{"sonar-pro", true},
		{"sonar", true},
		{"sonar-reasoning-pro", true},
		{"sonar-reasoning", true},
		{"sonar-deep-research", true},
		{"invalid-model", false},
		{"", false},
		{"gpt-4", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			if got := ValidateModel(tt.model); got != tt.want {
				t.Errorf("ValidateModel(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestGetAvailableModelsString(t *testing.T) {
	result := GetAvailableModelsString()
	if result == "" {
		t.Error("GetAvailableModelsString() returned empty string")
	}
	// Should contain at least the default model
	if !contains(result, DefaultModel) {
		t.Errorf("GetAvailableModelsString() should contain %q", DefaultModel)
	}
}

func TestGetAPIKeysFromEnv(t *testing.T) {
	// Save original env vars
	origKeys := os.Getenv(EnvAPIKeys)
	origKey := os.Getenv(EnvAPIKey)
	defer func() {
		os.Setenv(EnvAPIKeys, origKeys)
		os.Setenv(EnvAPIKey, origKey)
	}()

	tests := []struct {
		name     string
		envKeys  string
		envKey   string
		wantLen  int
		wantKeys []string
	}{
		{
			name:     "multiple keys from PERPLEXITY_API_KEYS",
			envKeys:  "key1,key2,key3",
			envKey:   "",
			wantLen:  3,
			wantKeys: []string{"key1", "key2", "key3"},
		},
		{
			name:     "single key from PERPLEXITY_API_KEY",
			envKeys:  "",
			envKey:   "single-key",
			wantLen:  1,
			wantKeys: []string{"single-key"},
		},
		{
			name:     "PERPLEXITY_API_KEYS takes precedence",
			envKeys:  "key1,key2",
			envKey:   "ignored-key",
			wantLen:  2,
			wantKeys: []string{"key1", "key2"},
		},
		{
			name:    "no keys set",
			envKeys: "",
			envKey:  "",
			wantLen: 0,
		},
		{
			name:     "keys with spaces",
			envKeys:  " key1 , key2 , key3 ",
			envKey:   "",
			wantLen:  3,
			wantKeys: []string{"key1", "key2", "key3"},
		},
		{
			name:    "empty keys filtered out",
			envKeys: "key1,,key2,",
			envKey:  "",
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv(EnvAPIKeys, tt.envKeys)
			os.Setenv(EnvAPIKey, tt.envKey)

			keys := GetAPIKeysFromEnv()
			if len(keys) != tt.wantLen {
				t.Errorf("GetAPIKeysFromEnv() returned %d keys, want %d", len(keys), tt.wantLen)
			}

			for i, wantKey := range tt.wantKeys {
				if i < len(keys) && keys[i] != wantKey {
					t.Errorf("GetAPIKeysFromEnv()[%d] = %q, want %q", i, keys[i], wantKey)
				}
			}
		})
	}
}

func TestNewConfig(t *testing.T) {
	cfg := NewConfig()

	if cfg.APIURL != DefaultAPIURL {
		t.Errorf("NewConfig().APIURL = %q, want %q", cfg.APIURL, DefaultAPIURL)
	}
	if cfg.Model != DefaultModel {
		t.Errorf("NewConfig().Model = %q, want %q", cfg.Model, DefaultModel)
	}
	if cfg.Timeout != DefaultTimeout {
		t.Errorf("NewConfig().Timeout = %v, want %v", cfg.Timeout, DefaultTimeout)
	}
	if cfg.startKeyIndex != -1 {
		t.Errorf("NewConfig().startKeyIndex = %d, want -1", cfg.startKeyIndex)
	}
}

func TestConfigValidate(t *testing.T) {
	// Save original env vars
	origKeys := os.Getenv(EnvAPIKeys)
	origKey := os.Getenv(EnvAPIKey)
	origTimeout := os.Getenv(EnvTimeout)
	defer func() {
		os.Setenv(EnvAPIKeys, origKeys)
		os.Setenv(EnvAPIKey, origKey)
		os.Setenv(EnvTimeout, origTimeout)
	}()

	t.Run("valid config with API key flag", func(t *testing.T) {
		os.Setenv(EnvAPIKeys, "")
		os.Setenv(EnvAPIKey, "")

		cfg := NewConfig()
		cfg.APIKey = "test-key"

		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate() error = %v, want nil", err)
		}
		if len(cfg.APIKeys) != 1 || cfg.APIKeys[0] != "test-key" {
			t.Error("APIKeys not set correctly from flag")
		}
	})

	t.Run("valid config from env", func(t *testing.T) {
		os.Setenv(EnvAPIKeys, "env-key")
		os.Setenv(EnvAPIKey, "")

		cfg := NewConfig()
		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate() error = %v, want nil", err)
		}
	})

	t.Run("missing API key", func(t *testing.T) {
		os.Setenv(EnvAPIKeys, "")
		os.Setenv(EnvAPIKey, "")

		cfg := NewConfig()
		if err := cfg.Validate(); err != ErrAPIKeyNotFound {
			t.Errorf("Validate() error = %v, want ErrAPIKeyNotFound", err)
		}
	})

	t.Run("invalid model", func(t *testing.T) {
		os.Setenv(EnvAPIKeys, "test-key")

		cfg := NewConfig()
		cfg.Model = "invalid-model"

		err := cfg.Validate()
		if err == nil {
			t.Error("Validate() should return error for invalid model")
		}
	})

	t.Run("timeout from env", func(t *testing.T) {
		os.Setenv(EnvAPIKeys, "test-key")
		os.Setenv(EnvTimeout, "60")

		cfg := NewConfig()
		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate() error = %v", err)
		}
		if cfg.Timeout != 60*time.Second {
			t.Errorf("Timeout = %v, want 60s", cfg.Timeout)
		}
	})
}

func TestRotateKey(t *testing.T) {
	t.Run("single key returns error", func(t *testing.T) {
		cfg := &Config{
			APIKeys:         []string{"key1"},
			CurrentKeyIndex: 0,
			startKeyIndex:   -1,
		}

		_, err := cfg.RotateKey()
		if err != ErrNoAvailableKeys {
			t.Errorf("RotateKey() error = %v, want ErrNoAvailableKeys", err)
		}
	})

	t.Run("rotates through all keys", func(t *testing.T) {
		cfg := &Config{
			APIKeys:         []string{"key0", "key1", "key2"},
			CurrentKeyIndex: 0,
			APIKey:          "key0",
			startKeyIndex:   -1,
		}

		// First rotation: 0 -> 1
		key, err := cfg.RotateKey()
		if err != nil {
			t.Errorf("RotateKey() error = %v", err)
		}
		if key != "key1" || cfg.CurrentKeyIndex != 1 {
			t.Errorf("After first rotation: key=%q, index=%d", key, cfg.CurrentKeyIndex)
		}

		// Second rotation: 1 -> 2
		key, err = cfg.RotateKey()
		if err != nil {
			t.Errorf("RotateKey() error = %v", err)
		}
		if key != "key2" || cfg.CurrentKeyIndex != 2 {
			t.Errorf("After second rotation: key=%q, index=%d", key, cfg.CurrentKeyIndex)
		}

		// Third rotation: 2 -> 0 (wrap), but should fail since we started at 0
		_, err = cfg.RotateKey()
		if err != ErrNoAvailableKeys {
			t.Errorf("RotateKey() should fail after full cycle, got %v", err)
		}
	})

	t.Run("wraps around from middle", func(t *testing.T) {
		cfg := &Config{
			APIKeys:         []string{"key0", "key1", "key2"},
			CurrentKeyIndex: 1,
			APIKey:          "key1",
			startKeyIndex:   -1,
		}

		// First rotation: 1 -> 2
		key, _ := cfg.RotateKey()
		if key != "key2" {
			t.Errorf("Expected key2, got %s", key)
		}

		// Second rotation: 2 -> 0 (wrap)
		key, _ = cfg.RotateKey()
		if key != "key0" {
			t.Errorf("Expected key0, got %s", key)
		}

		// Third rotation: should fail (back to start index 1)
		_, err := cfg.RotateKey()
		if err != ErrNoAvailableKeys {
			t.Errorf("Expected ErrNoAvailableKeys, got %v", err)
		}
	})
}

func TestResetKeyRotation(t *testing.T) {
	cfg := &Config{startKeyIndex: 5}
	cfg.ResetKeyRotation()
	if cfg.startKeyIndex != -1 {
		t.Errorf("ResetKeyRotation() didn't reset startKeyIndex")
	}
}

func TestGetKeyCount(t *testing.T) {
	tests := []struct {
		keys []string
		want int
	}{
		{nil, 0},
		{[]string{}, 0},
		{[]string{"key1"}, 1},
		{[]string{"key1", "key2", "key3"}, 3},
	}

	for _, tt := range tests {
		cfg := &Config{APIKeys: tt.keys}
		if got := cfg.GetKeyCount(); got != tt.want {
			t.Errorf("GetKeyCount() = %d, want %d", got, tt.want)
		}
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
