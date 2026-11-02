//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"github.com/jkpair/reelterm/internal/app"
	"github.com/jkpair/reelterm/internal/config"
	"github.com/jkpair/reelterm/internal/winget"
)

const (
	uninstallKey = `Software\Microsoft\Windows\CurrentVersion\Uninstall\reelterm`
	storeURL     = "ms-windows-store://pdp/?ProductId=9NBLGGH4NNS1"
)

type layout struct {
	dir, exe, setup, fragmentDir, startMenu, desktop string
}

func paths() layout {
	local := os.Getenv("LOCALAPPDATA")
	dir := filepath.Join(local, "Programs", "reelterm")
	l := layout{
		dir:         dir,
		exe:         filepath.Join(dir, "reelterm.exe"),
		setup:       filepath.Join(dir, "reelterm-setup.exe"),
		fragmentDir: filepath.Join(local, "Microsoft", "Windows Terminal", "Fragments", "reelterm"),
	}
	if d, err := windows.KnownFolderPath(windows.FOLDERID_Programs, 0); err == nil {
		l.startMenu = filepath.Join(d, "reelterm.lnk")
	}
	if d, err := windows.KnownFolderPath(windows.FOLDERID_Desktop, 0); err == nil {
		l.desktop = filepath.Join(d, "reelterm.lnk")
	}
	return l
}

func step(format string, a ...any) { fmt.Printf("• "+format+"\n", a...) }

func install(o options) error {
	fmt.Printf("reelterm setup %s\n\n", version)
	if v := windows.RtlGetVersion(); v.MajorVersion < 10 || v.BuildNumber < 17763 {
		return fmt.Errorf("reelterm needs Windows 10 version 1809 or newer (this is %d.%d.%d)", v.MajorVersion, v.MinorVersion, v.BuildNumber)
	}
	exeData, err := reeltermExe()
	if err != nil {
		return err
	}
	if !o.silent {
		ask(&o)
		fmt.Println()
	}
	needWinget := o.deps || o.font
	if needWinget && !winget.Available() {
		fmt.Println("winget was not found. Install App Installer from the Microsoft Store, then run this setup again.")
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", storeURL).Start()
		return errors.New("winget is missing; nothing was changed")
	}

	if o.deps {
		installDeps(o)
	}
	fontOK := fontInstalled
	if o.font {
		fontOK = func() bool { return ensure("JetBrainsMono Nerd Font", fontInstalled, winget.NerdFont) }
	}
	hasFont := fontOK()
	winget.RefreshPath()

	l := paths()
	if err := writeFiles(l, exeData); err != nil {
		return err
	}
	if added, err := winget.AddUserPath(l.dir); err != nil {
		return fmt.Errorf("adding %s to PATH: %w", l.dir, err)
	} else if added {
		step("Added %s to your PATH", l.dir)
	}
	addPlayerToPath(o.player)

	if err := os.MkdirAll(l.fragmentDir, 0o755); err == nil {
		err = os.WriteFile(filepath.Join(l.fragmentDir, "reelterm.json"), fragment(l.exe, hasFont), 0o644)
		if err == nil {
			step("Added a \"reelterm\" profile to Windows Terminal")
		}
	}

	target, args := l.exe, ""
	wt, err := exec.LookPath("wt")
	if err == nil {
		target, args = wt, `-p "reelterm"`
	}
	lnks := []string{l.startMenu}
	if o.desktop {
		lnks = append(lnks, l.desktop)
	}
	for _, lnk := range lnks {
		if lnk == "" {
			continue
		}
		if err := shortcut(lnk, target, args, l.exe); err != nil {
			fmt.Printf("  could not create %s: %v\n", lnk, err)
		} else {
			step("Created shortcut %s", lnk)
		}
	}

	if err := register(l); err != nil {
		return fmt.Errorf("registering in Settings → Apps: %w", err)
	}
	step("Registered reelterm in Settings → Apps")

	if err := savePlayer(o.player); err != nil {
		fmt.Println("  could not save the player setting:", err)
	}

	fmt.Println()
	c := exec.Command(l.exe, "--check-deps")
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	if c.Run() != nil {
		fmt.Println("\nSome dependencies are still missing; run `reelterm --install` in a new terminal.")
	}
	fmt.Println("\nreelterm is installed. Open it from the Start menu or type reelterm in a new terminal.")
	if !o.silent && yes("Launch reelterm now?") {
		launch := exec.Command("cmd", "/c", "start", "", l.exe)
		if wt != "" {
			launch = exec.Command(wt, "-p", "reelterm")
		}
		_ = launch.Start()
	}
	return nil
}

