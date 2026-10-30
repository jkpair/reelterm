// Package theme parses the shell-variable *.theme files used by the Bash
// version and turns them into lipgloss styles.
package theme

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/jkpair/reelterm/themes"
)

const Default = "everforest-dark-hard"

// Theme holds the resolved palette, icons and box glyphs.
type Theme struct {
	Name string
	Key  string

	Red, Green, Yellow, Blue, Magenta, Cyan, Orange, Gray, White, Dim lipgloss.TerminalColor

	// From the FZF_COLORS block.
	Fg, FgSel, Bg, BgSel, Highlight, Prompt, Pointer, Header, Border, Info lipgloss.TerminalColor

	Icons map[string]string
	Box   map[string]string
}

var (
	assignRe = regexp.MustCompile(`^([A-Z_][A-Z0-9_]*)=(.*)$`)
	ansi256  = regexp.MustCompile(`38;5;(\d+)m`)
	hexRe    = regexp.MustCompile(`#[0-9a-fA-F]{6}`)
	fzfPair  = regexp.MustCompile(`([a-z+-]+):(#[0-9a-fA-F]{6})`)
)

// parseVars extracts VAR='value' assignments, including multi-line quoted values.
// The trailing "# comment" is kept separately since theme files note hex colors there.
func parseVars(data string) (vals, comments map[string]string) {
	vals, comments = map[string]string{}, map[string]string{}
	sc := bufio.NewScanner(strings.NewReader(data))
	for sc.Scan() {
		m := assignRe.FindStringSubmatch(strings.TrimSpace(sc.Text()))
		if m == nil {
			continue
		}
		key, rest := m[1], m[2]
		if rest == "" {
			continue
		}
		q := rest[0]
		if q != '\'' && q != '"' {
			vals[key] = strings.Fields(rest)[0]
			continue
		}
		body := rest[1:]
		for !strings.ContainsRune(body, rune(q)) && sc.Scan() {
			body += "\n" + sc.Text()
		}
		end := strings.IndexByte(body, q)
		if end < 0 {
			continue
		}
		vals[key] = body[:end]
		if c, ok := strings.CutPrefix(strings.TrimSpace(body[end+1:]), "#"); ok {
			comments[key] = strings.TrimSpace(c)
		}
	}
	return vals, comments
}

func color(vals, comments map[string]string, key string, def lipgloss.TerminalColor) lipgloss.TerminalColor {
	if h := hexRe.FindString(comments[key]); h != "" {
		return lipgloss.Color(h)
	}
	if m := ansi256.FindStringSubmatch(vals[key]); m != nil {
		return lipgloss.Color(m[1])
	}
	return def
}

// Parse builds a Theme from file contents.
func Parse(key, data string) *Theme {
	v, c := parseVars(data)
	fzf := map[string]lipgloss.TerminalColor{}
	for _, m := range fzfPair.FindAllStringSubmatch(v["FZF_COLORS"], -1) {
		fzf[m[1]] = lipgloss.Color(m[2])
	}
	pick := func(k string, def lipgloss.TerminalColor) lipgloss.TerminalColor {
		if col, ok := fzf[k]; ok {
			return col
		}
		return def
	}
	t := &Theme{
		Key:     key,
		Name:    v["THEME_NAME"],
		Red:     color(v, c, "COLOR_RED", lipgloss.Color("167")),
		Green:   color(v, c, "COLOR_GREEN", lipgloss.Color("142")),
		Yellow:  color(v, c, "COLOR_YELLOW", lipgloss.Color("179")),
		Blue:    color(v, c, "COLOR_BLUE", lipgloss.Color("109")),
		Magenta: color(v, c, "COLOR_MAGENTA", lipgloss.Color("175")),
		Cyan:    color(v, c, "COLOR_CYAN", lipgloss.Color("108")),
		Orange:  color(v, c, "COLOR_ORANGE", lipgloss.Color("173")),
		Gray:    color(v, c, "COLOR_GRAY", lipgloss.Color("102")),
		White:   color(v, c, "COLOR_WHITE", lipgloss.Color("223")),
		Dim:     color(v, c, "COLOR_DIM", lipgloss.Color("245")),
		Icons:   map[string]string{},
		Box:     map[string]string{},
	}
	if t.Name == "" {
		t.Name = key
	}
	t.Fg = pick("fg", t.White)
	t.FgSel = pick("fg+", t.Fg)
	t.Bg = pick("bg", lipgloss.NoColor{})
	t.BgSel = pick("bg+", t.Gray)
	t.Highlight = pick("hl", t.Green)
	t.Prompt = pick("prompt", t.Red)
	t.Pointer = pick("pointer", t.Magenta)
	t.Header = pick("header", t.Blue)
	t.Border = pick("border", t.Gray)
	t.Info = pick("info", t.Cyan)
	for k, val := range v {
		if name, ok := strings.CutPrefix(k, "ICON_"); ok {
			t.Icons[strings.ToLower(name)] = val
		}
		if name, ok := strings.CutPrefix(k, "BOX_"); ok {
			t.Box[strings.ToLower(name)] = val
		}
	}
	for k, d := range defaultIcons {
		if t.Icons[k] == "" {
			t.Icons[k] = d
		}
	}
	return t
}

var defaultIcons = map[string]string{
	"home": "\U000F02DC", "search": "\uF002", "playlist": "\U000F0411", "watch_later": "\uF017",
	"liked": "\uF164", "saved": "\uF02E", "history": "\uF1DA", "recent": "\U000F02DA",
	"subscription": "\U000F0D40", "channel": "\U000F0448", "custom": "\uF0CA", "clips": "\uF03D",
	"settings": "\uF013", "config": "\uF044", "exit": "\U000F0206", "play": "\uF04B",
	"download": "\uF019", "audio": "\uF001", "back": "\U000F030D", "menu": "\U000F035C",
	"next": "\uF061", "previous": "\uF060", "reload": "\uF021",
}

// Icon returns an icon by lowercase name.
func (t *Theme) Icon(name string) string { return t.Icons[name] }

// Load finds a theme by name, user directory first, then built-ins.
func Load(name, userDir string) (*Theme, error) {
	if name == "" {
		name = Default
	}
	if data, err := os.ReadFile(filepath.Join(userDir, name+".theme")); err == nil {
		return Parse(name, string(data)), nil
	}
	data, err := fs.ReadFile(themes.FS, name+".theme")
	if err != nil {
		return nil, err
	}
	return Parse(name, string(data)), nil
}

// MustLoad loads name, falling back to the default theme.
func MustLoad(name, userDir string) *Theme {
	if t, err := Load(name, userDir); err == nil {
		return t
	}
	t, _ := Load(Default, userDir)
	return t
}

// List returns sorted, de-duplicated theme names from built-ins and userDir.
func List(userDir string) []string {
	set := map[string]bool{}
	entries, _ := fs.ReadDir(themes.FS, ".")
	for _, e := range entries {
		if n, ok := strings.CutSuffix(e.Name(), ".theme"); ok {
			set[n] = true
		}
	}
	files, _ := filepath.Glob(filepath.Join(userDir, "*.theme"))
	for _, f := range files {
		set[strings.TrimSuffix(filepath.Base(f), ".theme")] = true
	}
	out := make([]string, 0, len(set))
	for n := range set {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
