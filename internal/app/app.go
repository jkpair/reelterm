// Package app holds process-wide constants and directory layout.
package app

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Version is set at build time with -ldflags "-X .../internal/app.Version=x.y.z".
var Version = "1.0.0"

const (
	Author            = "Benexl"
	SupportProjectURL = "https://buymeacoffee.com/benexl"
	Repo              = "jkpair/yt-x-plus"
)

// Paths describes every directory and file yt-x reads or writes.
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

// Name returns the CLI name, overridable via YT_X_APP_NAME.
func Name() string {
	if n := os.Getenv("YT_X_APP_NAME"); n != "" {
		return n
	}
	return "yt-x"
}

func xdg(env, fallback string) string {
	if v := os.Getenv(env); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, fallback)
}

// NewPaths resolves the directory layout (same locations as the Bash version).
func NewPaths() Paths {
	name := Name()
	cfg := filepath.Join(xdg("XDG_CONFIG_HOME", ".config"), name)
	cache := filepath.Join(xdg("XDG_CACHE_HOME", ".cache"), name)
	return Paths{
		Name:          name,
		ConfigDir:     cfg,
		ConfigFile:    filepath.Join(cfg, name+".conf"),
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
