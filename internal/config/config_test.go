package config

import (
	"path/filepath"
	"testing"
)

func TestNumeric(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{{"", 30}, {"abc", 30}, {"50", 50}, {"0", 1}, {"9999", 100}, {"4k5", 45}}
	for _, c := range cases {
		if got := Numeric(c.in, 30, 1, 100); got != c.want {
			t.Errorf("Numeric(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestLoadAndSet(t *testing.T) {
	p := filepath.Join(t.TempDir(), "yt-x.conf")
	cfg, first, err := Load(p)
	if err != nil || !first {
		t.Fatalf("first load: first=%v err=%v", first, err)
	}
	if cfg.Theme != "everforest-dark-hard" || cfg.NoOfSearchResults != 30 {
		t.Fatalf("defaults wrong: %+v", cfg)
	}
	if err := cfg.Set("THEME", "nord"); err != nil {
		t.Fatal(err)
	}
	cfg2, first, _ := Load(p)
	if first || cfg2.Theme != "nord" {
		t.Fatalf("reload: first=%v theme=%s", first, cfg2.Theme)
	}
}

func TestParseIgnoresComments(t *testing.T) {
	raw := Parse([]byte("# THEME: x\nTHEME: nord\nPLAYER:vlc\n"))
	if raw["THEME"] != "nord" || raw["PLAYER"] != "vlc" {
		t.Fatalf("%v", raw)
	}
}
