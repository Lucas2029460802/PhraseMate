//go:build windows

package main

import (
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"phrasemate/internal/app"
	"phrasemate/internal/dpi"
	"phrasemate/internal/floatwin"
	"phrasemate/internal/shortcut"
	"phrasemate/internal/tray"
	"phrasemate/internal/winutil"

	"github.com/jchv/go-webview2"
)

func runDesktop(url string) {
	dpi.Enable()

	var (
		mainView  atomic.Pointer[webviewHolder]
		floatHost *floatwin.Host
		trayHost  *tray.Host
		mu        sync.Mutex
	)

	refreshMain := func() {
		if p := mainView.Load(); p != nil && p.w != nil {
			p.w.Dispatch(func() {
				p.w.Eval(`window.phrasemateRefresh && window.phrasemateRefresh();`)
			})
		}
	}

	showNotebook := func() {
		if p := mainView.Load(); p != nil && p.hwnd != 0 {
			winutil.Show(p.hwnd)
			refreshMain()
		}
	}

	hideNotebook := func() {
		if p := mainView.Load(); p != nil && p.hwnd != 0 {
			winutil.Hide(p.hwnd)
		}
	}

	toggleNotebook := func() {
		if p := mainView.Load(); p != nil && p.hwnd != 0 {
			if winutil.IsVisible(p.hwnd) {
				hideNotebook()
			} else {
				showNotebook()
			}
		}
	}

	showFloat := func() {
		if floatHost != nil {
			floatHost.Show()
		}
	}

	createShortcut := func() {
		path, err := shortcut.CreateDesktop()
		if err != nil {
			app.MessageBox("PhraseMate", err.Error())
			return
		}
		app.InfoBox("PhraseMate", "已创建桌面快捷方式：\n"+path)
	}

	exitApp := func() {
		mu.Lock()
		defer mu.Unlock()
		if floatHost != nil {
			floatHost.Close()
		}
		if p := mainView.Load(); p != nil && p.w != nil {
			// Force destroy (bypass hide-on-close) by restoring then destroying.
			p.restore()
			p.w.Destroy()
		}
		if trayHost != nil {
			trayHost.Close()
		}
		// Give windows a moment, then hard-exit process.
		go func() {
			time.Sleep(200 * time.Millisecond)
			os.Exit(0)
		}()
	}

	var err error
	floatHost, err = floatwin.Start(url, floatwin.Hooks{
		OnCaptured: func(term string) {
			log.Printf("已速记: %s", term)
			refreshMain()
		},
		OnToggleNotebook: toggleNotebook,
	})
	if err != nil {
		log.Printf("悬浮窗启动失败: %v", err)
	} else {
		log.Println("系统置顶速记窗已启动")
	}

	trayHost = tray.Start(showNotebook, showFloat, createShortcut, exitApp)

	if path, err := shortcut.EnsureDesktop(); err != nil {
		log.Printf("自动创建桌面快捷方式失败: %v", err)
	} else if path != "" {
		log.Printf("已自动创建桌面快捷方式: %s", path)
	}

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     envTruthy("PHRASEMATE_DEBUG"),
		AutoFocus: true,
		DataPath:  filepathJoinTemp("phrasemate-webview-main"),
		WindowOptions: webview2.WindowOptions{
			Title:  "PhraseMate",
			Width:  1180,
			Height: 760,
			Center: true,
		},
	})
	if w == nil {
		fatalPopup(
			"PhraseMate 无法创建窗口",
			"未能加载 Microsoft Edge WebView2。\n请安装 WebView2 Runtime 后重试：\nhttps://developer.microsoft.com/microsoft-edge/webview2/",
		)
	}

	hwnd := winutil.Hwnd(w.Window())
	restore := winutil.HideOnClose(hwnd)
	mainView.Store(&webviewHolder{w: w, hwnd: hwnd, restore: restore})

	_ = w.Bind("showFloatWindow", func() {
		showFloat()
	})
	_ = w.Bind("hideNotebookWindow", func() {
		hideNotebook()
	})

	w.SetSize(1180, 760, webview2.HintNone)
	w.Navigate(url)
	log.Println("生词本窗口已启动：关闭窗口会隐藏到后台，托盘可重新打开；速记窗保持运行")
	w.Run()

	// If main Run returns unexpectedly, keep process alive for float/tray until exit.
	log.Println("生词本消息循环结束，后台继续运行（托盘退出可关闭）")
	select {}
}

type webviewHolder struct {
	w       webview2.WebView
	hwnd    uintptr
	restore func()
}

func runWebOnly(url string) {
	log.Printf("浏览器模式，请打开: %s", url)
	log.Println("提示: 浏览器模式没有系统级置顶悬浮窗")
}

func filepathJoinTemp(name string) string {
	return filepath.Join(os.TempDir(), name)
}
