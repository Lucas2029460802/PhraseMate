//go:build windows

package tray

import (
	"runtime"
	"syscall"
	"unsafe"
)

var (
	shell32              = syscall.NewLazyDLL("shell32.dll")
	user32               = syscall.NewLazyDLL("user32.dll")
	procShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")
	procLoadIconW        = user32.NewProc("LoadIconW")
	procCreatePopupMenu  = user32.NewProc("CreatePopupMenu")
	procAppendMenuW      = user32.NewProc("AppendMenuW")
	procTrackPopupMenu   = user32.NewProc("TrackPopupMenu")
	procDestroyMenu      = user32.NewProc("DestroyMenu")
	procGetCursorPos     = user32.NewProc("GetCursorPos")
	procSetForeground    = user32.NewProc("SetForegroundWindow")
	procPostMessageW     = user32.NewProc("PostMessageW")
	procDefWindowProcW   = user32.NewProc("DefWindowProcW")
	procRegisterClassExW = user32.NewProc("RegisterClassExW")
	procCreateWindowExW  = user32.NewProc("CreateWindowExW")
	procGetMessageW      = user32.NewProc("GetMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procDispatchMessageW = user32.NewProc("DispatchMessageW")
	procPostQuitMessage  = user32.NewProc("PostQuitMessage")
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
)

const (
	nimAdd    = 0x00000000
	nimDelete = 0x00000002
	nifMessage = 0x00000001
	nifIcon    = 0x00000002
	nifTip     = 0x00000004
	wmApp      = 0x8000
	wmTray     = wmApp + 40
	wmLButtonUp = 0x0202
	wmRButtonUp = 0x0205
	idOpen     = 1001
	idShowFloat = 1002
	idExit     = 1003
	mfString   = 0x0000
	mfSeparator = 0x0800
	tpmRightButton = 0x0002
)

type notifyIconData struct {
	Size     uint32
	Wnd      uintptr
	ID       uint32
	Flags    uint32
	Callback uint32
	Icon     uintptr
	Tip      [128]uint16
}

type point struct{ X, Y int32 }

type msg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

// Host owns a system tray icon.
type Host struct {
	hwnd      uintptr
	nid       notifyIconData
	onOpen    func()
	onFloat   func()
	onExit    func()
	quit      chan struct{}
}

var active *Host

// Start creates a tray icon on a dedicated UI thread.
func Start(onOpen, onFloat, onExit func()) *Host {
	h := &Host{
		onOpen:  onOpen,
		onFloat: onFloat,
		onExit:  onExit,
		quit:    make(chan struct{}),
	}
	ready := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		active = h
		h.loop(ready)
	}()
	<-ready
	return h
}

// Close removes the tray icon.
func (h *Host) Close() {
	if h == nil {
		return
	}
	_, _, _ = procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&h.nid)))
	_, _, _ = procPostMessageW.Call(h.hwnd, 0x0010, 0, 0) // WM_CLOSE
	select {
	case <-h.quit:
	default:
	}
}

func (h *Host) loop(ready chan struct{}) {
	instance, _, _ := procGetModuleHandleW.Call(0)
	cls, _ := syscall.UTF16PtrFromString("PhraseMateTray")
	wc := wndClassEx{
		Size:      uint32(unsafe.Sizeof(wndClassEx{})),
		WndProc:   syscall.NewCallback(trayWndProc),
		Instance:  instance,
		ClassName: cls,
	}
	_, _, _ = procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	hwnd, _, _ := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), 0, 0, 0, 0, 0, 0, 0, 0, instance, 0)
	h.hwnd = hwnd

	icon := createTrayIcon()
	h.nid = notifyIconData{
		Size:     uint32(unsafe.Sizeof(notifyIconData{})),
		Wnd:      hwnd,
		ID:       1,
		Flags:    nifMessage | nifIcon | nifTip,
		Callback: wmTray,
		Icon:     icon,
	}
	tip, _ := syscall.UTF16FromString("PhraseMate · 后台运行中")
	copy(h.nid.Tip[:], tip)
	_, _, _ = procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&h.nid)))
	close(ready)

	var m msg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		_, _, _ = procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		_, _, _ = procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
	_, _, _ = procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&h.nid)))
	close(h.quit)
}

func trayWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	h := active
	if h == nil {
		r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
		return r
	}
	switch msg {
	case wmTray:
		switch lParam {
		case wmLButtonUp:
			if h.onOpen != nil {
				h.onOpen()
			}
		case wmRButtonUp:
			h.showMenu()
		}
		return 0
	case 0x0111: // WM_COMMAND
		switch wParam & 0xffff {
		case idOpen:
			if h.onOpen != nil {
				h.onOpen()
			}
		case idShowFloat:
			if h.onFloat != nil {
				h.onFloat()
			}
		case idExit:
			if h.onExit != nil {
				h.onExit()
			}
		}
		return 0
	case 0x0010: // WM_CLOSE
		_, _, _ = procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

func (h *Host) showMenu() {
	menu, _, _ := procCreatePopupMenu.Call()
	appendMenu(menu, idOpen, "打开生词本")
	appendMenu(menu, idShowFloat, "显示速记窗")
	_, _, _ = procAppendMenuW.Call(menu, mfSeparator, 0, 0)
	appendMenu(menu, idExit, "退出 PhraseMate")
	var pt point
	_, _, _ = procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	_, _, _ = procSetForeground.Call(h.hwnd)
	_, _, _ = procTrackPopupMenu.Call(menu, tpmRightButton, uintptr(pt.X), uintptr(pt.Y), 0, h.hwnd, 0)
	_, _, _ = procDestroyMenu.Call(menu)
}

func appendMenu(menu uintptr, id uintptr, text string) {
	p, _ := syscall.UTF16PtrFromString(text)
	_, _, _ = procAppendMenuW.Call(menu, mfString, id, uintptr(unsafe.Pointer(p)))
}
