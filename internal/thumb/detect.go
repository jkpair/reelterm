package thumb

import (
	"os"
	"runtime"
	"strings"
	"time"

	"golang.org/x/term"
)

// Protocol is how images reach the terminal.
type Protocol int

const (
	None      Protocol = iota
	HalfBlock          // ▀ cells with truecolor, works everywhere
	Chafa              // external chafa binary, symbol output
	Kitty              // kitty graphics protocol (kitty, ghostty)
	Sixel              // DEC sixel (foot, wezterm, konsole, xterm -ti vt340, windows terminal)
	ITerm              // iTerm2 inline images (iTerm2, wezterm)
)

func (p Protocol) String() string {
	return [...]string{"none", "halfblock", "chafa", "kitty", "sixel", "iterm"}[p]
}

// Overlay reports whether the protocol draws outside the text grid.
func (p Protocol) Overlay() bool { return p == Kitty || p == Sixel || p == ITerm }

// Parse maps an IMAGE_RENDERER value to a protocol; "auto" returns ok=false.
func Parse(s string) (Protocol, bool) {
	switch strings.ToLower(s) {
	case "kitty", "icat":
		return Kitty, true
	case "sixel":
		return Sixel, true
	case "iterm", "iterm2", "imgcat":
		return ITerm, true
	case "halfblock", "blocks", "text":
		return HalfBlock, true
	case "chafa":
		return Chafa, true
	case "none", "off":
		return None, true
	}
	return None, false
}

// Detect picks the best protocol for the current terminal. It must run
// before Bubble Tea takes over stdin since it may query the terminal.
func Detect(setting string) Protocol {
	if p, ok := Parse(setting); ok {
		return p
	}
	if runtime.GOOS == "windows" {
		// Sixel needs Windows Terminal 1.22+ and the cell pixel size can't be
		// read, so images could be mis-sized; half-blocks always work.
		// IMAGE_RENDERER: sixel opts in.
		return HalfBlock
	}
	env := os.Getenv
	termName := env("TERM")
	prog := env("TERM_PROGRAM")
	if env("TMUX") != "" || strings.HasPrefix(termName, "screen") {
		// Multiplexers mangle graphics escapes without explicit passthrough.
		return HalfBlock
	}
	switch {
	case env("KITTY_WINDOW_ID") != "" || termName == "xterm-kitty":
		return Kitty
	case env("GHOSTTY_RESOURCES_DIR") != "" || env("GHOSTTY_BIN_DIR") != "" || prog == "ghostty" || termName == "xterm-ghostty":
		return Kitty
	case prog == "iTerm.app" || prog == "WezTerm":
		return ITerm
	case strings.HasPrefix(termName, "foot") || env("WT_SESSION") != "" || env("KONSOLE_VERSION") != "" ||
		strings.Contains(termName, "mlterm") || strings.Contains(termName, "contour"):
		return Sixel
	}
	if querySixel() {
		return Sixel
	}
	return HalfBlock
}

// querySixel asks the terminal for its primary device attributes and looks
// for capability 4 (sixel graphics).
func querySixel() bool {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return false
	}
	old, err := term.MakeRaw(fd)
	if err != nil {
		return false
	}
	defer term.Restore(fd, old)
	if _, err := os.Stdout.WriteString("\x1b[c"); err != nil {
		return false
	}
	resp := make(chan string, 1)
	go func() {
		var b []byte
		buf := make([]byte, 64)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				break
			}
			b = append(b, buf[:n]...)
			if strings.Contains(string(b), "c") {
				break
			}
		}
		resp <- string(b)
	}()
	select {
	case r := <-resp:
		r = strings.TrimSuffix(strings.TrimPrefix(r, "\x1b[?"), "c")
		for _, f := range strings.Split(r, ";") {
			if f == "4" {
				return true
			}
		}
	case <-time.After(200 * time.Millisecond):
	}
	return false
}
