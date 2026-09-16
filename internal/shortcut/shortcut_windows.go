//go:build windows

package shortcut

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"phrasemate/internal/brandicon"
)

// CreateDesktop creates/overwrites PhraseMate.lnk on the current user's desktop.
// Returns the shortcut path.
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
		return "", fmt.Errorf("当前是 go run 临时程序，请先用 go build 打包成 PhraseMate.exe 再创建快捷方式")
	}

	desktop, err := desktopDir()
	if err != nil {
		return "", err
	}
	lnk := filepath.Join(desktop, "PhraseMate.lnk")
	workDir := filepath.Dir(exe)

	iconPath, err := brandicon.EnsureFile(workDir)
	if err != nil {
		iconPath = exe // fall back to exe icon (may be default)
	}

	ps := fmt.Sprintf(
		"$ws = New-Object -ComObject WScript.Shell; "+
			"$s = $ws.CreateShortcut(%s); "+
			"$s.TargetPath = %s; "+
			"$s.WorkingDirectory = %s; "+
			"$s.Description = %s; "+
			"$s.IconLocation = %s; "+
			"$s.Save()",
		psQuote(lnk),
		psQuote(exe),
		psQuote(workDir),
		psQuote("PhraseMate 英语生词本"),
		psQuote(iconPath+",0"),
	)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", ps)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("创建快捷方式失败: %s", msg)
	}
	return lnk, nil
}

// Exists reports whether PhraseMate.lnk is already on the desktop.
func Exists() bool {
	desktop, err := desktopDir()
	if err != nil {
		return false
	}
	st, err := os.Stat(filepath.Join(desktop, "PhraseMate.lnk"))
	return err == nil && !st.IsDir()
}

// EnsureDesktop creates the shortcut if missing and the exe is a real build.
// Returns created path, or "" when skipped.
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

func desktopDir() (string, error) {
	cmd := exec.Command(
		"powershell", "-NoProfile", "-NonInteractive", "-Command",
		"[Environment]::GetFolderPath('Desktop')",
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		p := strings.TrimSpace(string(out))
		if p != "" {
			if st, err := os.Stat(p); err == nil && st.IsDir() {
				return p, nil
			}
		}
	}
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

func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
