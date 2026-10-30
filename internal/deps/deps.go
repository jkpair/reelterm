// Package deps checks for and installs external programs yt-x needs.
package deps

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Dep is one external program.
type Dep struct {
	Name     string
	Bins     []string // any of these satisfies the dependency
	Required bool
	Purpose  string
	Pkg      map[string]string // package manager -> package name (default: Name)
}

var All = []Dep{
	{Name: "yt-dlp", Bins: []string{"yt-dlp"}, Required: true, Purpose: "fetching YouTube data and streams"},
	{Name: "mpv", Bins: []string{"mpv", "mpv.exe", "vlc"}, Required: true, Purpose: "playback (or vlc)"},
	{Name: "ffmpeg", Bins: []string{"ffmpeg"}, Required: true, Purpose: "merging and converting downloads"},
	{Name: "chafa", Bins: []string{"chafa"}, Purpose: "optional thumbnail renderer (IMAGE_RENDERER: chafa)"},
	{Name: "libnotify", Bins: []string{"notify-send"}, Purpose: "desktop notifications", Pkg: map[string]string{"apt": "libnotify-bin", "brew": ""}},
	{Name: "clipboard", Bins: []string{"wl-copy", "xclip", "xsel", "pbcopy", "termux-clipboard-set"}, Purpose: "Copy Link",
		Pkg: map[string]string{"pacman": "wl-clipboard", "apt": "wl-clipboard", "dnf": "wl-clipboard", "zypper": "wl-clipboard", "xbps": "wl-clipboard", "apk": "wl-clipboard", "brew": ""}},
}

// Found returns the first available binary for d, or "".
func (d Dep) Found() string {
	for _, b := range d.Bins {
		if p, err := exec.LookPath(b); err == nil {
			return p
		}
	}
	return ""
}

// Missing returns required (or all, if optional) dependencies not installed.
func Missing(includeOptional bool) []Dep {
	var out []Dep
	for _, d := range All {
		if (d.Required || includeOptional) && d.Found() == "" {
			out = append(out, d)
		}
	}
	return out
}

// Report prints a status table and returns false if a required dep is missing.
func Report(w io.Writer) bool {
	ok := true
	fmt.Fprintln(w, "Dependencies:")
	for _, d := range All {
		p := d.Found()
		mark, state := "✓", p
		if p == "" {
			mark, state = "·", "not found (optional)"
			if d.Required {
				mark, state, ok = "✗", "MISSING", false
			}
		}
		fmt.Fprintf(w, "  %s %-10s %-34s %s\n", mark, d.Name, d.Purpose, state)
	}
	return ok
}

type manager struct{ name, install string }

func detect() *manager {
	if runtime.GOOS == "darwin" {
		return &manager{"brew", "brew install"}
	}
	if os.Getenv("TERMUX_VERSION") != "" {
		return &manager{"pkg", "pkg install -y"}
	}
	for _, m := range []manager{
		{"pacman", "sudo pacman -S --needed --noconfirm"}, {"apt", "sudo apt install -y"}, {"dnf", "sudo dnf install -y"},
		{"zypper", "sudo zypper install -y"}, {"emerge", "sudo emerge --ask=n"}, {"xbps", "sudo xbps-install -y"},
		{"apk", "sudo apk add"}, {"brew", "brew install"},
	} {
		bin := m.name
		if bin == "xbps" {
			bin = "xbps-install"
		}
		if _, err := exec.LookPath(bin); err == nil {
			return &m
		}
	}
	return nil
}

// Install offers to install missing dependencies with the system package manager.
func Install(in io.Reader, out io.Writer, includeOptional bool) error {
	missing := Missing(includeOptional)
	if len(missing) == 0 {
		fmt.Fprintln(out, "All dependencies are installed.")
		return nil
	}
	m := detect()
	if m == nil {
		return fmt.Errorf("no supported package manager found; install manually: %s", names(missing))
	}
	var pkgs []string
	for _, d := range missing {
		p, ok := d.Pkg[m.name]
		if !ok {
			p = d.Name
		}
		if p != "" {
			pkgs = append(pkgs, p)
		}
	}
	cmdline := m.install + " " + strings.Join(pkgs, " ")
	fmt.Fprintf(out, "Will run: %s\nProceed? [y/N]: ", cmdline)
	ans, _ := bufio.NewReader(in).ReadString('\n')
	if a := strings.ToLower(strings.TrimSpace(ans)); a != "y" && a != "yes" {
		return fmt.Errorf("cancelled")
	}
	f := strings.Fields(cmdline)
	c := exec.Command(f[0], f[1:]...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	return c.Run()
}

func names(ds []Dep) string {
	var n []string
	for _, d := range ds {
		n = append(n, d.Name)
	}
	return strings.Join(n, ", ")
}

// InstallSelf copies the running binary to dir.
func InstallSelf(dir string, out io.Writer) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return err
	}
	name := filepath.Base(exe)
	dst := filepath.Join(dir, name)
	data, err := os.ReadFile(exe)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dst+".tmp", data, 0o755); err == nil {
		if err := os.Rename(dst+".tmp", dst); err != nil {
			return err
		}
		fmt.Fprintln(out, "Installed to", dst)
		return nil
	}
	fmt.Fprintf(out, "%s is not writable, using sudo…\n", dir)
	c := exec.Command("sudo", "install", "-m", "0755", exe, dst)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		return err
	}
	fmt.Fprintln(out, "Installed to", dst)
	return nil
}
