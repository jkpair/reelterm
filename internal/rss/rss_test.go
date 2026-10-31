package rss

import (
	"testing"

	"github.com/jkpair/reelterm/internal/ytdlp"
)

const sample = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015" xmlns:media="http://search.yahoo.com/mrss/" xmlns="http://www.w3.org/2005/Atom">
 <title>Chan</title>
 <entry>
  <id>yt:video:abcdefghijk</id>
  <yt:videoId>abcdefghijk</yt:videoId>
  <yt:channelId>UCaaaaaaaaaaaaaaaaaaaaaa</yt:channelId>
  <title>First &amp; best</title>
  <link rel="alternate" href="https://www.youtube.com/watch?v=abcdefghijk"/>
  <author><name>Chan</name><uri>https://www.youtube.com/channel/UCaaaaaaaaaaaaaaaaaaaaaa</uri></author>
  <published>2026-10-29T12:00:00+00:00</published>
  <media:group>
   <media:title>First &amp; best</media:title>
   <media:thumbnail url="https://i1.ytimg.com/vi/abcdefghijk/hqdefault.jpg" width="480" height="360"/>
   <media:description>Line one
Line two</media:description>
   <media:community><media:statistics views="1234"/></media:community>
  </media:group>
 </entry>
 <entry>
  <yt:videoId>shortshort1</yt:videoId>
  <title>A short</title>
  <link rel="alternate" href="https://www.youtube.com/shorts/shortshort1"/>
  <published>bad date</published>
 </entry>
</feed>`

func TestParse(t *testing.T) {
	es, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if len(es) != 2 {
		t.Fatalf("got %d entries", len(es))
	}
	e := es[0]
	if e.ID != "abcdefghijk" || e.Title != "First & best" || e.Channel != "Chan" || e.ChannelID != "UCaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("%+v", e)
	}
	if e.WatchURL() != "https://www.youtube.com/watch?v=abcdefghijk" || e.ChannelBase() != "https://www.youtube.com/channel/UCaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("urls: %q %q", e.WatchURL(), e.ChannelBase())
	}
	if e.ViewCount == nil || *e.ViewCount != 1234 || e.Timestamp == nil || *e.Timestamp != 1793275200 {
		t.Fatalf("views/time: %v %v", e.ViewCount, e.Timestamp)
	}
	if e.Desc() != "Line one\nLine two" || e.ThumbnailURL() != "https://i1.ytimg.com/vi/abcdefghijk/hqdefault.jpg" {
		t.Fatalf("desc/thumb: %q %q", e.Desc(), e.ThumbnailURL())
	}
	if IsShort(e) || !IsShort(es[1]) || es[1].Timestamp != nil {
		t.Fatalf("short: %+v", es[1])
	}
}

func TestChannelID(t *testing.T) {
	const id = "UCZW5lIUz93q_aZIkJPAC0IQ"
	for _, tc := range []struct {
		e    ytdlp.Entry
		want string
	}{
		{ytdlp.Entry{ID: id}, id},
		{ytdlp.Entry{ID: "https://www.youtube.com/@x", ChannelID: id}, id},
		{ytdlp.Entry{ID: "x", URL: "https://www.youtube.com/channel/" + id}, id},
		{ytdlp.Entry{ID: "https://www.youtube.com/@x", URL: "https://www.youtube.com/@x"}, ""},
	} {
		if got := ChannelID(tc.e); got != tc.want {
			t.Errorf("%+v: got %q", tc.e, got)
		}
	}
}

func TestResolvePatternsPreferCanonical(t *testing.T) {
	page := []byte(`<a href="/channel/UCbbbbbbbbbbbbbbbbbbbbbb">other</a>` +
		`<link rel="canonical" href="https://www.youtube.com/channel/UCaaaaaaaaaaaaaaaaaaaaaa">`)
	for _, re := range pageIDRes {
		if m := re.FindSubmatch(page); m != nil {
			if string(m[1]) != "UCaaaaaaaaaaaaaaaaaaaaaa" {
				t.Fatalf("got %s", m[1])
			}
			return
		}
	}
	t.Fatal("no match")
}
