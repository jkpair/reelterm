// Package winget installs and locates Windows programs through winget. It
// is shared by `reelterm --install` and reelterm-setup.exe.
package winget

import (
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Package ids of the programs reelterm uses.
const (
	YtDlp    = "yt-dlp.yt-dlp"
	FFmpeg   = "Gyan.FFmpeg"
	Mpv      = "shinchiro.mpv"
	VLC      = "VideoLAN.VLC"
	Terminal = "Microsoft.WindowsTerminal"
	NerdFont = "DEVCOM.JetBrainsMonoNerdFont"
)

// InstallArgs returns the winget arguments that install id silently.
func InstallArgs(id string, userScope bool) []string {
	args := []string{"install", "-e", "--id", id}
	if userScope {
		args = append(args, "--scope", "user")
	}
	return append(args, "--silent", "--accept-package-agreements", "--accept-source-agreements")
}

// Available reports whether winget is on PATH.
func Available() bool {
	_, err := exec.LookPath("winget")
	return err == nil
}

// Install installs id for the current user, retrying without --scope user
// for packages that only ship a machine-wide installer. winget's output
// goes to out.
func Install(id string, out io.Writer) error {
	err := run(out, InstallArgs(id, true)...)
	if err != nil {
		err = run(out, InstallArgs(id, false)...)
	}
	return err
}

// Listed reports whether winget knows id as installed.
func Listed(id string) bool {
	return exec.Command("winget", "list", "-e", "--id", id, "--accept-source-agreements").Run() == nil
}

func run(out io.Writer, args ...string) error {
	c := exec.Command("winget", args...)
	c.Stdout, c.Stderr = out, out
	return c.Run()
}

// Find returns the full path of program bin: from PATH, or on Windows from
// the places winget installs to when PATH hasn't caught up yet (the Links
// folder, the package folders of ids, Program Files for VLC). It returns
// "" when bin isn't found.
func Find(bin string, ids ...string) string {
	if p, err := exec.LookPath(bin); err == nil {
		return p
	}
	if runtime.GOOS != "windows" {
		return ""
	}
	exe := bin
	if !strings.HasSuffix(strings.ToLower(exe), ".exe") {
		exe += ".exe"
	}
	base := filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "WinGet")
	cands := []string{filepath.Join(base, "Links", exe)}
	if strings.EqualFold(exe, "vlc.exe") {
		for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)"} {
			cands = append(cands, filepath.Join(os.Getenv(env), "VideoLAN", "VLC", exe))
		}
	}
	for _, c := range cands {
		if isFile(c) {
			return c
		}
	}
	for _, id := range ids {
		if p := FindInPackage(base, id, exe); p != "" {
			return p
		}
	}
	return ""
}

// FindInPackage searches the winget package folders <wingetDir>\Packages\<id>*
// for exe.
func FindInPackage(wingetDir, id, exe string) string {
	dirs, _ := filepath.Glob(filepath.Join(wingetDir, "Packages", id+"*"))
	for _, d := range dirs {
		found := ""
		_ = filepath.WalkDir(d, func(p string, e fs.DirEntry, err error) error {
			if err == nil && !e.IsDir() && strings.EqualFold(e.Name(), exe) {
				found = p
				return fs.SkipAll
			}
			return nil
		})
		if found != "" {
			return found
		}
	}
	return ""
}

func isFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
