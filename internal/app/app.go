// Package app holds process-wide constants and directory layout.
package app

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Version is set at build time with -ldflags "-X .../internal/app.Version=x.y.z".
var Version = "1.1.0"

const (
	Author            = "jkpair"
	SupportProjectURL = "https://buymeacoffee.com/benexl"
	Repo              = "jkpair/reelterm"

	// LegacyName is the name used before the rename to reelterm. Its
	// config and cache directories are moved over on first start.
	LegacyName = "yt-x"
)

// Paths describes every directory and file reelterm reads or writes.
type Paths struct {
	Name          string
	ConfigDir     string
	ConfigFile    string
	ThemesDir     string
	PluginsDir    string
	CacheDir      string
	ThumbsDir     string
	PlaylistsDir  string
	ArchiveDir    string
	CustomLists   string
	CustomCmds    string
	SearchHistory string
}

// Name returns the CLI name, overridable via REELTERM_APP_NAME.
func Name() string {
	if n := os.Getenv("REELTERM_APP_NAME"); n != "" {
		return n
	}
	return "reelterm"
}

func xdg(env, fallback string) string {
	if v := os.Getenv(env); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, fallback)
	if runtime.GOOS == "windows" {
		// Use %APPDATA% / %LOCALAPPDATA% unless an earlier install (e.g.
		// the Bash version under Git Bash) already lives in ~/.config.
		for _, n := range []string{Name(), LegacyName} {
			if _, err := os.Stat(filepath.Join(dir, n)); err == nil {
				return dir
			}
		}
		native := os.UserConfigDir
		if env == "XDG_CACHE_HOME" {
			native = os.UserCacheDir
		}
		if d, err := native(); err == nil {
			return d
		}
	}
	return dir
}

// migrate moves base/yt-x to base/reelterm when only the old one exists.
// If the move fails the old directory keeps being used.
func migrate(base, name string) string {
	dir := filepath.Join(base, name)
	if name != "reelterm" {
		return dir
	}
	old := filepath.Join(base, LegacyName)
	if _, err := os.Stat(dir); err == nil {
		return dir
	}
	if _, err := os.Stat(old); err != nil {
		return dir
	}
	if err := os.Rename(old, dir); err != nil {
		return old
	}
	return dir
}

// NewPaths resolves the directory layout, moving an install from before
// the rename into place.
func NewPaths() Paths {
	name := Name()
	cfg := migrate(xdg("XDG_CONFIG_HOME", ".config"), name)
	cache := migrate(xdg("XDG_CACHE_HOME", ".cache"), name)
	conf := filepath.Join(cfg, name+".conf")
	if old := filepath.Join(cfg, LegacyName+".conf"); filepath.Base(cfg) != LegacyName {
		if _, err := os.Stat(conf); os.IsNotExist(err) {
			_ = os.Rename(old, conf)
		}
	} else {
		conf = old
	}
	return Paths{
		Name:          name,
		ConfigDir:     cfg,
		ConfigFile:    conf,
		ThemesDir:     filepath.Join(cfg, "themes"),
		PluginsDir:    filepath.Join(cfg, "plugins"),
		CacheDir:      cache,
		ThumbsDir:     filepath.Join(cache, "thumbs"),
		PlaylistsDir:  filepath.Join(cache, "playlists"),
		ArchiveDir:    filepath.Join(cache, "yt-dlp-archive"),
		CustomLists:   filepath.Join(cfg, "custom_playlists.json"),
		CustomCmds:    filepath.Join(cfg, "custom_cmds.json"),
		SearchHistory: filepath.Join(cache, "search_history.txt"),
	}
}

// Ensure creates all directories.
func (p Paths) Ensure() error {
	for _, d := range []string{p.ConfigDir, p.ThemesDir, p.PluginsDir, p.CacheDir, p.ThumbsDir, p.PlaylistsDir, p.ArchiveDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// Platform returns linux, mac, windows or android.
func Platform() string {
	switch runtime.GOOS {
	case "android":
		return "android"
	case "darwin":
		return "mac"
	case "windows":
		return "windows"
	}
	if os.Getenv("TERMUX_VERSION") != "" || strings.Contains(os.Getenv("PREFIX"), "com.termux") {
		return "android"
	}
	return "linux"
}

// ExpandHome expands a leading ~ or $HOME.
func ExpandHome(p string) string {
	home, _ := os.UserHomeDir()
	switch {
	case p == "~":
		return home
	case strings.HasPrefix(p, "~/"):
		return filepath.Join(home, p[2:])
	case strings.HasPrefix(p, "$HOME"):
		return home + p[len("$HOME"):]
	}
	return p
}
