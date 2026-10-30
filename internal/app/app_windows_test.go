//go:build windows

package app

import (
	"os"
	"strings"
	"testing"
)

func TestWindowsPathsUseAppData(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("USERPROFILE", t.TempDir()) // no existing ~/.config/reelterm
	p := NewPaths()
	cfg, _ := os.UserConfigDir()
	cache, _ := os.UserCacheDir()
	if !strings.HasPrefix(p.ConfigDir, cfg) || !strings.HasPrefix(p.CacheDir, cache) {
		t.Fatalf("config=%s cache=%s, want under %s and %s", p.ConfigDir, p.CacheDir, cfg, cache)
	}
}

func TestWindowsKeepsExistingDotConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("USERPROFILE", home)
	os.MkdirAll(home+`\.config\reelterm`, 0o755)
	if p := NewPaths(); p.ConfigDir != home+`\.config\reelterm` {
		t.Fatalf("config=%s", p.ConfigDir)
	}
}
