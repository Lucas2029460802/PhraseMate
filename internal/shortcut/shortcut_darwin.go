//go:build darwin

package shortcut

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"phrasemate/internal/brandicon"
)

// CreateDesktop installs PhraseMate.app (if needed) and places a Desktop alias.
func CreateDesktop() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("定位程序失败: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return "", err
	}
	if IsEphemeral(exe) {
		return "", fmt.Errorf("当前是 go run 临时程序，请先用 go build 打包成 PhraseMate 再创建快捷方式")
	}

	appPath, err := ensureAppBundle(exe)
	if err != nil {
		return "", err
	}

	desktop, err := desktopDir()
	if err != nil {
		return "", err
	}
	aliasPath := filepath.Join(desktop, "PhraseMate")
	_ = os.RemoveAll(aliasPath)
	_ = os.RemoveAll(aliasPath + ".app")

	script := fmt.Sprintf(
		`tell application "Finder" to make alias file to POSIX file %s at POSIX file %s`,
		appleQuote(appPath),
		appleQuote(desktop),
	)
	cmd := exec.Command("osascript", "-e", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("创建桌面别名失败: %s", msg)
	}
	if st, err := os.Stat(aliasPath); err == nil && !st.IsDir() {
		return aliasPath, nil
	}
	return filepath.Join(desktop, "PhraseMate"), nil
}

// Exists reports whether a PhraseMate alias/app is already on the desktop.
func Exists() bool {
	desktop, err := desktopDir()
	if err != nil {
		return false
	}
	for _, name := range []string{"PhraseMate", "PhraseMate.app", "PhraseMate alias"} {
		if st, err := os.Stat(filepath.Join(desktop, name)); err == nil && !st.IsDir() {
			return true
		}
		if st, err := os.Stat(filepath.Join(desktop, name)); err == nil && st.IsDir() && strings.HasSuffix(name, ".app") {
			return true
		}
	}
	return false
}

// EnsureDesktop creates the shortcut if missing and the exe is a real build.
func EnsureDesktop() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	if IsEphemeral(exe) {
		return "", nil
	}
	if Exists() {
		return "", nil
	}
	return CreateDesktop()
}

func ensureAppBundle(exe string) (string, error) {
	if root := appBundleRoot(exe); root != "" {
		return root, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	apps := filepath.Join(home, "Applications")
	if err := os.MkdirAll(apps, 0o755); err != nil {
		return "", err
	}
	app := filepath.Join(apps, "PhraseMate.app")
	macosDir := filepath.Join(app, "Contents", "MacOS")
	resDir := filepath.Join(app, "Contents", "Resources")
	if err := os.MkdirAll(macosDir, 0o755); err != nil {
		return "", err
	}
	if err := os.MkdirAll(resDir, 0o755); err != nil {
		return "", err
	}
	destExe := filepath.Join(macosDir, "PhraseMate")
	if err := copyFile(exe, destExe); err != nil {
		return "", fmt.Errorf("复制程序到应用包失败: %w", err)
	}
	if err := os.Chmod(destExe, 0o755); err != nil {
		return "", err
	}
	plist := filepath.Join(app, "Contents", "Info.plist")
	if err := os.WriteFile(plist, []byte(macInfoPlist), 0o644); err != nil {
		return "", err
	}
	if _, err := brandicon.EnsureICNS(resDir); err != nil {
		return "", fmt.Errorf("写入应用图标失败: %w", err)
	}
	return app, nil
}

func appBundleRoot(exe string) string {
	macOS := filepath.Dir(exe)
	contents := filepath.Dir(macOS)
	app := filepath.Dir(contents)
	if filepath.Base(macOS) == "MacOS" && filepath.Base(contents) == "Contents" && strings.HasSuffix(app, ".app") {
		return app
	}
	return ""
}

func desktopDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("无法定位桌面目录: %w", err)
	}
	for _, name := range []string{"Desktop", "桌面"} {
		p := filepath.Join(home, name)
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("未找到桌面目录")
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func appleQuote(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

const macInfoPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleDevelopmentRegion</key>
	<string>zh_CN</string>
	<key>CFBundleExecutable</key>
	<string>PhraseMate</string>
	<key>CFBundleIconFile</key>
	<string>AppIcon</string>
	<key>CFBundleIdentifier</key>
	<string>com.phrasemate.app</string>
	<key>CFBundleInfoDictionaryVersion</key>
	<string>6.0</string>
	<key>CFBundleName</key>
	<string>PhraseMate</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleShortVersionString</key>
	<string>1.0</string>
	<key>CFBundleVersion</key>
	<string>1</string>
	<key>LSMinimumSystemVersion</key>
	<string>11.0</string>
	<key>NSHighResolutionCapable</key>
	<true/>
	<key>NSPrincipalClass</key>
	<string>NSApplication</string>
	<key>NSAppTransportSecurity</key>
	<dict>
		<key>NSAllowsLocalNetworking</key>
		<true/>
	</dict>
</dict>
</plist>
`
