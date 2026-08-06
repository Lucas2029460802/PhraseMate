//go:build !windows

package app

import (
	"fmt"
	"os"
)

// MessageBox prints to stderr on non-Windows builds.
func MessageBox(title, text string) {
	fmt.Fprintf(os.Stderr, "%s\n%s\n", title, text)
}