func installDeps(o options) {
	ensure("yt-dlp", found("yt-dlp", winget.YtDlp), winget.YtDlp)
	ensure("ffmpeg", found("ffmpeg", winget.FFmpeg), winget.FFmpeg)
	switch o.player {
	case "mpv":
		ensure("mpv", found("mpv", winget.Mpv), winget.Mpv)
	case "vlc":
		ensure("VLC", found("vlc", winget.VLC), winget.VLC)
	}
	ensure("Windows Terminal", found("wt"), winget.Terminal)
}

func found(bin string, ids ...string) func() bool {
	return func() bool { return winget.Find(bin, ids...) != "" }
}

// ensure installs id with winget unless have() says it's there already.
func ensure(name string, have func() bool, id string) bool {
	if have() {
		step("%s is already installed", name)
		return true
	}
	step("Installing %s (%s)…", name, id)
	if err := winget.Install(id, os.Stdout); err != nil {
		fmt.Printf("  %s failed to install: %v\n", name, err)
		return false
	}
	return true
}

func fontInstalled() bool {
	for _, dir := range []string{
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "Windows", "Fonts"),
		filepath.Join(os.Getenv("WINDIR"), "Fonts"),
	} {
		if m, _ := filepath.Glob(filepath.Join(dir, "JetBrainsMonoNerdFont*")); len(m) > 0 {
			return true
		}
	}
	return winget.Listed(winget.NerdFont)
}

