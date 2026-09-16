//go:build darwin

package floatwin

import (
	"fmt"
	"sync/atomic"

	"phrasemate/internal/macui"
)

// Host is a frameless topmost WKWebView capture bar.
type Host struct {
	view   *macui.Window
	closed atomic.Bool
}

// Start opens the float window on the AppKit main thread (no extra run loop).
func Start(baseURL string, hooks Hooks) (*Host, error) {
	flags := macui.WindowFloat
	w := macui.NewWindow("PhraseMate 速记", floatW, floatH, flags)
	if w == nil {
		return nil, fmt.Errorf("无法创建速记窗 WKWebView")
	}
	h := &Host{view: w}
	w.Bind("notifyCaptured", func(args []string) {
		term := ""
		if len(args) > 0 {
			term = args[0]
		}
		if hooks.OnCaptured != nil {
			hooks.OnCaptured(term)
		}
	})
	w.Bind("toggleNotebook", func(_ []string) {
		if hooks.OnToggleNotebook != nil {
			hooks.OnToggleNotebook()
		}
	})
	w.Bind("startWindowDrag", func(_ []string) {
		// Dragging is handled natively (movableByWindowBackground / CSS app-region).
	})
	w.Navigate(baseURL + "/float.html")
	w.PlaceBottomRight(floatW, floatH)
	w.Show()
	return h, nil
}

// Show restores the float window.
func (h *Host) Show() {
	if h == nil || h.view == nil || h.closed.Load() {
		return
	}
	h.view.Show()
	h.view.EnsureTopmost()
	h.view.PlaceBottomRight(floatW, floatH)
}

// Close destroys the float window.
func (h *Host) Close() {
	if h == nil || h.view == nil {
		return
	}
	h.closed.Store(true)
	h.view.Destroy()
}
