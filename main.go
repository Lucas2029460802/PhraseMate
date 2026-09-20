package main

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"phrasemate/internal/app"
	"phrasemate/internal/config"
	"phrasemate/internal/dpi"
	"phrasemate/internal/shortcut"
)

func init() {
	// AppKit / some Win32 UI must run on the thread that started the process.
	runtime.LockOSThread()
}

func main() {
	dpi.Enable()

	// Prefer .env next to the executable when running as a packaged app.
	loadDotEnv(findEnvFile())

	if len(os.Args) > 1 && (os.Args[1] == "--install-shortcut" || os.Args[1] == "-install-shortcut") {
		installDesktopShortcut()
		return
	}
	if len(os.Args) > 1 && (os.Args[1] == "--pack-app" || os.Args[1] == "-pack-app") {
		packMacApp()
		return
	}

	cfg := config.Load()
	webOnly := envTruthy("PHRASEMATE_WEB")

	listen := ""
	if webOnly {
		listen = cfg.Addr
		if listen == "" {
			listen = ":8080"
		}
	}

	rt, err := app.Start(cfg, listen)
	if err != nil {
		fatalPopup("PhraseMate 启动失败", err.Error())
	}
	defer rt.Close()

	if !rt.AI.Enabled() {
		log.Println("提示: 未检测到 API Key，请在应用内打开「设置」填写")
	} else {
		log.Printf("模型: %s  |  BaseURL: %s", rt.AI.Model(), rt.AI.BaseURL())
	}
	log.Printf("本地服务: %s", rt.URL)

	if webOnly {
		runWebOnly(rt.URL)
		waitForShutdown()
		return
	}
	runDesktop(rt.URL)
}

func waitForShutdown() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	sig := <-ch
	log.Printf("收到退出信号 (%s)，正在关闭", sig)
}

func findEnvFile() string {
	candidates := []string{".env"}
	if exe, err := os.Executable(); err == nil {
		candidates = append([]string{filepath.Join(filepath.Dir(exe), ".env")}, candidates...)
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(wd, ".env"))
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ".env"
}

func loadDotEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
	abs, _ := filepath.Abs(path)
	log.Printf("已加载环境文件: %s", abs)
}

func envTruthy(key string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func fatalPopup(title, msg string) {
	log.Println(title + ": " + msg)
	app.MessageBox(title, msg)
	os.Exit(1)
}

func installDesktopShortcut() {
	path, err := shortcut.CreateDesktop()
	if err != nil {
		fatalPopup("创建桌面快捷方式失败", err.Error())
	}
	log.Printf("已创建桌面快捷方式: %s", path)
	app.InfoBox("PhraseMate", "已创建桌面快捷方式：\n"+path)
}

func packMacApp() {
	dest := "PhraseMate.app"
	if len(os.Args) > 2 && strings.TrimSpace(os.Args[2]) != "" {
		dest = os.Args[2]
	}
	path, err := shortcut.PackApp(dest)
	if err != nil {
		log.Println("打包失败:", err)
		os.Exit(1)
	}
	log.Printf("已打包应用: %s", path)
}
