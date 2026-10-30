package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewPathsMigratesLegacyDirs(t *testing.T) {
	cfgHome, cacheHome := t.TempDir(), t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgHome)
	t.Setenv("XDG_CACHE_HOME", cacheHome)
	t.Setenv("REELTERM_APP_NAME", "")
	os.MkdirAll(filepath.Join(cfgHome, "yt-x"), 0o755)
	os.WriteFile(filepath.Join(cfgHome, "yt-x", "yt-x.conf"), []byte("THEME: nord\n"), 0o644)
	os.MkdirAll(filepath.Join(cacheHome, "yt-x"), 0o755)

	p := NewPaths()
	if p.ConfigFile != filepath.Join(cfgHome, "reelterm", "reelterm.conf") {
		t.Fatalf("config file = %s", p.ConfigFile)
	}
	if b, err := os.ReadFile(p.ConfigFile); err != nil || string(b) != "THEME: nord\n" {
		t.Fatalf("config not moved: %q %v", b, err)
	}
	if p.CacheDir != filepath.Join(cacheHome, "reelterm") {
		t.Fatalf("cache dir = %s", p.CacheDir)
	}
	for _, d := range []string{filepath.Join(cfgHome, "yt-x"), filepath.Join(cacheHome, "yt-x")} {
		if _, err := os.Stat(d); !os.IsNotExist(err) {
			t.Fatalf("%s still exists", d)
		}
	}
}

func TestNewPathsKeepsExistingNewDir(t *testing.T) {
	cfgHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgHome)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("REELTERM_APP_NAME", "")
	os.MkdirAll(filepath.Join(cfgHome, "yt-x"), 0o755)
	os.MkdirAll(filepath.Join(cfgHome, "reelterm"), 0o755)

	if p := NewPaths(); p.ConfigDir != filepath.Join(cfgHome, "reelterm") {
		t.Fatalf("config dir = %s", p.ConfigDir)
	}
	if _, err := os.Stat(filepath.Join(cfgHome, "yt-x")); err != nil {
		t.Fatal("legacy dir should be left alone when the new one exists")
	}
}