// writeFiles puts reelterm.exe and a copy of this setup in l.dir. A running
// reelterm.exe is renamed out of the way first; it removes the .old.exe
// file on its next start.
func writeFiles(l layout, exeData []byte) error {
	if err := os.MkdirAll(l.dir, 0o755); err != nil {
		return err
	}
	tmp := l.exe + ".new"
	if err := os.WriteFile(tmp, exeData, 0o755); err != nil {
		return err
	}
	if err := os.Rename(tmp, l.exe); err != nil {
		old := strings.TrimSuffix(l.exe, ".exe") + ".old.exe"
		_ = os.Remove(old)
		if err := os.Rename(l.exe, old); err != nil {
			os.Remove(tmp)
			return fmt.Errorf("replacing %s (close reelterm and try again): %w", l.exe, err)
		}
		if err := os.Rename(tmp, l.exe); err != nil {
			return err
		}
	}
	step("Installed reelterm.exe to %s", l.dir)
	self, err := os.Executable()
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Clean(self), filepath.Clean(l.setup)) {
		data, err := os.ReadFile(self)
		if err != nil {
			return err
		}
		if err := os.WriteFile(l.setup, data, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// addPlayerToPath puts a winget-installed player's folder on the user PATH
// when winget didn't link it (shinchiro.mpv is a portable zip).
func addPlayerToPath(player string) {
	ids := map[string]string{"mpv": winget.Mpv, "vlc": winget.VLC}
	id, ok := ids[player]
	if !ok {
		return
	}
	if _, err := exec.LookPath(player); err == nil {
		return
	}
	if p := winget.Find(player, id); p != "" {
		if added, err := winget.AddUserPath(filepath.Dir(p)); err == nil && added {
			step("Added %s to your PATH", filepath.Dir(p))
		}
	}
}

// shortcut writes a .lnk through WScript.Shell; the values travel in the
// environment so no quoting can break the script.
func shortcut(lnk, target, args, icon string) error {
	const script = `$s=(New-Object -ComObject WScript.Shell).CreateShortcut($env:RT_LNK);` +
		`$s.TargetPath=$env:RT_TARGET;$s.Arguments=$env:RT_ARGS;$s.IconLocation=$env:RT_ICON;` +
		`$s.WorkingDirectory=$env:USERPROFILE;$s.Description='Browse YouTube from the terminal';$s.Save()`
	c := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	c.Env = append(os.Environ(), "RT_LNK="+lnk, "RT_TARGET="+target, "RT_ARGS="+args, "RT_ICON="+icon+",0")
	if out, err := c.CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func register(l layout) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, uninstallKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	size := uint32(0)
	for _, f := range []string{l.exe, l.setup} {
		if st, err := os.Stat(f); err == nil {
			size += uint32(st.Size() / 1024)
		}
	}
	for name, v := range map[string]string{
		"DisplayName":          "reelterm",
		"DisplayVersion":       version,
		"Publisher":            app.Author,
		"DisplayIcon":          l.exe,
		"InstallLocation":      l.dir,
		"URLInfoAbout":         "https://github.com/" + app.Repo,
		"UninstallString":      `"` + l.setup + `" --uninstall`,
		"QuietUninstallString": `"` + l.setup + `" --uninstall /S`,
	} {
		if err := k.SetStringValue(name, v); err != nil {
			return err
		}
	}
	for name, v := range map[string]uint32{"NoModify": 1, "NoRepair": 1, "EstimatedSize": size} {
		if err := k.SetDWordValue(name, v); err != nil {
			return err
		}
	}
	return nil
}

// savePlayer stores the player choice as PLAYER in reelterm.conf. A config
// created here leaves a .first_run marker so reelterm still shows its
// first-run screens.
func savePlayer(player string) error {
	if player == "none" || player == "" {
		return nil
	}
	p := app.NewPaths()
	if err := p.Ensure(); err != nil {
		return err
	}
	cfg, firstRun, err := config.Load(p.ConfigFile)
	if err != nil {
		return err
	}
	if firstRun {
		_ = os.WriteFile(filepath.Join(p.CacheDir, ".first_run"), nil, 0o644)
	}
	if cfg.Player == player {
		return nil
	}
	if err := cfg.Set("PLAYER", player); err != nil {
		return err
	}
	step("Set the video player to %s", player)
	return nil
}

func uninstall(o options) error {
	l := paths()
	if !o.silent && !yes("Uninstall reelterm? Your settings and the dependencies stay installed.") {
		return errors.New("cancelled")
	}
	exec.Command("taskkill", "/IM", "reelterm.exe", "/F").Run()
	for _, f := range []string{l.startMenu, l.desktop} {
		if f != "" && os.Remove(f) == nil {
			step("Removed %s", f)
		}
	}
	if os.RemoveAll(l.fragmentDir) == nil {
		step("Removed the Windows Terminal profile")
	}
	if removed, err := winget.RemoveUserPath(l.dir); err == nil && removed {
		step("Removed %s from your PATH", l.dir)
	}
	if err := registry.DeleteKey(registry.CURRENT_USER, uninstallKey); err == nil {
		step("Removed reelterm from Settings → Apps")
	}
	if o.purge {
		p := app.NewPaths()
		for _, d := range []string{p.ConfigDir, p.CacheDir} {
			if os.RemoveAll(d) == nil {
				step("Removed %s", d)
			}
		}
	}
	self, _ := os.Executable()
	if strings.HasPrefix(strings.ToLower(filepath.Clean(self)), strings.ToLower(l.dir)+`\`) {
		// A running exe can't delete itself: let cmd retry removing the
		// folder every ~second for two minutes, until this process has
		// exited. ping waits and, unlike timeout, works without a console.
		c := exec.Command("cmd")
		c.SysProcAttr = &syscall.SysProcAttr{
			CmdLine:       fmt.Sprintf(`cmd /c for /l %%i in (1,1,120) do (ping -n 2 127.0.0.1 >nul & rmdir /s /q "%[1]s" 2>nul & if not exist "%[1]s" exit)`, l.dir),
			CreationFlags: windows.CREATE_NO_WINDOW,
		}
		if err := c.Start(); err != nil {
			return fmt.Errorf("removing %s: %w", l.dir, err)
		}
	} else if err := os.RemoveAll(l.dir); err != nil {
		return err
	}
	step("Removed %s", l.dir)
	fmt.Println("\nreelterm was uninstalled.")
	return nil
}
