package shortcut

import (
	"path/filepath"
	"strings"
)

// IsEphemeral detects go run / go test temp binaries.
func IsEphemeral(exe string) bool {
	lower := strings.ToLower(filepath.ToSlash(exe))
	return strings.Contains(lower, "/go-build") ||
		strings.Contains(lower, "/go-link-") ||
		strings.HasSuffix(lower, "/exe/main.exe")
}
