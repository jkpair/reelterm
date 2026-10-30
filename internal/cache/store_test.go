package cache

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/jkpair/reelterm/internal/app"
	"github.com/jkpair/reelterm/internal/ytdlp"
)

func newStore(t *testing.T) *Store {
	d := t.TempDir()
	p := app.Paths{
		ConfigDir: d, CacheDir: d, ThumbsDir: filepath.Join(d, "t"), PlaylistsDir: filepath.Join(d, "p"),
		SearchHistory: filepath.Join(d, "h.txt"), CustomLists: filepath.Join(d, "cl.json"), CustomCmds: filepath.Join(d, "cc.json"),
	}
	return New(p)
}

func TestRecentLimitAndOrder(t *testing.T) {
	s := newStore(t)
	for _, id := range []string{"a", "b", "c", "a"} {
		if err := s.AddRecent(ytdlp.Entry{ID: id}, 2); err != nil {
			t.Fatal(err)
		}
	}
	r := s.Recent()
	if len(r) != 2 || r[0].ID != "a" || r[1].ID != "c" {
		t.Fatalf("%+v", r)
	}
}

func TestHistory(t *testing.T) {
	s := newStore(t)
	for _, q := range []string{"x", "y", "x"} {
		_ = s.AddHistory(q)
	}
	h := s.History()
	if len(h) != 2 || h[0] != "x" || h[1] != "y" {
		t.Fatalf("%v", h)
	}
}

func TestFeedCache(t *testing.T) {
	s := newStore(t)
	if !s.FeedStale(time.Hour) {
		t.Fatal("missing cache should be stale")
	}
	_ = s.SaveFeed(&ytdlp.Playlist{Entries: []ytdlp.Entry{{ID: "a"}}})
	if s.FeedStale(time.Hour) {
		t.Fatal("fresh cache reported stale")
	}
	pl, err := s.LoadFeed()
	if err != nil || len(pl.Entries) != 1 {
		t.Fatal(err)
	}
}

func TestMergeFeed(t *testing.T) {
	got := MergeFeed([]ytdlp.Entry{{ID: "b"}, {ID: "c"}}, []ytdlp.Entry{{ID: "a"}, {ID: "b"}})
	if len(got) != 3 || got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("%+v", got)
	}
}
