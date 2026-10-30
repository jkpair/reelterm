//go:build windows

package player

import "os/exec"

func detach(cmd *exec.Cmd) {}
