//go:build windows

package winutil

import (
	"syscall"
	"unsafe"
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	procShowWindow       = user32.NewProc("ShowWindow")
	procSetWindowPos     = user32.NewProc("SetWindowPos")
	procGetWindowLongPtr = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtr = user32.NewProc("SetWindowLongPtrW")
	procSetWindowLong    = user32.NewProc("SetWindowLongW")
	procGetWindowLong    = user32.NewProc("GetWindowLongW")
	procGetSystemMetrics = user32.NewProc("GetSystemMetrics")
	procReleaseCapture   = user32.NewProc("ReleaseCapture")
	procSendMessageW     = user32.NewProc("SendMessageW")
	procCallWindowProcW  = user32.NewProc("CallWindowProcW")
	procGetWindowRect    = user32.NewProc("GetWindowRect")
	procIsWindowVisible  = user32.NewProc("IsWindowVisible")
	procSetForeground    = user32.NewProc("SetForegroundWindow")
	procSetWindowRgn     = user32.NewProc("SetWindowRgn")
	procGetClientRect    = user32.NewProc("GetClientRect")
	procSystemParametersInfoW = user32.NewProc("SystemParametersInfoW")
	gdi32                = syscall.NewLazyDLL("gdi32.dll")
	procCreateRoundRectRgn = gdi32.NewProc("CreateRoundRectRgn")
)

func init() {
	if err := procGetWindowLongPtr.Find(); err != nil {
		procGetWindowLongPtr = user32.NewProc("GetWindowLongW")
	}
	if err := procSetWindowLongPtr.Find(); err != nil {
		procSetWindowLongPtr = user32.NewProc("SetWindowLongW")
	}
}

const (
	WSCaption       = 0x00C00000
	WSThickFrame    = 0x00040000
	WSMinimizeBox   = 0x00020000
	WSMaximizeBox   = 0x00010000
	WSSysMenu       = 0x00080000
	WSBorder        = 0x00800000
	WSPopup         = 0x80000000
	WSVisible       = 0x10000000
	WSClipChildren  = 0x02000000
	WSClipSiblings  = 0x04000000
	ExToolWindow    = 0x00000080
	ExAppWindow     = 0x00040000
	ExTopmost       = 0x00000008
	ExLayered       = 0x00080000
	SWHide          = 0
	SWShow          = 5
	SWRestore       = 9
	HWNDTopmost     = ^uintptr(0) // -1
	SWPNoMove       = 0x0002
	SWPNoSize       = 0x0001
	SWPNoZOrder     = 0x0004
	SWPFrameChanged = 0x0020
	SWPShowWindow   = 0x0040
	WMClose         = 0x0010
	WMNCLButtonDown = 0x00A1
	HTCaption       = 2
	spiGetWorkArea  = 0x0030
)

type Rect struct {
	Left, Top, Right, Bottom int32
}

func Hwnd(p unsafe.Pointer) uintptr {
	return uintptr(p)
}

func Show(hwnd uintptr) {
	_, _, _ = procShowWindow.Call(hwnd, uintptr(SWRestore))
	_, _, _ = procShowWindow.Call(hwnd, uintptr(SWShow))
	_, _, _ = procSetForeground.Call(hwnd)
}

func Hide(hwnd uintptr) {
	_, _, _ = procShowWindow.Call(hwnd, uintptr(SWHide))
}

func IsVisible(hwnd uintptr) bool {
	r, _, _ := procIsWindowVisible.Call(hwnd)
	return r != 0
}

