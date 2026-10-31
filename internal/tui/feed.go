package tui

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/jkpair/reelterm/internal/rss"
	"github.com/jkpair/reelterm/internal/ytdlp"
)

const (
	// channelFeedLimit caps the merged feed; every entry queues a thumbnail.
	channelFeedLimit = 250
	// channelFeedPerChannel matches the 15 uploads an RSS feed lists.
	channelFeedPerChannel = 15
	rssWorkers            = 8
)

// NewChannelFeed merges the RSS feeds of the local channels (subscriptions.json).
func NewChannelFeed(env *Env) Screen {
	if subs, _ := env.Store.Subscriptions(); len(subs) == 0 {
		return NewMessage(env, "Channel Feed", "No local channels yet.\n\nSubscribe to channels from a video's action menu, or import your YouTube subscriptions from Channels.")
	}
	return NewVideoList(env, Source{
		Title: "Channel Feed",
		Kind:  KindFeed,
		Load: func(ctx context.Context, fresh bool) (*ytdlp.Playlist, string, error) {
			return loadChannelFeed(ctx, env, fresh)
		},
	})
}

type channelFeedResult struct {
	entries  []ytdlp.Entry
	resolved string // channel ID looked up from the channel page
	err      error
}

// loadChannelFeed returns the merged feed and a note naming how many channels failed.
func loadChannelFeed(ctx context.Context, env *Env, fresh bool) (*ytdlp.Playlist, string, error) {
	subs, _ := env.Store.Subscriptions()
	ttl := time.Duration(max(env.Cfg.FeedCacheTTL, 300)) * time.Second
	res := make([]channelFeedResult, len(subs))
	sem := make(chan struct{}, rssWorkers)
	var wg sync.WaitGroup
	for i, ch := range subs {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			res[i] = fetchChannelFeed(ctx, env, ch, ttl, fresh)
		})
	}
	wg.Wait()

	var all []ytdlp.Entry
	var errs []error
	resolved := map[string]string{}
	for i, r := range res {
		if r.resolved != "" {
			resolved[subs[i].Key()] = r.resolved
		}
		if r.err != nil {
			errs = append(errs, r.err)
			continue
		}
		all = append(all, r.entries...)
	}
	if len(resolved) > 0 {
		_ = env.Store.SetChannelIDs(resolved)
	}
	if len(all) == 0 && len(errs) > 0 {
		return nil, "", fmt.Errorf("%d of %d channels failed: %w", len(errs), len(subs), errors.Join(errs[:min(len(errs), 3)]...))
	}
	var note string
	if len(errs) > 0 {
		note = fmt.Sprintf("%d of %d channels could not be loaded", len(errs), len(subs))
	}
	slices.SortStableFunc(all, func(a, b ytdlp.Entry) int {
		return cmp.Compare(ts(b), ts(a))
	})
	return &ytdlp.Playlist{Title: "Channel Feed", Entries: all[:min(len(all), channelFeedLimit)]}, note, nil
}

// fetchChannelFeed returns one channel's entries: from the cache while fresh,
// else from RSS, else from yt-dlp, and from a stale cache when all of that fails.
func fetchChannelFeed(ctx context.Context, env *Env, ch ytdlp.Entry, ttl time.Duration, fresh bool) channelFeedResult {
	var r channelFeedResult
	base := ch.ChannelBase()
	id := rss.ChannelID(ch)
	if id == "" && base != "" {
		// On failure yt-dlp below still works from the URL and reports the ID.
		if rid, err := rss.Resolve(ctx, base); err == nil {
			id, r.resolved = rid, rid
		}
	}
	var cached []ytdlp.Entry
	var at time.Time
	var ok bool
	if id != "" {
		base = "https://www.youtube.com/channel/" + id
		if cached, at, ok = env.Store.ChannelFeed(id); ok && !fresh && time.Since(at) < ttl {
			r.entries = cached
			return r
		}
		if es, err := rss.Fetch(ctx, id); err == nil {
			_ = env.Store.SaveChannelFeed(id, es)
			r.entries = es
			return r
		}
	}
	if base == "" {
		r.err = fmt.Errorf("%s: no channel URL", ch.Title)
		return r
	}
	pl, err := env.YT.Fetch(ctx, base+"/videos", ytdlp.Range{End: channelFeedPerChannel})
	if err != nil {
		if ok {
			r.entries = cached
		} else {
			r.err = fmt.Errorf("%s: %w", ch.ChannelName(), err)
		}
		return r
	}
	pl.FillChannel()
	if id == "" {
		if id = rss.ChannelID(ytdlp.Entry{ID: pl.ID}); id != "" {
			r.resolved = id
		}
	}
	if id != "" {
		_ = env.Store.SaveChannelFeed(id, pl.Entries)
	}
	r.entries = pl.Entries
	return r
}

func ts(e ytdlp.Entry) int64 {
	if e.Timestamp == nil {
		return 0
	}
	return *e.Timestamp
}
