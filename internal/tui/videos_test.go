package tui

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/jkpair/yt-x-plus/internal/app"
	"github.com/jkpair/yt-x-plus/internal/cache"
	"github.com/jkpair/yt-x-plus/internal/config"
	"github.com/jkpair/yt-x-plus/internal/theme"
	"github.com/jkpair/yt-x-plus/internal/thumb"
	"github.com/jkpair/yt-x-plus/internal/ytdlp"
)

var cuf = regexp.MustCompile(`\x1b\[(\d+)C`)

// visibleWidth counts cells, including those skipped by cursor-forward.
func visibleWidth(line string) int {
	w := 0
	for _, m := range cuf.FindAllStringSubmatch(line, -1) {
		n, _ := strconv.Atoi(m[1])
		w += n
	}
	return w + ansi.StringWidth(line)
}

func testEnv(t *testing.T, proto thumb.Protocol) *Env {
	d := t.TempDir()
	cfg, _, err := config.Load(filepath.Join(d, "c.conf"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.EnablePreview = true
	p := app.Paths{ConfigDir: d, CacheDir: d, ThumbsDir: filepath.Join(d, "t"), PlaylistsDir: d}
	env := &Env{Paths: p, Cfg: cfg, Store: cache.New(p), Thumbs: thumb.New(proto, func(k string) string { return filepath.Join(d, k) }, nil, 0)}
	env.SetTheme(theme.MustLoad("", d))
	return env
}

func TestRowsNeverExceedWidth(t *testing.T) {
	dur := 754.0
	titles := []string{
		"ㅤㅤㅤㅤㅤ ㅤㅤㅤㅤㅤㅤㅤㅤㅤ",
		"Living Alone in Japan | Letting the Day Drift By — A 27-year-old tobacco shop clerk's day",
		"My New PERFECT TENT for RAIN Camping 🌧️ Inflatable Hot Tent | Cozy Solo ASMR",
		"lofi hip hop radio 📚 beats to relax/study to",
		"tab\there\r\nnewline",
		"[Private video]",
	}
	var es []ytdlp.Entry
	for i, ti := range titles {
		desc := "line one\r\nline two with a very long unbroken url https://example.com/" + strings.Repeat("x", 200)
		es = append(es, ytdlp.Entry{ID: strconv.Itoa(i), Title: ti, Channel: "ㅤㅤㅤᵒ Alone in Japan - Letting the Day Drift By", Duration: &dur, Description: &desc})
	}
	for _, proto := range []thumb.Protocol{thumb.None, thumb.Kitty} {
		env := testEnv(t, proto)
		l := NewVideoList(env, Source{Title: "Feed", Kind: KindStatic, Entries: es})
		for _, w := range []int{60, 100, 160, 211} {
			for cur := range es {
				l.cursor = cur
				for i, line := range strings.Split(l.View(w, 40), "\n") {
					if vw := visibleWidth(line); vw > w-1 {
						t.Errorf("proto=%s w=%d cursor=%d line %d is %d cells: %q", proto, w, cur, i, vw, line)
					}
				}
			}
		}
	}
}

func TestSafeWidth(t *testing.T) {
	cases := map[string]int{
		"abc":                     3,
		"ナイフ":                     6,
		"\U0001F327":              2, // text-default emoji: some terminals draw it wide
		"\U0001F327️":             2,
		"\U0001F1EF\U0001F1F5":    4, // flag
		"\U0001F511\U0001F327 (x": 7,
	}
	for in, want := range cases {
		if got := safeWidth(in); got != want {
			t.Errorf("safeWidth(%q) = %d, want %d", in, got, want)
		}
	}
	if got := fit("ab\U0001F511cd", 3, ""); got != "ab" {
		t.Errorf("fit = %q", got)
	}
}
