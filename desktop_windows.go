//go:build windows

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
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
		mainView        atomic.Pointer[webviewHolder]
		writingView     atomic.Pointer[webviewHolder]
		writingStartMu  sync.Mutex
		writingStarting bool
		readerSeq       atomic.Int64
		readers         sync.Map // seq int64 -> *webviewHolder
		floatHost       *floatwin.Host
		trayHost        *tray.Host
		mu              sync.Mutex
	)

	refreshMain := func() {
		if p := mainView.Load(); p != nil && p.w != nil {
			p.w.Dispatch(func() {
				p.w.Eval(`window.phrasemateRefresh && window.phrasemateRefresh();`)
			})
		}
	}

	refreshWriting := func() {
		if p := writingView.Load(); p != nil && p.w != nil {
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

	var openReader func(id float64)
	openReader = func(id float64) {
		wid := int64(id)
		if wid <= 0 {
			return
		}
		seq := readerSeq.Add(1)
		go func() {
			runtime.LockOSThread()
			w := webview2.NewWithOptions(webview2.WebViewOptions{
				Debug:     envTruthy("PHRASEMATE_DEBUG"),
				AutoFocus: true,
				DataPath:  filepathJoinTemp(fmt.Sprintf("phrasemate-webview-reader-%d-%d", wid, seq)),
				WindowOptions: webview2.WindowOptions{
					Title:  "PhraseMate 范文阅读",
					Width:  960,
					Height: 820,
					Center: true,
				},
			})
			if w == nil {
				log.Printf("无法创建范文阅读窗口 id=%d", wid)
				return
			}
			hwnd := winutil.Hwnd(w.Window())
			holder := &webviewHolder{w: w, hwnd: hwnd, restore: func() {}}
			readers.Store(seq, holder)
			w.SetSize(960, 820, webview2.HintNone)
			w.Navigate(fmt.Sprintf("%s/?page=reader&id=%d", url, wid))
			log.Printf("范文阅读窗口已打开 id=%d", wid)
			w.Run()
			readers.Delete(seq)
		}()
	}

	showWriting := func() {
		if p := writingView.Load(); p != nil && p.hwnd != 0 {
			winutil.Show(p.hwnd)
			refreshWriting()
			return
		}
		writingStartMu.Lock()
		if writingStarting {
			writingStartMu.Unlock()
			return
		}
		writingStarting = true
		writingStartMu.Unlock()

		go func() {
			runtime.LockOSThread()
			defer func() {
				writingStartMu.Lock()
				writingStarting = false
				writingStartMu.Unlock()
			}()

			w := webview2.NewWithOptions(webview2.WebViewOptions{
				Debug:     envTruthy("PHRASEMATE_DEBUG"),
				AutoFocus: true,
				DataPath:  filepathJoinTemp("phrasemate-webview-writing"),
				WindowOptions: webview2.WindowOptions{
					Title:  "PhraseMate 写作本",
					Width:  1180,
					Height: 760,
					Center: true,
				},
			})
			if w == nil {
				log.Println("无法创建写作本窗口（WebView2）")
				return
			}
			hwnd := winutil.Hwnd(w.Window())
			restore := winutil.HideOnClose(hwnd)
			holder := &webviewHolder{w: w, hwnd: hwnd, restore: restore}
			writingView.Store(holder)

			_ = w.Bind("showFloatWindow", func() { showFloat() })
			_ = w.Bind("showNotebookWindow", func() { showNotebook() })
			_ = w.Bind("hideNotebookWindow", func() { hideNotebook() })
			_ = w.Bind("openReaderWindow", func(id float64) { openReader(id) })

			w.SetSize(1180, 760, webview2.HintNone)
			w.Navigate(url + "/?page=writing")
			log.Println("写作本窗口已启动：可与生词本同时显示")
			w.Run()
			writingView.Store(nil)
		}()
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
		readers.Range(func(_, v any) bool {
			if p, ok := v.(*webviewHolder); ok && p != nil && p.w != nil {
				p.w.Destroy()
			}
			return true
		})
		if p := writingView.Load(); p != nil && p.w != nil {
			p.restore()
			p.w.Destroy()
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
	_ = w.Bind("openWritingWindow", func() {
		showWriting()
	})
	_ = w.Bind("showNotebookWindow", func() {
		showNotebook()
	})
	_ = w.Bind("openReaderWindow", func(id float64) {
		openReader(id)
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
