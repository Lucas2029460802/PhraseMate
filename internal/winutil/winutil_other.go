//go:build !windows

package winutil

import "unsafe"

func Hwnd(p unsafe.Pointer) uintptr { return 0 }
func Show(hwnd uintptr)             {}
func Hide(hwnd uintptr)             {}
func IsVisible(hwnd uintptr) bool   { return false }
func MakeFramelessTopmost(hwnd uintptr) {}
func PlaceBottomRight(hwnd uintptr, width, height int) {}
func SetRoundedRegion(hwnd uintptr, width, height, radius int) {}
func EnsureTopmost(hwnd uintptr) {}
func StartDrag(hwnd uintptr)     {}
func HideOnClose(hwnd uintptr) (restore func()) {
	return func() {}
}
