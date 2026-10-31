package tui

import (
	"context"
	"testing"

	"github.com/jkpair/reelterm/internal/thumb"
	"github.com/jkpair/reelterm/internal/ytdlp"
)

func TestChannelFeedMergesCachedFeeds(t *testing.T) {
	env := testEnv(t, thumb.None)
	const a, b = "UCaaaaaaaaaaaaaaaaaaaaaa", "UCbbbbbbbbbbbbbbbbbbbbbb"
	at := func(n int64) *int64 { return &n }
	_ = env.Store.SetSubscriptions([]ytdlp.Entry{{ID: a, Title: "A"}, {ID: "x", ChannelID: b, Title: "B"}})
	_ = env.Store.SaveChannelFeed(a, []ytdlp.Entry{{ID: "a1", Timestamp: at(10)}, {ID: "a2", Timestamp: at(30)}})
	_ = env.Store.SaveChannelFeed(b, []ytdlp.Entry{{ID: "b1", Timestamp: at(20)}})

	pl, note, err := loadChannelFeed(context.Background(), env, false)
	if err != nil || note != "" {
		t.Fatal(err, note)
	}
	var ids []string
	for _, e := range pl.Entries {
		ids = append(ids, e.ID)
	}
	if len(ids) != 3 || ids[0] != "a2" || ids[1] != "b1" || ids[2] != "a1" {
		t.Fatalf("order: %v", ids)
	}
}
