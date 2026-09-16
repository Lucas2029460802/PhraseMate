//go:build darwin

package app

import (
	"os"
	"os/exec"
	"strings"
)

// MessageBox shows a native macOS error dialog.
func MessageBox(title, text string) {
	showDialog(title, text, "stop")
}

// InfoBox shows a native macOS informational dialog.
func InfoBox(title, text string) {
	showDialog(title, text, "note")
}

func showDialog(title, text, icon string) {
	if strings.TrimSpace(os.Getenv("CI")) != "" {
		return
	}
	script := "display dialog " + appleString(text) +
		" with title " + appleString(title) +
		` buttons {"OK"} default button 1 with icon ` + icon
	_ = exec.Command("osascript", "-e", script).Run()
}

func appleString(s string) string {
	s = strings.ReplaceAll(s, `"`, `""`)
	return `"` + s + `"`
}
