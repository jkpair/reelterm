// Package rss reads YouTube's per-channel Atom feeds, which need no sign-in.
package rss

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jkpair/reelterm/internal/ytdlp"
)

var client = &http.Client{Timeout: 15 * time.Second}

var (
	idRe      = regexp.MustCompile(`^UC[\w-]{22}$`)
	channelRe = regexp.MustCompile(`/channel/(UC[\w-]{22})`)
	// Ways a channel page names its own ID, most specific first: other
	// channels' /channel/ links can appear earlier in the page.
	pageIDRes = []*regexp.Regexp{
		regexp.MustCompile(`<link rel="canonical" href="https://www\.youtube\.com/channel/(UC[\w-]{22})"`),
		regexp.MustCompile(`"externalId":"(UC[\w-]{22})"`),
		regexp.MustCompile(`feeds/videos\.xml\?channel_id=(UC[\w-]{22})`),
	}
)

// FeedURL is the Atom feed of a channel's latest 15 uploads.
func FeedURL(channelID string) string {
	return "https://www.youtube.com/feeds/videos.xml?channel_id=" + channelID
}

// ChannelID returns a subscription's UC… ID without network access, or "".
func ChannelID(e ytdlp.Entry) string {
	for _, s := range []string{e.ChannelID, e.ID} {
		if idRe.MatchString(s) {
			return s
		}
	}
	for _, s := range []string{e.ChannelURL, e.UploaderURL, e.URL, e.ID} {
		if m := channelRe.FindStringSubmatch(s); m != nil {
			return m[1]
		}
	}
	return ""
}

// Resolve finds the channel ID of a channel URL such as youtube.com/@handle.
func Resolve(ctx context.Context, channelURL string) (string, error) {
	body, err := get(ctx, channelURL, 2<<20)
	if err != nil {
		return "", err
	}
	for _, re := range pageIDRes {
		if m := re.FindSubmatch(body); m != nil {
			return string(m[1]), nil
		}
	}
	return "", fmt.Errorf("no channel ID on %s", channelURL)
}

// Fetch downloads and parses a channel's feed.
func Fetch(ctx context.Context, channelID string) ([]ytdlp.Entry, error) {
	body, err := get(ctx, FeedURL(channelID), 4<<20)
	if err != nil {
		return nil, err
	}
	return Parse(body)
}

func get(ctx context.Context, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	// Skips the EU cookie-consent redirect on channel pages.
	req.Header.Set("Cookie", "SOCS=CAI")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}

type feed struct {
	Entries []struct {
		VideoID   string `xml:"http://www.youtube.com/xml/schemas/2015 videoId"`
		ChannelID string `xml:"http://www.youtube.com/xml/schemas/2015 channelId"`
		Title     string `xml:"title"`
		Link      struct {
			Href string `xml:"href,attr"`
		} `xml:"link"`
		Author struct {
			Name string `xml:"name"`
			URI  string `xml:"uri"`
		} `xml:"author"`
		Published string `xml:"published"`
		Group     struct {
			Description string `xml:"description"`
			Thumbnail   struct {
				URL    string `xml:"url,attr"`
				Width  int    `xml:"width,attr"`
				Height int    `xml:"height,attr"`
			} `xml:"thumbnail"`
			Stats struct {
				Views string `xml:"views,attr"`
			} `xml:"community>statistics"`
		} `xml:"http://search.yahoo.com/mrss/ group"`
	} `xml:"entry"`
}

// Parse converts an Atom feed into flat-playlist entries.
func Parse(data []byte) ([]ytdlp.Entry, error) {
	var f feed
	if err := xml.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	out := make([]ytdlp.Entry, 0, len(f.Entries))
	for _, x := range f.Entries {
		if x.VideoID == "" {
			continue
		}
		e := ytdlp.Entry{
			Type: "url", IEKey: "Youtube", ID: x.VideoID,
			URL:       "https://www.youtube.com/watch?v=" + x.VideoID,
			Title:     strings.TrimSpace(x.Title),
			Channel:   x.Author.Name,
			ChannelID: x.ChannelID,
			// Shorts link to /shorts/; keep them distinguishable.
			WebpageURL: x.Link.Href,
		}
		if x.Author.URI != "" {
			e.ChannelURL, e.UploaderURL = x.Author.URI, x.Author.URI
		}
		if d := x.Group.Description; d != "" {
			e.Description = &d
		}
		if t, err := time.Parse(time.RFC3339, x.Published); err == nil {
			ts := t.Unix()
			e.Timestamp = &ts
		}
		if v, err := strconv.ParseInt(x.Group.Stats.Views, 10, 64); err == nil {
			e.ViewCount = &v
		}
		if th := x.Group.Thumbnail; th.URL != "" {
			e.Thumbnails = []ytdlp.Thumbnail{{URL: th.URL, Width: th.Width, Height: th.Height}}
		}
		out = append(out, e)
	}
	return out, nil
}

// IsShort reports whether a parsed entry is a YouTube Short.
func IsShort(e ytdlp.Entry) bool { return strings.Contains(e.WebpageURL, "/shorts/") }
