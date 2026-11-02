package main

import (
	"encoding/json"
	"testing"
)

func TestParseArgs(t *testing.T) {
	o, err := parseArgs([]string{"/S", "--no-deps", "--player", "vlc", "--no-desktop"})
	if err != nil {
		t.Fatal(err)
	}
	if !o.silent || o.deps || o.player != "vlc" || o.desktop || !o.font || o.uninstall {
		t.Fatalf("options = %+v", o)
	}
	if o, _ := parseArgs(nil); o.silent || !o.deps || o.player != "mpv" || !o.desktop {
		t.Fatalf("defaults = %+v", o)
	}
	if _, err := parseArgs([]string{"--player"}); err == nil {
		t.Fatal("--player without a value was accepted")
	}
	if _, err := parseArgs([]string{"--bogus"}); err == nil {
		t.Fatal("unknown flag was accepted")
	}
}

func TestPlayerChoice(t *testing.T) {
	for in, want := range map[string]string{"": "mpv", "1": "mpv", " 2 ": "vlc", "VLC": "vlc", "3": ""} {
		if got, ok := playerChoice(in); !ok || got != want {
			t.Errorf("playerChoice(%q) = %q, %v", in, got, ok)
		}
	}
	if _, ok := playerChoice("4"); ok {
		t.Error("4 was accepted")
	}
}

func TestFragment(t *testing.T) {
	var f struct {
		Profiles []struct {
			Name        string
			Commandline string
			Font        *struct{ Face string }
		}
	}
	exe := `C:\Users\a\AppData\Local\Programs\reelterm\reelterm.exe`
	if err := json.Unmarshal(fragment(exe, true), &f); err != nil {
		t.Fatal(err)
	}
	p := f.Profiles[0]
	if p.Name != "reelterm" || p.Commandline != `"`+exe+`"` || p.Font == nil || p.Font.Face != nerdFontFace {
		t.Fatalf("profile = %+v", p)
	}
	f.Profiles = nil
	json.Unmarshal(fragment(exe, false), &f)
	if f.Profiles[0].Font != nil {
		t.Fatal("font set without the Nerd Font")
	}
}

func TestPayloadMissing(t *testing.T) {
	if _, err := payload.ReadFile("payload/reelterm.exe"); err == nil {
		t.Skip("payload present")
	}
	if _, err := reeltermExe(); err == nil {
		t.Fatal("no error without a payload")
	}
}
