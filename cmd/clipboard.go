package cmd

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// ClipboardError represents a clipboard operation error with helpful suggestions
type ClipboardError struct {
	OS      string
	Message string
	Hint    string
}

func (e *ClipboardError) Error() string {
	if e.Hint != "" {
		return fmt.Sprintf("%s. %s", e.Message, e.Hint)
	}
	return e.Message
}

// copyToClipboard copies text to the system clipboard
func copyToClipboard(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("pbcopy"); err != nil {
			return &ClipboardError{
				OS:      "macOS",
				Message: "pbcopy command not found",
				Hint:    "This is unexpected on macOS - pbcopy should be available by default",
			}
		}
		cmd = exec.Command("pbcopy")
	case "linux":
		// Try xclip first, then xsel, then wl-copy for Wayland
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		} else if _, err := exec.LookPath("wl-copy"); err == nil {
			// Support for Wayland
			cmd = exec.Command("wl-copy")
		} else {
			return &ClipboardError{
				OS:      "Linux",
				Message: "no clipboard tool found",
				Hint:    "Install one of: xclip (sudo apt install xclip), xsel (sudo apt install xsel), or wl-copy for Wayland (sudo apt install wl-clipboard)",
			}
		}
	case "windows":
		cmd = exec.Command("clip")
	default:
		return &ClipboardError{
			OS:      runtime.GOOS,
			Message: fmt.Sprintf("clipboard not supported on %s", runtime.GOOS),
		}
	}

	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err != nil {
		return &ClipboardError{
			OS:      runtime.GOOS,
			Message: fmt.Sprintf("failed to copy to clipboard: %v", err),
			Hint:    "Make sure the clipboard tool is working correctly",
		}
	}
	return nil
}
