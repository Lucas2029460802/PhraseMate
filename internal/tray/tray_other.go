//go:build !windows

package tray

// Host is a stub.
type Host struct{}

// Start is unavailable outside Windows.
func Start(onOpen, onFloat, onExit func()) *Host { return &Host{} }

// Close is a no-op.
func (h *Host) Close() {}
