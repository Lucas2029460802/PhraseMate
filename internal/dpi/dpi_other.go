//go:build !windows

package dpi

// Enable is a no-op on non-Windows platforms.
func Enable() {}
