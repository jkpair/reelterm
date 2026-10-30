package tui

import (
	"github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"
)

// Terminals disagree on how wide emoji, flags and pictographs are (e.g.
// Ghostty draws some that width tables call 1 as 2). A line that comes out
// wider than measured wraps and scrolls the whole screen, so layout uses
// cells(), which counts such graphemes at their widest. The worst case is a
// row ending a cell or two short.

// risky reports graphemes whose rendered width varies between terminals.
func risky(cluster string) bool {
	for _, r := range cluster {
		switch {
		case r >= 0x1F000, // emoji, pictographs, regional indicators
			r >= 0x2600 && r <= 0x27BF, // misc symbols, dingbats
			r >= 0x2B00 && r <= 0x2BFF, // arrows and symbols with emoji forms
			r == 0xFE0F, r == 0x200D:
			return true
		}
	}
	return false
}

// safeWidth is the conservative width of plain text.
func safeWidth(s string) int {
	total, state := 0, -1
	for len(s) > 0 {
		var cluster string
		var w int
		cluster, s, w, state = uniseg.FirstGraphemeClusterInString(s, state)
		if risky(cluster) {
			n := 0
			for _, r := range cluster {
				if r >= 0x1F1E6 && r <= 0x1F1FF { // each regional indicator may take 2
					n += 2
				}
			}
			w = max(w, 2, n)
		}
		total += w
	}
	return total
}

// cells is safeWidth for styled text.
func cells(s string) int { return safeWidth(ansi.Strip(s)) }

// fit truncates styled text so its conservative width is at most w.
func fit(s string, w int, tail string) string {
	if w <= 0 {
		return ""
	}
	for t := w; t > 0; t-- {
		out := ansi.Truncate(s, t, tail)
		if cells(out) <= w {
			return out
		}
	}
	return ""
}
