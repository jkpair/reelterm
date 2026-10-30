package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Item is a menu row. Section rows are non-selectable dividers.
type Item struct {
	Icon      string
	IconColor lipgloss.TerminalColor
	Label     string
	Suffix    string
	Desc      string
	Section   bool
	Value     string
}

func section(label string) Item { return Item{Section: true, Label: label} }

// Menu is a filterable list (fzf-style: type to filter, arrows to move).
type Menu struct {
	env      *Env
	prompt   string
	build    func() []Item
	items    []Item
	visible  []int
	cursor   int
	offset   int
	filter   string
	onSelect func(Item) tea.Cmd
	onBack   func() tea.Cmd
	help     string
}

// NewMenu creates a menu. build is re-run whenever the menu regains focus,
// so labels like "Toggle Previews [ON]" stay current.
func NewMenu(env *Env, prompt string, build func() []Item, onSelect func(Item) tea.Cmd) *Menu {
	m := &Menu{env: env, prompt: prompt, build: build, onSelect: onSelect}
	m.rebuild()
	return m
}

func (m *Menu) rebuild() {
	m.items = m.build()
	m.applyFilter()
}

// reset clears the filter and rebuilds, for menus that stay open after a selection.
func (m *Menu) reset() {
	m.filter = ""
	m.rebuild()
}

func (m *Menu) applyFilter() {
	m.visible = m.visible[:0]
	q := strings.ToLower(m.filter)
	for i, it := range m.items {
		if q != "" && (it.Section || !strings.Contains(strings.ToLower(it.Label), q)) {
			continue
		}
		m.visible = append(m.visible, i)
	}
	m.cursor = min(m.cursor, max(len(m.visible)-1, 0))
	m.skip(1)
}

// skip moves the cursor off section rows in direction dir.
func (m *Menu) skip(dir int) {
	n := len(m.visible)
	for range n {
		if n == 0 || !m.items[m.visible[m.cursor]].Section {
			return
		}
		m.cursor = (m.cursor + dir + n) % n
	}
}

func (m *Menu) move(d int) {
	n := len(m.visible)
	if n == 0 {
		return
	}
	m.cursor = (m.cursor + d + n) % n
	m.skip(sign(d))
}

func sign(d int) int {
	if d < 0 {
		return -1
	}
	return 1
}

func (m *Menu) Selected() (Item, bool) {
	if len(m.visible) == 0 {
		return Item{}, false
	}
	it := m.items[m.visible[m.cursor]]
	return it, !it.Section
}

func (m *Menu) Init() tea.Cmd { return nil }

func (m *Menu) Help() string {
	if m.help != "" {
		return m.help
	}
	return hints(m.env, "enter", "select", "type", "filter", "esc", "back")
}

func (m *Menu) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case focusMsg:
		m.reset()
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "ctrl+p", "ctrl+k", "shift+tab":
			m.move(-1)
		case "down", "ctrl+n", "ctrl+j", "tab":
			m.move(1)
		case "pgup":
			m.move(-10)
		case "pgdown":
			m.move(10)
		case "home":
			m.cursor = 0
			m.skip(1)
		case "end":
			m.cursor = max(len(m.visible)-1, 0)
			m.skip(-1)
		case "enter", "right":
			if it, ok := m.Selected(); ok && m.onSelect != nil {
				return m, m.onSelect(it)
			}
		case "esc":
			if m.filter != "" {
				m.filter = ""
				m.applyFilter()
				return m, nil
			}
			if m.onBack != nil {
				return m, m.onBack()
			}
			return m, pop()
		case "backspace":
			if m.filter != "" {
				r := []rune(m.filter)
				m.filter = string(r[:len(r)-1])
				m.applyFilter()
			}
		case "ctrl+u":
			m.filter = ""
			m.applyFilter()
		default:
			if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
				m.filter += string(msg.Runes)
				m.cursor = 0
				m.applyFilter()
			}
		}
	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.move(-1)
		case tea.MouseButtonWheelDown:
			m.move(1)
		}
	}
	return m, nil
}

