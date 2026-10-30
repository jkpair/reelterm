package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jkpair/yt-x-plus/internal/config"
	"github.com/jkpair/yt-x-plus/internal/ytdlp"
)

func newRuntime(t *testing.T) *Runtime {
	cfg, _, err := config.Load(filepath.Join(t.TempDir(), "yt-x.conf"))
	if err != nil {
		t.Fatal(err)
	}
	return New(cfg, ytdlp.New(cfg), nil, func(string, []string) int { return 0 })
}

func TestExamplePluginLoads(t *testing.T) {
	r := newRuntime(t)
	defer r.Close()
	if err := r.Load("../../plugins/example.lua"); err != nil {
		t.Fatal(err)
	}
	h := r.Hooks()
	if n := len(h.MenuItems()); n != 2 {
		t.Errorf("menu items = %d", n)
	}
	if n := len(h.VideoActions()); n != 2 {
		t.Errorf("video actions = %d", n)
	}
}

func TestCallbackResult(t *testing.T) {
	r := newRuntime(t)
	defer r.Close()
	p := filepath.Join(t.TempDir(), "t.lua")
	src := `
yt.video_action("echo", function(v) return "got " .. v.title end)
yt.menu_item{label = "list", run = function()
  yt.show_list("Mine", {{id = "a", title = "A", url = "https://youtu.be/a"}})
end}
`
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := r.Load(p); err != nil {
		t.Fatal(err)
	}
	h := r.Hooks()
	res := h.VideoActions()[0].Run(&ytdlp.Entry{Title: "X"})
	if res.Toast != "got X" {
		t.Errorf("toast = %q", res.Toast)
	}
	res = h.MenuItems()[0].Run(nil)
	if res.Title != "Mine" || len(res.Entries) != 1 || res.Entries[0].ID != "a" {
		t.Errorf("result = %+v", res)
	}
}

func TestBashExtensionRejected(t *testing.T) {
	r := newRuntime(t)
	defer r.Close()
	if err := r.Load("../../extensions/downloads"); err == nil {
		t.Fatal("expected bash extension to be rejected")
	}
}
