//go:build !windows

package floatwin

import "fmt"

// Host is a stub.
type Host struct{}

// Start is unavailable outside Windows.
func Start(baseURL string, hooks Hooks) (*Host, error) {
	return nil, fmt.Errorf("系统级悬浮窗仅支持 Windows")
}

// Show is a no-op.
func (h *Host) Show() {}

// Close is a no-op.
func (h *Host) Close() {}
