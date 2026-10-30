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
