package selfupdate

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// LegacyInstall reports whether the running executable still has the
// pre-rename file name (in-place updates keep it), and makes a symlink with
// the new name beside it. link is "" when the link could not be made.
func LegacyInstall(legacy, name string) (exe, link string, ok bool) {
	exe, err := os.Executable()
	if err != nil {
		return "", "", false
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return "", "", false
	}
	link, ok = linkBeside(exe, legacy, name)
	return exe, link, ok
}

func linkBeside(exe, legacy, name string) (string, bool) {
	if strings.TrimSuffix(filepath.Base(exe), ".exe") != legacy {
		return "", false
	}
	if runtime.GOOS == "windows" {
		// Symlinks need extra privileges there.
		return "", true
	}
	link := filepath.Join(filepath.Dir(exe), name)
	if _, err := os.Lstat(link); err == nil {
		return link, true
	}
	if err := os.Symlink(filepath.Base(exe), link); err != nil {
		return "", true
	}
	return link, true
}
