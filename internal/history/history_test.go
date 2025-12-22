package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewHistory(t *testing.T) {
	h := NewHistory()
	if h == nil {
		t.Fatal("NewHistory() returned nil")
	}
	if h.Conversations == nil {
		t.Error("Conversations slice is nil")
	}
	if len(h.Conversations) != 0 {
		t.Errorf("Expected empty conversations, got %d", len(h.Conversations))
	}
}

func TestAddConversation(t *testing.T) {
	h := NewHistory()

	messages := []Message{
		{Role: "system", Content: "Be helpful"},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
	}

	h.AddConversation("test-id", "sonar-pro", messages)

	if len(h.Conversations) != 1 {
		t.Fatalf("Expected 1 conversation, got %d", len(h.Conversations))
	}

	conv := h.Conversations[0]
	if conv.ID != "test-id" {
		t.Errorf("ID = %q, want %q", conv.ID, "test-id")
	}
	if conv.Model != "sonar-pro" {
		t.Errorf("Model = %q, want %q", conv.Model, "sonar-pro")
	}
	if len(conv.Messages) != 3 {
		t.Errorf("Messages count = %d, want 3", len(conv.Messages))
	}
}

func TestUpdateConversation(t *testing.T) {
	h := NewHistory()

	messages := []Message{
		{Role: "user", Content: "Hello"},
	}
	h.AddConversation("test-id", "sonar-pro", messages)

	// Update with new messages
	newMessages := []Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi!"},
		{Role: "user", Content: "How are you?"},
	}

	updated := h.UpdateConversation("test-id", newMessages)
	if !updated {
		t.Error("UpdateConversation() returned false, want true")
	}

	conv := h.GetConversation("test-id")
	if len(conv.Messages) != 3 {
		t.Errorf("Messages count = %d, want 3", len(conv.Messages))
	}

	// Try to update non-existent conversation
	updated = h.UpdateConversation("non-existent", newMessages)
	if updated {
		t.Error("UpdateConversation() for non-existent ID returned true")
	}
}

func TestGetConversation(t *testing.T) {
	h := NewHistory()

	h.AddConversation("id1", "model1", []Message{{Role: "user", Content: "msg1"}})
	h.AddConversation("id2", "model2", []Message{{Role: "user", Content: "msg2"}})

	conv := h.GetConversation("id1")
	if conv == nil {
		t.Fatal("GetConversation() returned nil for existing ID")
	}
	if conv.Model != "model1" {
		t.Errorf("Model = %q, want %q", conv.Model, "model1")
	}

	conv = h.GetConversation("non-existent")
	if conv != nil {
		t.Error("GetConversation() should return nil for non-existent ID")
	}
}

func TestGetLastConversation(t *testing.T) {
	h := NewHistory()

	// Empty history
	if h.GetLastConversation() != nil {
		t.Error("GetLastConversation() should return nil for empty history")
	}

	h.AddConversation("id1", "model1", []Message{})
	h.AddConversation("id2", "model2", []Message{})

	last := h.GetLastConversation()
	if last == nil {
		t.Fatal("GetLastConversation() returned nil")
	}
	if last.ID != "id2" {
		t.Errorf("Last conversation ID = %q, want %q", last.ID, "id2")
	}
}

func TestGetRecentConversations(t *testing.T) {
	h := NewHistory()

	// Empty history
	if h.GetRecentConversations(5) != nil {
		t.Error("GetRecentConversations() should return nil for empty history")
	}

	// Add conversations
	for i := 0; i < 5; i++ {
		h.AddConversation(string(rune('a'+i)), "model", []Message{})
	}

	tests := []struct {
		n    int
		want int
	}{
		{0, 0},
		{-1, 0},
		{3, 3},
		{5, 5},
		{10, 5}, // More than available
	}

	for _, tt := range tests {
		result := h.GetRecentConversations(tt.n)
		got := 0
		if result != nil {
			got = len(result)
		}
		if got != tt.want {
			t.Errorf("GetRecentConversations(%d) returned %d, want %d", tt.n, got, tt.want)
		}
	}
}

func TestSearchConversations(t *testing.T) {
	h := NewHistory()

	h.AddConversation("id1", "model", []Message{
		{Role: "user", Content: "How do I use Go?"},
		{Role: "assistant", Content: "Go is a programming language..."},
	})
	h.AddConversation("id2", "model", []Message{
		{Role: "user", Content: "What is Python?"},
		{Role: "assistant", Content: "Python is a language..."},
	})
	h.AddConversation("id3", "model", []Message{
		{Role: "user", Content: "Tell me about Go modules"},
	})

	tests := []struct {
		keyword string
		want    int
	}{
		{"Go", 2},       // Found in id1 and id3
		{"go", 2},       // Case insensitive
		{"Python", 1},   // Found in id2
		{"Java", 0},     // Not found
		{"language", 2}, // Found in id1 and id2
		{"", 0},         // Empty keyword
	}

	for _, tt := range tests {
		results := h.SearchConversations(tt.keyword)
		got := 0
		if results != nil {
			got = len(results)
		}
		if got != tt.want {
			t.Errorf("SearchConversations(%q) returned %d results, want %d", tt.keyword, got, tt.want)
		}
	}
}

