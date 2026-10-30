// Package youtube builds YouTube URLs and talks to the oEmbed endpoint.
package youtube

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"time"
)

const (
	Home          = "https://www.youtube.com"
	Subscriptions = "https://www.youtube.com/feed/subscriptions"
	Playlists     = "https://www.youtube.com/feed/playlists"
	Channels      = "https://www.youtube.com/feed/channels"
	History       = "https://www.youtube.com/feed/history"
	Clips         = "https://www.youtube.com/feed/clips"
	Liked         = "https://www.youtube.com/playlist?list=LL"
	WatchLater    = "https://www.youtube.com/playlist?list=WL"
	Trending      = "https://www.youtube.com/feed/trending"
)

// Filters maps search prefixes to YouTube's sp= parameter.
var Filters = map[string]string{
	":hour": "EgIIAQ%253D%253D", ":today": "EgIIAg%253D%253D", ":week": "EgIIAw%253D%253D",
	":month": "EgIIBA%253D%253D", ":year": "EgIIBQ%253D%253D", ":video": "EgIQAQ%253D%253D",
	":playlist": "EgIQAw%253D%253D", ":movie": "EgIQBA%253D%253D", ":live": "EgJAAQ%253D%253D",
	":short": "EgQQARgB", ":long": "EgQQARgC", ":4k": "EgJwAQ%253D%253D", ":hd": "EgIgAQ%253D%253D",
	":subtitles": "EgIoAQ%253D%253D", ":360": "EgJ4AQ%253D%253D", ":vr": "EgLIAQ%253D%253D",
	":3d": "EgI4AQ%253D%253D", ":hdr": "EgPIAQ%253D%253D", ":local": "EgO4AQ%253D%253D",
	":newest": "CAISAhAB", ":views": "CAMSAhAB", ":rating": "CAESAhAB",
}

const (
	spVideo    = "EgIQAQ%253D%253D"
	spChannel  = "EgIQAg%253D%253D"
	spPlaylist = "EgIQAw%253D%253D"
)

var filterRe = regexp.MustCompile(`^(:[a-z0-9]+)\s+(.+)$`)

// ParseQuery splits ":week foo bar" into the term and sp value. Without a
// filter the sp is empty, matching the Bash version.
func ParseQuery(q string) (term, sp string) {
	m := filterRe.FindStringSubmatch(q)
	if m == nil {
		return q, ""
	}
	if sp, ok := Filters[m[1]]; ok {
		return m[2], sp
	}
	return m[2], spVideo
}

func results(term, sp string) string {
	return "https://www.youtube.com/results?search_query=" + url.QueryEscape(term) + "&sp=" + sp
}

// SearchURL builds a search URL, honouring a leading :filter.
func SearchURL(q string) string {
	term, sp := ParseQuery(q)
	return results(term, sp)
}

// HistorySearchURL is used when re-running a history item (always videos).
func HistorySearchURL(term string) string { return results(term, spVideo) }

func ChannelSearchURL(term string) string  { return results(term, spChannel) }
func PlaylistSearchURL(term string) string { return results(term, spPlaylist) }

// MixURL returns the radio mix playlist URL for a video.
func MixURL(videoID string) string {
	return "https://www.youtube.com/watch?v=" + videoID + "&list=RD" + videoID
}

// ChannelTabSearch builds <channel>/search?query=term.
func ChannelTabSearch(base, term string) string {
	return base + "/search?query=" + url.QueryEscape(term)
}

var client = &http.Client{Timeout: 10 * time.Second}

// OEmbedAuthor fetches the channel name for a video without auth.
func OEmbedAuthor(ctx context.Context, id string) (string, error) {
	u := "https://www.youtube.com/oembed?format=json&url=" + url.QueryEscape("https://www.youtube.com/watch?v="+id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var v struct {
		AuthorName string `json:"author_name"`
	}
	err = json.NewDecoder(resp.Body).Decode(&v)
	return v.AuthorName, err
}
