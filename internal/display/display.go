package display

import "fmt"

// Color codes for terminal output
var colors = map[string]string{
	"red":    "91m",
	"green":  "92m",
	"yellow": "93m",
	"blue":   "94m",
	"white":  "97m",
}

// Background color codes
var bgColors = map[string]string{
	"black":  "40",
	"red":    "41",
	"green":  "42",
	"yellow": "43",
	"blue":   "44",
	"white":  "47",
}

// Display prints a colored message to the terminal
func Display(message, color string, bold bool, bgColor string) {
	c, ok := colors[color]
	if !ok {
		c = colors["white"]
	}
	bg, ok := bgColors[bgColor]
	if !ok {
		bg = bgColors["black"]
	}

	if bold {
		fmt.Printf("\033[1;%s;%s %s\033[0m", bg, c, message)
	} else {
		fmt.Printf("\033[%s;%s %s\033[0m", bg, c, message)
	}
}

// ShowUsage displays token usage statistics
func ShowUsage(usage map[string]int, useGlow bool) {
	if useGlow {
		fmt.Println("# Tokens")
	} else {
		Display("Tokens \n", "yellow", true, "blue")
	}
	for key, value := range usage {
		fmt.Printf("- %s: %d\n", key, value)
	}
	fmt.Println()
}

// ShowCitations displays the citations list
func ShowCitations(citations []string, useGlow bool) {
	if useGlow {
		fmt.Println("# Citations")
	} else {
		Display("Citations \n", "yellow", true, "blue")
	}
	for _, citation := range citations {
		fmt.Printf("- %s\n", citation)
	}
	fmt.Println()
}

// ShowContent displays the main content response
func ShowContent(content string, useGlow bool) {
	if useGlow {
		fmt.Println("# Content")
	} else {
		Display("Content \n", "yellow", true, "blue")
	}
	fmt.Println(content)
}

// ShowError displays an error message in red
func ShowError(message string) {
	Display(message, "red", false, "black")
	fmt.Println()
}
