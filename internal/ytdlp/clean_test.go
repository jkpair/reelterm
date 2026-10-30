package ytdlp

import "testing"

func TestCleanText(t *testing.T) {
	cases := map[string]string{
		"ㅤㅤㅤ ㅤㅤ":                        "",
		"rain \U0001F327\uFE0F camping": "rain \U0001F327\uFE0F camping",
		"a\tb\r\nc":                     "a b c",
		"left‮right​x":                  "leftrightx",
		"  many    spaces  ":            "many spaces",
	}
	for in, want := range cases {
		if got := CleanText(in, false); got != want {
			t.Errorf("CleanText(%q) = %q, want %q", in, got, want)
		}
	}
	if got := CleanText("line1\r\nline2", true); got != "line1 \nline2" {
		t.Errorf("multiline = %q", got)
	}
	e := Entry{Title: "ㅤㅤ", Channel: "ㅤx"}
	e.Clean()
	if e.Title != "" || e.Channel != "x" {
		t.Errorf("entry = %q %q", e.Title, e.Channel)
	}
}
