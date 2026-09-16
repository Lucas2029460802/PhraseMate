//go:build darwin

package tray

import (
	"phrasemate/internal/brandicon"
	"phrasemate/internal/macui"
)

// Host owns a macOS menu-bar status item.
type Host struct{}

// Start creates a tray icon. AppKit must already be initialized.
func Start(onOpen, onFloat, onShortcut, onExit func()) *Host {
	img := brandicon.Render(44)
	macui.StartTray(img.Pix, img.Bounds().Dx(), img.Bounds().Dy(), macui.TrayCallbacks{
		OnOpen:     onOpen,
		OnFloat:    onFloat,
		OnShortcut: onShortcut,
		OnExit:     onExit,
	})
	return &Host{}
}

// Close removes the tray icon.
func (h *Host) Close() {
	if h == nil {
		return
	}
	macui.StopTray()
}
