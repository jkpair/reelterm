package player

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jkpair/reelterm/internal/config"
	"github.com/jkpair/reelterm/internal/ytdlp"
)

func TestMpvYtdlRaw(t *testing.T) {
	got := mpvYtdlRaw([]string{"--ytdl-format=best", "--sponsorblock-remove", "sponsor", "-N", "4", "--live-from-start"})
	want := []string{"--ytdl-format=best", "--ytdl-raw-options-append=sponsorblock-remove=sponsor", "--ytdl-raw-options-append=concurrent-fragments=4", "--ytdl-raw-options-append=live-from-start="}
	if !slices.Equal(got, want) {
		t.Fatalf("%q", got)
	}
}

func TestClassify(t *testing.T) {
	if _, bot := Classify("ERROR: Sign in to confirm you're not a bot"); !bot {
		t.Fatal("bot not detected")
	}
	if m, _ := Classify("ERROR: Private video"); m == "" {
		t.Fatal("private not detected")
	}
}

func TestMpvCommand(t *testing.T) {
	cfg, _, err := config.Load(filepath.Join(t.TempDir(), "c.conf"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Player = "mpv"
	_ = cfg.Set("SPONSORBLOCK", "true")
	yt := ytdlp.New(cfg)
	yt.Browser = "firefox"
	p := New(cfg, yt)
	cmd, err := p.Command(context.Background(), Request{URL: "https://www.youtube.com/watch?v=x", Title: "T", AudioOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Join(cmd.Args[1:], " ")
	for _, want := range []string{
		"--ytdl-raw-options=cookies-from-browser=firefox", "--ytdl-format=bestvideo+bestaudio/best",
		"--ytdl-raw-options-append=sponsorblock-remove=sponsor,selfpromo", "--save-position-on-quit",
		"--force-media-title=T", "https://www.youtube.com/watch?v=x --no-video --force-window=no",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("missing %q in %s", want, args)
		}
	}
}
