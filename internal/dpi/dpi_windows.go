//go:build windows

package dpi

import "syscall"

var (
	user32                           = syscall.NewLazyDLL("user32.dll")
	procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	procSetProcessDPIAware            = user32.NewProc("SetProcessDPIAware")
)

// Enable turns on Per-Monitor V2 DPI awareness before any window is created.
// This prevents WebView2 / Win32 UI from looking blurry on high-DPI screens.
func Enable() {
	// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 == (HANDLE)-4
	r, _, _ := procSetProcessDpiAwarenessContext.Call(^uintptr(3))
	if r != 0 {
		return
	}
	_, _, _ = procSetProcessDPIAware.Call()
}
