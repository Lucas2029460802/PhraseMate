//go:build !windows && !darwin

package shortcut

import "fmt"

func CreateDesktop() (string, error) {
	return "", fmt.Errorf("桌面快捷方式仅支持 Windows / macOS")
}

func Exists() bool { return false }

func EnsureDesktop() (string, error) { return "", nil }
