//go:build !windows

package gitdata

import "os/exec"

func configureCmd(cmd *exec.Cmd) {}
