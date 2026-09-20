//go:build darwin

package main

import (
	"log"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"phrasemate/internal/app"
	"phrasemate/internal/brandicon"
	"phrasemate/internal/floatwin"
	"phrasemate/internal/macui"
	"phrasemate/internal/shortcut"
	"phrasemate/internal/tray"
)

func runDesktop(url string) {
	runtime.LockOSThread()
	macui.Init()
	if img := brandicon.Render(256); img != nil {
		macui.SetDockIcon(img.Pix, img.Bounds().Dx(), img.Bounds().Dy())
	}

	var (
		mainView  atomic.Pointer[macWindow]
		floatHost *floatwin.Host
		trayHost  *tray.Host
		mu        sync.Mutex
	)

	refreshMain := func() {
		if p := mainView.Load(); p != nil && p.w != nil {
			p.w.Eval(`window.phrasemateRefresh && window.phrasemateRefresh();`)
		}
	}

	showNotebook := func() {
		if p := mainView.Load(); p != nil && p.w != nil {
			p.w.Show()
			refreshMain()
		}
	}

	hideNotebook := func() {
		if p := mainView.Load(); p != nil && p.w != nil {
			p.w.Hide()
		}
	}

	toggleNotebook := func() {
		if p := mainView.Load(); p != nil && p.w != nil {
			if p.w.IsVisible() {
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
			p.w.Destroy()
		}
		if trayHost != nil {
			trayHost.Close()
		}
		go func() {
			time.Sleep(150 * time.Millisecond)
			macui.Quit()
			time.Sleep(150 * time.Millisecond)
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

	flags := macui.WindowHideOnClose | macui.WindowCenter
	if envTruthy("PHRASEMATE_DEBUG") {
		flags |= macui.WindowDebug
	}
	w := macui.NewWindow("PhraseMate", 1180, 760, flags)
	if w == nil {
		fatalPopup("PhraseMate 无法创建窗口", "未能创建 macOS WKWebView 窗口。请确认已安装 Xcode Command Line Tools。")
	}
	mainView.Store(&macWindow{w: w})

	w.Bind("showFloatWindow", func(_ []string) {
		showFloat()
	})
	w.Bind("hideNotebookWindow", func(_ []string) {
		hideNotebook()
	})
	w.Navigate(url)
	w.Show()
	log.Println("生词本窗口已启动：关闭窗口会隐藏到菜单栏，可重新打开；速记窗保持运行")
	macui.Run()
	log.Println("生词本消息循环结束")
}

type macWindow struct {
	w *macui.Window
}

func runWebOnly(url string) {
	log.Printf("浏览器模式，请打开: %s", url)
	log.Println("提示: 浏览器模式没有系统级置顶悬浮窗")
}
