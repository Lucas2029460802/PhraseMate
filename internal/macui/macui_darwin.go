//go:build darwin

package macui

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -Wno-deprecated-declarations
#cgo LDFLAGS: -framework Cocoa -framework WebKit -framework Foundation -framework QuartzCore
#include "native_darwin.h"
#include <stdlib.h>
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"unsafe"
)

const (
	WindowFloat       = 1 << 0
	WindowHideOnClose = 1 << 1
	WindowCenter      = 1 << 2
	WindowDebug       = 1 << 3
)

var (
	winMu   sync.Mutex
	windows = map[int]*Window{}
	trayMu  sync.Mutex
	trayCB  TrayCallbacks
)

// Window is a native WKWebView-backed NSWindow.
type Window struct {
	id    int
	binds map[string]func([]string)
}

// TrayCallbacks are menu-bar actions.
type TrayCallbacks struct {
	OnOpen     func()
	OnFloat    func()
	OnShortcut func()
	OnExit     func()
}

// Init prepares NSApplication. Call from the main thread.
func Init() {
	C.PMInit()
}

// Run starts the AppKit event loop (blocking).
func Run() {
	C.PMRun()
}

// Quit stops the AppKit event loop.
func Quit() {
	C.PMQuit()
}

// SetDockIcon sets the Dock icon from non-premultiplied RGBA pixels.
func SetDockIcon(pix []byte, width, height int) {
	if width <= 0 || height <= 0 || len(pix) < width*height*4 {
		return
	}
	C.PMSetAppIconRGBA((*C.uint8_t)(unsafe.Pointer(&pix[0])), C.int(width), C.int(height))
}

// NewWindow creates a native window. Bind handlers before Navigate.
func NewWindow(title string, width, height int, flags int) *Window {
	ctitle := C.CString(title)
	defer C.free(unsafe.Pointer(ctitle))
	id := int(C.PMWindowCreate(ctitle, C.int(width), C.int(height), C.int(flags)))
	if id == 0 {
		return nil
	}
	w := &Window{id: id, binds: map[string]func([]string){}}
	winMu.Lock()
	windows[id] = w
	winMu.Unlock()
	return w
}

// Bind exposes a JavaScript function on window.<name>.
func (w *Window) Bind(name string, fn func([]string)) {
	if w == nil || name == "" || fn == nil {
		return
	}
	w.binds[name] = fn
	w.syncBinds()
}

func (w *Window) syncBinds() {
	names := make([]string, 0, len(w.binds))
	for name := range w.binds {
		names = append(names, name)
	}
	var b strings.Builder
	b.WriteString("(function(){")
	for _, name := range names {
		esc := strings.ReplaceAll(name, `\`, `\\`)
		esc = strings.ReplaceAll(esc, `"`, `\"`)
		b.WriteString(`window["` + esc + `"]=function(){return window.__pmCall("` + esc + `",arguments);};`)
	}
	b.WriteString("})();")
	cjs := C.CString(b.String())
	defer C.free(unsafe.Pointer(cjs))
	C.PMWindowSetBindScript(C.int(w.id), cjs)
}

// Navigate loads a URL.
func (w *Window) Navigate(url string) {
	if w == nil {
		return
	}
	cu := C.CString(url)
	defer C.free(unsafe.Pointer(cu))
	C.PMWindowNavigate(C.int(w.id), cu)
}

// Eval runs JavaScript in the page.
func (w *Window) Eval(js string) {
	if w == nil {
		return
	}
	cjs := C.CString(js)
	defer C.free(unsafe.Pointer(cjs))
	C.PMWindowEval(C.int(w.id), cjs)
}

func (w *Window) Show() {
	if w != nil {
		C.PMWindowShow(C.int(w.id))
	}
}

func (w *Window) Hide() {
	if w != nil {
		C.PMWindowHide(C.int(w.id))
	}
}

func (w *Window) IsVisible() bool {
	if w == nil {
		return false
	}
	return bool(C.PMWindowIsVisible(C.int(w.id)))
}

func (w *Window) PlaceBottomRight(width, height int) {
	if w != nil {
		C.PMWindowPlaceBottomRight(C.int(w.id), C.int(width), C.int(height))
	}
}

func (w *Window) EnsureTopmost() {
	if w != nil {
		C.PMWindowEnsureTopmost(C.int(w.id))
	}
}

func (w *Window) Destroy() {
	if w == nil {
		return
	}
	C.PMWindowDestroy(C.int(w.id))
	winMu.Lock()
	delete(windows, w.id)
	winMu.Unlock()
}

// StartTray creates a menu-bar status item.
func StartTray(pix []byte, width, height int, cb TrayCallbacks) {
	trayMu.Lock()
	trayCB = cb
	trayMu.Unlock()
	if width <= 0 || height <= 0 || len(pix) < width*height*4 {
		C.PMTrayStart(nil, 0, 0)
		return
	}
	C.PMTrayStart((*C.uint8_t)(unsafe.Pointer(&pix[0])), C.int(width), C.int(height))
}

// StopTray removes the menu-bar status item.
func StopTray() {
	C.PMTrayStop()
}

//export goPMMessage
func goPMMessage(id C.int, name *C.char, argsJSON *C.char) {
	n := C.GoString(name)
	raw := C.GoString(argsJSON)
	winMu.Lock()
	w := windows[int(id)]
	winMu.Unlock()
	if w == nil {
		return
	}
	fn := w.binds[n]
	if fn == nil {
		return
	}
	var rawArgs []any
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &rawArgs)
	}
	args := make([]string, 0, len(rawArgs))
	for _, v := range rawArgs {
		if v == nil {
			args = append(args, "")
			continue
		}
		args = append(args, fmt.Sprint(v))
	}
	fn(args)
}

//export goPMTray
func goPMTray(action C.int) {
	trayMu.Lock()
	cb := trayCB
	trayMu.Unlock()
	switch int(action) {
	case 1:
		if cb.OnOpen != nil {
			cb.OnOpen()
		}
	case 2:
		if cb.OnFloat != nil {
			cb.OnFloat()
		}
	case 3:
		if cb.OnShortcut != nil {
			cb.OnShortcut()
		}
	case 4:
		if cb.OnExit != nil {
			cb.OnExit()
		}
	}
}