func MakeFramelessTopmost(hwnd uintptr) {
	idxStyle := ^uintptr(15) // GWL_STYLE = -16
	idxEx := ^uintptr(19)    // GWL_EXSTYLE = -20
	style, _, _ := procGetWindowLong.Call(hwnd, idxStyle)
	style = style &^ uintptr(WSCaption|WSThickFrame|WSMinimizeBox|WSMaximizeBox|WSSysMenu|WSBorder)
	style |= uintptr(WSPopup | WSVisible | WSClipChildren | WSClipSiblings)
	_, _, _ = procSetWindowLong.Call(hwnd, idxStyle, style)

	ex, _, _ := procGetWindowLong.Call(hwnd, idxEx)
	ex |= uintptr(ExTopmost | ExToolWindow)
	ex &^= uintptr(ExAppWindow)
	_, _, _ = procSetWindowLong.Call(hwnd, idxEx, ex)

	_, _, _ = procSetWindowPos.Call(hwnd, HWNDTopmost, 0, 0, 0, 0, SWPNoMove|SWPNoSize|SWPFrameChanged|SWPShowWindow)
}

// SetRoundedRegion clips the window to a rounded rectangle (removes sharp outer box).
func SetRoundedRegion(hwnd uintptr, width, height, radius int) {
	if hwnd == 0 {
		return
	}
	if width <= 0 || height <= 0 {
		var rc Rect
		procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
		if rc.Right > 0 {
			width = int(rc.Right - rc.Left)
			height = int(rc.Bottom - rc.Top)
		}
	}
	if width <= 0 || height <= 0 {
		return
	}
	rgn, _, _ := procCreateRoundRectRgn.Call(0, 0, uintptr(width+1), uintptr(height+1), uintptr(radius), uintptr(radius))
	if rgn == 0 {
		return
	}
	_, _, _ = procSetWindowRgn.Call(hwnd, rgn, 1)
}

func workArea() Rect {
	var rc Rect
	_, _, _ = procSystemParametersInfoW.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&rc)), 0)
	if rc.Right > rc.Left && rc.Bottom > rc.Top {
		return rc
	}
	sw, _, _ := procGetSystemMetrics.Call(0)
	sh, _, _ := procGetSystemMetrics.Call(1)
	return Rect{Left: 0, Top: 0, Right: int32(sw), Bottom: int32(sh)}
}

func PlaceBottomRight(hwnd uintptr, width, height int) {
	if hwnd == 0 {
		return
	}
	wa := workArea()
	w := int(wa.Right - wa.Left)
	h := int(wa.Bottom - wa.Top)
	marginX := 16
	marginY := 16
	x := int(wa.Left) + w - width - marginX
	y := int(wa.Top) + h - height - marginY
	if x < int(wa.Left)+8 {
		x = int(wa.Left) + 8
	}
	if y < int(wa.Top)+8 {
		y = int(wa.Top) + 8
	}
	_, _, _ = procSetWindowPos.Call(
		hwnd, HWNDTopmost,
		uintptr(x), uintptr(y),
		uintptr(width), uintptr(height),
		SWPShowWindow,
	)
	SetRoundedRegion(hwnd, 0, 0, 18)
}

func EnsureTopmost(hwnd uintptr) {
	_, _, _ = procSetWindowPos.Call(hwnd, HWNDTopmost, 0, 0, 0, 0, SWPNoMove|SWPNoSize)
}

func StartDrag(hwnd uintptr) {
	_, _, _ = procReleaseCapture.Call()
	_, _, _ = procSendMessageW.Call(hwnd, WMNCLButtonDown, HTCaption, 0)
}

// HideOnClose subclasses the window so WM_CLOSE hides instead of destroying.
func HideOnClose(hwnd uintptr) (restore func()) {
	idx := ^uintptr(3) // GWLP_WNDPROC = -4
	old, _, _ := procGetWindowLongPtr.Call(hwnd, idx)
	cb := syscall.NewCallback(func(h uintptr, msg uint32, wp, lp uintptr) uintptr {
		if msg == WMClose {
			Hide(h)
			return 0
		}
		r, _, _ := procCallWindowProcW.Call(old, h, uintptr(msg), wp, lp)
		return r
	})
	_, _, _ = procSetWindowLongPtr.Call(hwnd, idx, cb)
	return func() {
		_, _, _ = procSetWindowLongPtr.Call(hwnd, idx, old)
	}
}
