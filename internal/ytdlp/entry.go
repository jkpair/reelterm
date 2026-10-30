// Package ytdlp wraps the yt-dlp binary and models its flat-playlist JSON.
package ytdlp

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

type Thumbnail struct {
	URL    string `json:"url"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

// Entry is one item of a flat playlist: a video, channel or playlist.
type Entry struct {
	Type                string      `json:"_type,omitempty"`
	IEKey               string      `json:"ie_key,omitempty"`
	ID                  string      `json:"id"`
	URL                 string      `json:"url,omitempty"`
	WebpageURL          string      `json:"webpage_url,omitempty"`
	Title               string      `json:"title"`
	Description         *string     `json:"description,omitempty"`
	Channel             string      `json:"channel,omitempty"`
	ChannelID           string      `json:"channel_id,omitempty"`
	ChannelURL          string      `json:"channel_url,omitempty"`
	Uploader            string      `json:"uploader,omitempty"`
	UploaderID          string      `json:"uploader_id,omitempty"`
	UploaderURL         string      `json:"uploader_url,omitempty"`
	ViewCount           *int64      `json:"view_count,omitempty"`
	ConcurrentViewCount *int64      `json:"concurrent_view_count,omitempty"`
	Duration            *float64    `json:"duration,omitempty"`
	Timestamp           *int64      `json:"timestamp,omitempty"`
	LiveStatus          string      `json:"live_status,omitempty"`
	Availability        string      `json:"availability,omitempty"`
	ChannelFollowers    *int64      `json:"channel_follower_count,omitempty"`
	PlaylistCount       *int        `json:"playlist_count,omitempty"`
	Thumbnails          []Thumbnail `json:"thumbnails,omitempty"`
	Thumbnail           string      `json:"thumbnail,omitempty"`
}

// Playlist is the top-level object of `yt-dlp -J --flat-playlist`.
type Playlist struct {
	ID          string  `json:"id,omitempty"`
	Title       string  `json:"title,omitempty"`
	UploaderURL string  `json:"uploader_url,omitempty"`
	Channel     string  `json:"channel,omitempty"`
	Uploader    string  `json:"uploader,omitempty"`
	WebpageURL  string  `json:"webpage_url,omitempty"`
	Entries     []Entry `json:"entries"`
}

// FillChannel sets the channel on entries that lack it, for channel tabs
// where yt-dlp only reports the channel at the playlist level.
func (p *Playlist) FillChannel() {
	name := p.Channel
	if name == "" {
		name = p.Uploader
	}
	if name == "" || p.UploaderURL == "" {
		return
	}
	for i := range p.Entries {
		if p.Entries[i].ChannelName() == "" {
			p.Entries[i].Channel = name
			p.Entries[i].UploaderURL = p.UploaderURL
		}
	}
}

// Key identifies an entry for de-duplication.
func (e *Entry) Key() string {
	if e.ID != "" {
		return e.ID
	}
	return e.URL
}

// ChannelName mirrors `.channel // .uploader // .uploader_id`.
func (e *Entry) ChannelName() string {
	for _, s := range []string{e.Channel, e.Uploader, e.UploaderID} {
		if s != "" {
			return s
		}
	}
	return ""
}

// ChannelBase returns the channel URL used for /videos, /shorts etc.
func (e *Entry) ChannelBase() string {
	for _, s := range []string{e.UploaderURL, e.ChannelURL, e.URL} {
		if s != "" {
			return strings.TrimSuffix(s, "/")
		}
	}
	return ""
}

// ThumbnailURL returns the largest thumbnail, normalising protocol-relative URLs.
func (e *Entry) ThumbnailURL() string {
	u := e.Thumbnail
	if n := len(e.Thumbnails); n > 0 {
		u = e.Thumbnails[n-1].URL
	}
	if strings.HasPrefix(u, "//") {
		u = "https:" + u
	}
	return u
}

// WatchURL returns a URL a player or browser can open.
func (e *Entry) WatchURL() string {
	if e.URL != "" {
		return e.URL
	}
	if e.WebpageURL != "" {
		return e.WebpageURL
	}
	return "https://www.youtube.com/watch?v=" + e.ID
}

// Kind classifies an entry as "video", "channel" or "playlist".
func (e *Entry) Kind() string {
	u := e.WatchURL()
	switch {
	case strings.Contains(u, "/channel/") || strings.Contains(u, "/@") || strings.Contains(u, "/c/") || strings.Contains(u, "/user/"):
		return "channel"
	case strings.Contains(u, "/playlist?") || (strings.Contains(u, "list=") && !strings.Contains(u, "v=")):
		return "playlist"
	}
	return "video"
}

// IsMix reports whether the entry is a YouTube radio mix.
func (e *Entry) IsMix() bool { return strings.Contains(e.WatchURL(), "list=RD") }

// Desc returns the description or "".
func (e *Entry) Desc() string {
	if e.Description == nil {
		return ""
	}
	return *e.Description
}

// NeedsEnrich reports whether metadata is missing (see enrich_metadata_bg).
func (e *Entry) NeedsEnrich() bool {
	return e.ChannelName() == "" || e.ViewCount == nil || e.Duration == nil || e.Timestamp == nil
}

// Merge fills missing fields of e from full metadata m.
func (e *Entry) Merge(m *Entry) {
	if e.Channel == "" {
		e.Channel = m.ChannelName()
	}
	if e.UploaderURL == "" {
		e.UploaderURL = m.UploaderURL
	}
	if e.ViewCount == nil {
		e.ViewCount = m.ViewCount
	}
	if e.Duration == nil {
		e.Duration = m.Duration
	}
	if e.Timestamp == nil {
		e.Timestamp = m.Timestamp
	}
	if e.Description == nil {
		e.Description = m.Description
	}
	if e.LiveStatus == "" {
		e.LiveStatus = m.LiveStatus
	}
	if e.Availability == "" {
		e.Availability = m.Availability
	}
}

// Availability mirrors get_video_availability.
func (e *Entry) Status() string {
	for prefix, s := range map[string]string{
		"[Private video]": "private", "[Private Video]": "private",
		"[Deleted video]": "deleted", "[Deleted Video]": "deleted",
		"[Members only]": "members_only", "[Member only]": "members_only",
		"[Unavailable]": "unavailable",
	} {
		if strings.HasPrefix(e.Title, prefix) {
			return s
		}
	}
	switch e.Availability {
	case "private":
		return "private"
	case "premium_only", "subscriber_only", "needs_premium":
		return "members_only"
	case "needs_auth":
		return "login_required"
	}
	return "available"
}

// StatusLabel is the short tag shown next to unavailable entries.
func StatusLabel(status string) string {
	return map[string]string{
		"private": "PRIVATE", "deleted": "DELETED", "members_only": "MEMBERS ONLY",
		"age_restricted": "AGE RESTRICTED", "login_required": "LOGIN REQUIRED",
		"unavailable": "UNAVAILABLE",
	}[status]
}

// StatusHelp explains why a video can't be played.
func StatusHelp(status string) string {
	return map[string]string{
		"private":        "This video is private and cannot be accessed.",
		"deleted":        "This video has been deleted by the uploader.",
		"members_only":   "This video is for channel members only.",
		"age_restricted": "This video is age-restricted. Try configuring browser cookies.",
		"login_required": "This video requires login. Try configuring browser cookies.",
		"unavailable":    "This video is currently unavailable.",
	}[status]
}

// Views formats the view count with thousands separators, or "N/A".
func (e *Entry) Views() string {
	v := e.ViewCount
	if v == nil {
		v = e.ConcurrentViewCount
	}
	if v == nil {
		return "N/A"
	}
	return Commas(*v)
}

// Commas inserts thousands separators.
func Commas(n int64) string {
	s := strconv.FormatInt(n, 10)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

func plural(n int64, unit string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, unit)
	}
	return fmt.Sprintf("%d %ss", n, unit)
}

