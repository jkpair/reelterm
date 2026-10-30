package theme

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestAllBuiltinsParse(t *testing.T) {
	names := List(t.TempDir())
	if len(names) < 20 {
		t.Fatalf("expected the built-in themes, got %d", len(names))
	}
	for _, n := range names {
		th, err := Load(n, t.TempDir())
		if err != nil {
			t.Fatalf("%s: %v", n, err)
		}
		if _, ok := th.Bg.(lipgloss.Color); !ok {
			t.Errorf("%s: missing fzf bg color", n)
		}
		if th.Icon("search") == "" {
			t.Errorf("%s: missing search icon", n)
		}
	}
}

func TestParseHexComment(t *testing.T) {
	th := Parse("x", "COLOR_RED='\\033[38;5;168m'        # #bf616a\nCOLOR_BLUE='\\033[38;5;67m'\n")
	if th.Red != lipgloss.Color("#bf616a") {
		t.Errorf("red = %v", th.Red)
	}
	if th.Blue != lipgloss.Color("67") {
		t.Errorf("blue = %v", th.Blue)
	}
}

func TestTerminalColors(t *testing.T) {
	th := Parse("x", "FZF_COLORS='--color=fg:#4c4f69,bg:#eff1f5'\n")
	if got, want := th.TerminalColors(), "\x1b]10;#4c4f69\x1b\\\x1b]11;#eff1f5\x1b\\"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := Parse("x", "").TerminalColors(); got != "" {
		t.Errorf("no fzf colors: got %q", got)
	}
}
