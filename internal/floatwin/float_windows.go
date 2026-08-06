//go:build windows

package floatwin

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"

	"phrasemate/internal/winutil"

	"github.com/jchv/go-webview2"
)

const (
	floatW = 420
	floatH = 96
)

// Hooks connects float UI actions to the desktop shell.
type Hooks struct {
	OnCaptured       func(term string)
	OnToggleNotebook func()
}

// Host is a frameless topmost WebView2 capture bar.
type Host struct {
	view   webview2.WebView
	hwnd   uintptr
	quit   chan struct{}
	closed atomic.Bool
}

// Start opens the float window on its own UI thread.
func Start(baseURL string, hooks Hooks) (*Host, error) {
	h := &Host{quit: make(chan struct{})}
	ready := make(chan error, 1)

	go func() {
		runtime.LockOSThread()
		w := webview2.NewWithOptions(webview2.WebViewOptions{
			Debug:     false,
			AutoFocus: true,
			DataPath:  filepath.Join(os.TempDir(), "phrasemate-webview-float"),
			WindowOptions: webview2.WindowOptions{
				Title:  "PhraseMate 速记",
				Width:  floatW,
				Height: floatH,
				Center: false,
			},
		})
		if w == nil {
			ready <- fmt.Errorf("无法创建速记窗 WebView2")
			close(h.quit)
			return
		}
		h.view = w
		h.hwnd = winutil.Hwnd(w.Window())
		w.SetSize(floatW, floatH, webview2.HintFixed)
		winutil.MakeFramelessTopmost(h.hwnd)
		winutil.PlaceBottomRight(h.hwnd, floatW, floatH)

		_ = w.Bind("notifyCaptured", func(term string) {
			if hooks.OnCaptured != nil {
				hooks.OnCaptured(term)
			}
		})
		_ = w.Bind("toggleNotebook", func() {
			if hooks.OnToggleNotebook != nil {
				hooks.OnToggleNotebook()
			}
		})
		_ = w.Bind("startWindowDrag", func() {
			winutil.StartDrag(h.hwnd)
		})

		w.Navigate(baseURL + "/float.html")
		ready <- nil
		w.Run()
		h.closed.Store(true)
		close(h.quit)
	}()

	if err := <-ready; err != nil {
		return nil, err
	}
	return h, nil
}

// Show restores the float window.
func (h *Host) Show() {
	if h == nil || h.hwnd == 0 || h.closed.Load() {
		return
	}
	winutil.Show(h.hwnd)
	winutil.EnsureTopmost(h.hwnd)
	winutil.PlaceBottomRight(h.hwnd, floatW, floatH)
}

// Close destroys the float window.
func (h *Host) Close() {
	if h == nil || h.view == nil {
		return
	}
	h.view.Destroy()
	select {
	case <-h.quit:
	case <-time.After(2 * time.Second):
	}
}