func (m *Menu) View(w, h int) string {
	s := m.env.S
	var b strings.Builder
	b.WriteString(promptLine(m.env, m.prompt, m.filter, w))
	b.WriteString("\n")

	desc := ""
	if it, ok := m.Selected(); ok {
		desc = it.Desc
	}
	descH := 0
	if desc != "" {
		descH = lipgloss.Height(desc) + 1
	}
	listH := max(h-1-descH, 1)
	m.offset = clampOffset(m.offset, m.cursor, listH, len(m.visible))
	for row := 0; row < listH; row++ {
		i := m.offset + row
		if i >= len(m.visible) {
			b.WriteString("\n")
			continue
		}
		b.WriteString(m.renderItem(m.items[m.visible[i]], i == m.cursor, w))
		b.WriteString("\n")
	}
	if desc != "" {
		b.WriteString(s.SectionRule.Render(strings.Repeat("─", w)) + "\n")
		b.WriteString(s.Dim.Render(wrap(desc, w-2)))
	}
	return b.String()
}

func (m *Menu) renderItem(it Item, sel bool, w int) string {
	s := m.env.S
	if it.Section {
		if it.Label == "" {
			return "  " + s.SectionRule.Render(strings.Repeat("┈", 26))
		}
		fill := max(16-lipgloss.Width(it.Label), 2)
		return "  " + s.SectionRule.Render("┈┈┈┈┈┈┈┈ ") + s.Section.Render(it.Label) + " " + s.SectionRule.Render(strings.Repeat("┈", fill))
	}
	icon := ""
	if it.Icon != "" {
		st := lipgloss.NewStyle()
		if it.IconColor != nil && m.env.Cfg.PrettyPrint {
			st = st.Foreground(it.IconColor)
		}
		icon = st.Render(it.Icon) + "  "
	}
	label := highlight(s, it.Label, m.filter, sel)
	suffix := ""
	if it.Suffix != "" {
		suffix = " " + s.Dim.Render(it.Suffix)
	}
	line := fit(icon+label+suffix, w-3, "…")
	if sel {
		pad := max(w-3-cells(line), 0)
		return s.Pointer.Render("▌ ") + s.Selected.Render(line+strings.Repeat(" ", pad))
	}
	return "  " + line
}

// highlight marks the first case-insensitive match of q in label.
func highlight(s Styles, label, q string, sel bool) string {
	base := s.Fg
	if sel {
		base = s.Selected
	}
	if q == "" {
		return base.Render(label)
	}
	i := strings.Index(strings.ToLower(label), strings.ToLower(q))
	if i < 0 {
		return base.Render(label)
	}
	match := s.Match
	if sel {
		match = match.Background(s.T.BgSel)
	}
	return base.Render(label[:i]) + match.Render(label[i:i+len(q)]) + base.Render(label[i+len(q):])
}

func promptLine(env *Env, prompt, filter string, w int) string {
	s := env.S
	return fit(s.Prompt.Render(" "+prompt+": ")+s.Fg.Render(filter)+s.Pointer.UnsetBackground().Render("█"), w-1, "")
}

func clampOffset(offset, cursor, height, n int) int {
	if cursor < offset {
		offset = cursor
	}
	if cursor >= offset+height {
		offset = cursor - height + 1
	}
	return max(min(offset, n-height), 0)
}

func hints(env *Env, kv ...string) string {
	s := env.S
	var parts []string
	for i := 0; i+1 < len(kv); i += 2 {
		parts = append(parts, s.Key.Render(kv[i])+" "+s.Dim.Render(kv[i+1]))
	}
	return strings.Join(parts, s.SectionRule.Render("  │  "))
}

func truncate(s string, w int) string { return fit(s, w, "…") }

// wrap word-wraps plain text to width w.
func wrap(text string, w int) string {
	if w < 10 {
		w = 10
	}
	return ansi.Wordwrap(text, w, "")
}
