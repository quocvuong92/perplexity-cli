// Package history provides conversation history persistence for interactive sessions.
package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// HistoryFileName is the name of the history file
	HistoryFileName = "conversation-history.json"
	// MaxHistoryEntries limits the number of conversations stored
	MaxHistoryEntries = 50
	// EnvHistoryPath is the environment variable for custom history path
	EnvHistoryPath = "PERPLEXITY_HISTORY_PATH"
)

// Message represents a chat message for history storage.
// This is a local type to avoid circular dependencies with the api package.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content,omitempty"`
}

// ConversationEntry represents a saved conversation
type ConversationEntry struct {
	ID        string    `json:"id"`
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// History manages conversation history persistence
type History struct {
	Conversations []ConversationEntry `json:"conversations"`
	path          string
}

// NewHistory creates a new History manager
func NewHistory() *History {
	return &History{
		Conversations: make([]ConversationEntry, 0),
		path:          getHistoryPath(),
	}
}

// getHistoryPath returns the path to the history file
func getHistoryPath() string {
	if customPath := os.Getenv(EnvHistoryPath); customPath != "" {
		return customPath
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".local", "share", "perplexity-cli", HistoryFileName)
}

// Load reads the history from disk
func (h *History) Load() error {
	if h.path == "" {
		return fmt.Errorf("history path not available")
	}

	data, err := os.ReadFile(h.path)
	if err != nil {
		if os.IsNotExist(err) {
			// No history file yet, start fresh
			return nil
		}
		return fmt.Errorf("failed to read history: %w", err)
	}

	if err := json.Unmarshal(data, h); err != nil {
		return fmt.Errorf("failed to parse history: %w", err)
	}

	return nil
}

// Save writes the history to disk
func (h *History) Save() error {
	if h.path == "" {
		return fmt.Errorf("history path not available")
	}

	// Ensure directory exists
	dir := filepath.Dir(h.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}

	// Trim to max entries
	if len(h.Conversations) > MaxHistoryEntries {
		h.Conversations = h.Conversations[len(h.Conversations)-MaxHistoryEntries:]
	}

	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal history: %w", err)
	}

	if err := os.WriteFile(h.path, data, 0600); err != nil {
		return fmt.Errorf("failed to write history: %w", err)
	}

	return nil
}

// AddConversation adds a new conversation to history
func (h *History) AddConversation(id, model string, messages []Message) {
	entry := ConversationEntry{
		ID:        id,
		Model:     model,
		Messages:  messages,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	h.Conversations = append(h.Conversations, entry)
}

// UpdateConversation updates an existing conversation
func (h *History) UpdateConversation(id string, messages []Message) bool {
	for i := range h.Conversations {
		if h.Conversations[i].ID == id {
			h.Conversations[i].Messages = messages
			h.Conversations[i].UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

// GetConversation retrieves a conversation by ID
func (h *History) GetConversation(id string) *ConversationEntry {
	for i := range h.Conversations {
		if h.Conversations[i].ID == id {
			return &h.Conversations[i]
		}
	}
	return nil
}

// GetLastConversation returns the most recent conversation
func (h *History) GetLastConversation() *ConversationEntry {
	if len(h.Conversations) == 0 {
		return nil
	}
	return &h.Conversations[len(h.Conversations)-1]
}

// Clear removes all conversation history
func (h *History) Clear() {
	h.Conversations = make([]ConversationEntry, 0)
}

// GetRecentConversations returns the N most recent conversations
func (h *History) GetRecentConversations(n int) []ConversationEntry {
	if n <= 0 || len(h.Conversations) == 0 {
		return nil
	}
	if n > len(h.Conversations) {
		n = len(h.Conversations)
	}
	return h.Conversations[len(h.Conversations)-n:]
}

// SearchConversations searches for conversations containing the keyword
func (h *History) SearchConversations(keyword string) []ConversationEntry {
	if keyword == "" || len(h.Conversations) == 0 {
		return nil
	}
	keyword = strings.ToLower(keyword)
	var results []ConversationEntry
	for _, conv := range h.Conversations {
		for _, msg := range conv.Messages {
			if strings.Contains(strings.ToLower(msg.Content), keyword) {
				results = append(results, conv)
				break
			}
		}
	}
	return results
}

// DeleteConversation removes a conversation by index (1-based from recent list)
func (h *History) DeleteConversation(index int) bool {
	recent := h.GetRecentConversations(10)
	if index < 1 || index > len(recent) {
		return false
	}
	targetID := recent[index-1].ID
	for i := range h.Conversations {
		if h.Conversations[i].ID == targetID {
			h.Conversations = append(h.Conversations[:i], h.Conversations[i+1:]...)
			return true
		}
	}
	return false
}