// DurationText mirrors the jq duration formatting ("3 mins", "1 hour").
func (e *Entry) DurationText() string {
	if e.Duration == nil {
		return "Unknown"
	}
	d := int64(math.Floor(*e.Duration))
	switch {
	case d >= 3600:
		return plural(d/3600, "hour")
	case d >= 60:
		return plural(d/60, "min")
	}
	return plural(d, "sec")
}

// Clock formats the duration as H:MM:SS / M:SS for list rows.
func (e *Entry) Clock() string {
	if e.Duration == nil {
		return ""
	}
	d := int(*e.Duration)
	if d >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", d/3600, d/60%60, d%60)
	}
	return fmt.Sprintf("%d:%02d", d/60, d%60)
}

// Uploaded returns a relative upload time ("3 days ago").
func (e *Entry) Uploaded(now time.Time) string {
	if e.Timestamp == nil {
		return "Unknown"
	}
	return Relative(now.Unix() - *e.Timestamp)
}

// Relative formats an age in seconds the same way the Bash version did.
func Relative(age int64) string {
	switch {
	case age < 60:
		return "just now"
	case age < 3600:
		return plural(age/60, "minute") + " ago"
	case age < 86400:
		return plural(age/3600, "hour") + " ago"
	case age < 604800:
		return plural(age/86400, "day") + " ago"
	case age < 2635200:
		return plural(age/604800, "week") + " ago"
	case age < 31622400:
		return plural(age/2635200, "month") + " ago"
	}
	return plural(age/31622400, "year") + " ago"
}

// Live returns the human live status.
func (e *Entry) Live() string {
	switch e.LiveStatus {
	case "is_live":
		return "Online"
	case "was_live":
		return "Offline"
	case "is_upcoming":
		return "Upcoming"
	}
	return "False"
}
