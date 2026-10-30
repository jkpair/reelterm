package youtube

import "testing"

func TestParseQuery(t *testing.T) {
	cases := []struct{ in, term, sp string }{
		{"lofi beats", "lofi beats", ""},
		{":week lofi", "lofi", "EgIIAw%253D%253D"},
		{":4k nature", "nature", "EgJwAQ%253D%253D"},
		{":bogus x", "x", "EgIQAQ%253D%253D"},
		{":week", ":week", ""},
	}
	for _, c := range cases {
		term, sp := ParseQuery(c.in)
		if term != c.term || sp != c.sp {
			t.Errorf("ParseQuery(%q) = %q,%q want %q,%q", c.in, term, sp, c.term, c.sp)
		}
	}
}

func TestSearchURL(t *testing.T) {
	want := "https://www.youtube.com/results?search_query=a+b%26c&sp=CAISAhAB"
	if got := SearchURL(":newest a b&c"); got != want {
		t.Errorf("got %s", got)
	}
}
