//go:build windows

package app

import (
	"syscall"
	"unsafe"
)

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	procMessageBoxW  = user32.NewProc("MessageBoxW")
)

// MessageBox shows a native Windows message dialog (error style).
func MessageBox(title, text string) {
	showMessageBox(title, text, 0x10) // MB_ICONERROR
}

// InfoBox shows an informational message dialog.
func InfoBox(title, text string) {
	showMessageBox(title, text, 0x40) // MB_ICONINFORMATION
}

func showMessageBox(title, text string, flags uintptr) {
	t, _ := syscall.UTF16PtrFromString(title)
	m, _ := syscall.UTF16PtrFromString(text)
	procMessageBoxW.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), flags)
}
