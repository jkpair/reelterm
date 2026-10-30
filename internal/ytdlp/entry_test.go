package ytdlp

import (
	"encoding/json"
	"testing"
	"time"
)

const fixture = `{"id":"PL1","title":"x","entries":[
 {"_type":"url","id":"abc","url":"https://www.youtube.com/watch?v=abc","title":"Hello","channel":"Chan",
  "view_count":1234567,"duration":3725,"timestamp":1700000000,"thumbnails":[{"url":"//i.ytimg.com/a.jpg"},{"url":"//i.ytimg.com/b.jpg"}]},
 {"id":"def","title":"[Private video]","duration":null,"availability":"private"}]}`

func TestDecode(t *testing.T) {
	var pl Playlist
	if err := json.Unmarshal([]byte(fixture), &pl); err != nil {
		t.Fatal(err)
	}
	e := pl.Entries[0]
	if e.Views() != "1,234,567" || e.DurationText() != "1 hour" || e.Clock() != "1:02:05" {
		t.Errorf("views=%s dur=%s clock=%s", e.Views(), e.DurationText(), e.Clock())
	}
	if e.ThumbnailURL() != "https://i.ytimg.com/b.jpg" {
		t.Errorf("thumb=%s", e.ThumbnailURL())
	}
	if got := e.Uploaded(time.Unix(1700000000+3*86400, 0)); got != "3 days ago" {
		t.Errorf("uploaded=%s", got)
	}
	p := pl.Entries[1]
	if p.Status() != "private" || p.Views() != "N/A" || p.DurationText() != "Unknown" || !p.NeedsEnrich() {
		t.Errorf("private entry: %s %s %s", p.Status(), p.Views(), p.DurationText())
	}
}

func TestRelative(t *testing.T) {
	cases := map[int64]string{30: "just now", 60: "1 minute ago", 7199: "1 hour ago", 7200: "2 hours ago", 86400 * 8: "1 week ago", 31622400 * 2: "2 years ago"}
	for in, want := range cases {
		if got := Relative(in); got != want {
			t.Errorf("Relative(%d)=%q want %q", in, got, want)
		}
	}
}

func TestCommas(t *testing.T) {
	for in, want := range map[int64]string{0: "0", 999: "999", 1000: "1,000", -1234: "-1,234"} {
		if got := Commas(in); got != want {
			t.Errorf("Commas(%d)=%s", in, got)
		}
	}
}

func TestKind(t *testing.T) {
	for u, want := range map[string]string{
		"https://www.youtube.com/watch?v=abc":            "video",
		"https://www.youtube.com/watch?v=abc&list=RDabc": "video",
		"https://www.youtube.com/channel/UCx":            "channel",
		"https://www.youtube.com/@DistroTube":            "channel",
		"https://www.youtube.com/playlist?list=PL1":      "playlist",
		"https://www.youtube.com/shorts/abc":             "video",
	} {
		e := Entry{URL: u}
		if got := e.Kind(); got != want {
			t.Errorf("Kind(%s)=%s want %s", u, got, want)
		}
	}
}