func TestDeleteConversation(t *testing.T) {
	h := NewHistory()

	// Add 5 conversations
	for i := 0; i < 5; i++ {
		h.AddConversation(string(rune('a'+i)), "model", []Message{})
	}

	// Delete conversation at index 2 (third conversation)
	deleted := h.DeleteConversation(2)
	if !deleted {
		t.Error("DeleteConversation(2) returned false")
	}
	if len(h.Conversations) != 4 {
		t.Errorf("After delete, have %d conversations, want 4", len(h.Conversations))
	}

	// Invalid indices
	if h.DeleteConversation(0) {
		t.Error("DeleteConversation(0) should return false")
	}
	if h.DeleteConversation(100) {
		t.Error("DeleteConversation(100) should return false")
	}
	if h.DeleteConversation(-1) {
		t.Error("DeleteConversation(-1) should return false")
	}
}

func TestClear(t *testing.T) {
	h := NewHistory()

	h.AddConversation("id1", "model", []Message{})
	h.AddConversation("id2", "model", []Message{})

	h.Clear()

	if len(h.Conversations) != 0 {
		t.Errorf("After Clear(), have %d conversations, want 0", len(h.Conversations))
	}
}

func TestSaveAndLoad(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "history_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testPath := filepath.Join(tmpDir, "test-history.json")

	// Create history with custom path
	h := &History{
		Conversations: make([]ConversationEntry, 0),
		path:          testPath,
	}

	// Add some conversations
	h.AddConversation("test-id", "sonar-pro", []Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi!"},
	})

	// Save
	if err := h.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(testPath); os.IsNotExist(err) {
		t.Error("History file was not created")
	}

	// Load into new history
	h2 := &History{
		Conversations: make([]ConversationEntry, 0),
		path:          testPath,
	}

	if err := h2.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if len(h2.Conversations) != 1 {
		t.Errorf("Loaded %d conversations, want 1", len(h2.Conversations))
	}

	conv := h2.Conversations[0]
	if conv.ID != "test-id" {
		t.Errorf("Loaded ID = %q, want %q", conv.ID, "test-id")
	}
	if len(conv.Messages) != 2 {
		t.Errorf("Loaded %d messages, want 2", len(conv.Messages))
	}
}

func TestLoadNonExistentFile(t *testing.T) {
	h := &History{
		Conversations: make([]ConversationEntry, 0),
		path:          "/non/existent/path/history.json",
	}

	// Should not error for non-existent file
	err := h.Load()
	if err != nil {
		t.Errorf("Load() should not error for non-existent file: %v", err)
	}
}

func TestMaxHistoryEntries(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "history_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testPath := filepath.Join(tmpDir, "test-history.json")

	h := &History{
		Conversations: make([]ConversationEntry, 0),
		path:          testPath,
	}

	// Add more than MaxHistoryEntries
	for i := 0; i < MaxHistoryEntries+10; i++ {
		h.Conversations = append(h.Conversations, ConversationEntry{
			ID:        string(rune(i)),
			Model:     "model",
			Messages:  []Message{},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		})
	}

	// Save should trim
	if err := h.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Load and verify
	h2 := &History{
		Conversations: make([]ConversationEntry, 0),
		path:          testPath,
	}
	if err := h2.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if len(h2.Conversations) > MaxHistoryEntries {
		t.Errorf("After save/load, have %d conversations, want <= %d", len(h2.Conversations), MaxHistoryEntries)
	}
}

func TestConversationTimestamps(t *testing.T) {
	h := NewHistory()

	before := time.Now()
	h.AddConversation("test", "model", []Message{})
	after := time.Now()

	conv := h.Conversations[0]

	if conv.CreatedAt.Before(before) || conv.CreatedAt.After(after) {
		t.Error("CreatedAt not set correctly")
	}
	if conv.UpdatedAt.Before(before) || conv.UpdatedAt.After(after) {
		t.Error("UpdatedAt not set correctly")
	}

	// Update and check UpdatedAt changes
	time.Sleep(10 * time.Millisecond)
	h.UpdateConversation("test", []Message{{Role: "user", Content: "new"}})

	convPtr := h.GetConversation("test")
	if !convPtr.UpdatedAt.After(conv.CreatedAt) {
		t.Error("UpdatedAt should be after CreatedAt after update")
	}
}
