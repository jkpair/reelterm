package ytdlp

import (
	"strings"
	"unicode"
)

// invisible reports runes that terminals and width libraries disagree on:
// fillers that look blank but measure as wide, and zero-width and bidi
// controls. Emoji variation selectors are kept; the TUI measures emoji at
// their widest instead.
func invisible(r rune) bool {
	switch {
	case r == 0x3164, r == 0x115F, r == 0x1160, r == 0xFFA0: // Hangul fillers
		return true
	case r >= 0x200B && r <= 0x200F, r >= 0x202A && r <= 0x202E, r >= 0x2060 && r <= 0x206F:
		return true
	case r == 0xFEFF, r == 0x00AD, r == 0x034F, r == 0x180E:
		return true
	case r >= 0xE0000 && r <= 0xE007F: // tag characters
		return true
	}
	return false
}

// CleanText makes text safe to lay out in a terminal grid: control
// characters become spaces (newlines are kept if multiline), invisible
// runes are dropped and runs of spaces collapse.
func CleanText(s string, multiline bool) string {
	var b strings.Builder
	b.Grow(len(s))
	space := false
	for _, r := range s {
		switch {
		case r == '\n' && multiline:
			b.WriteRune('\n')
			space = false
			continue
		case r == '\t', r == '\n', r == '\r', r == '\v', r == '\f', unicode.IsControl(r):
			r = ' '
		case invisible(r):
			continue
		}
		if r == ' ' {
			if space {
				continue
			}
			space = true
		} else {
			space = false
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

// Clean sanitises the displayed text fields in place.
func (e *Entry) Clean() {
	// A title that was only invisible characters stays blank on purpose.
	e.Title = CleanText(e.Title, false)
	e.Channel = CleanText(e.Channel, false)
	e.Uploader = CleanText(e.Uploader, false)
	if e.Description != nil {
		d := CleanText(*e.Description, true)
		e.Description = &d
	}
}
