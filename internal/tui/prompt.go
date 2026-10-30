package tui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// Prompt asks for a line of text.
type Prompt struct {
	env      *Env
	title    string
	input    textinput.Model
	extra    func() string
	onSubmit func(string) tea.Cmd
}

func NewPrompt(env *Env, title, initial string, onSubmit func(string) tea.Cmd) *Prompt {
	ti := textinput.New()
	ti.Prompt = ""
	ti.SetValue(initial)
	ti.CursorEnd()
	ti.Focus()
	ti.TextStyle = env.S.Fg
	ti.Cursor.Style = env.S.Pointer.UnsetBackground()
	return &Prompt{env: env, title: title, input: ti, onSubmit: onSubmit}
}

func (p *Prompt) Init() tea.Cmd { return textinput.Blink }

func (p *Prompt) Help() string {
	return hints(p.env, "enter", "confirm", "esc", "cancel")
}

func (p *Prompt) Update(msg tea.Msg) (Screen, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc":
			return p, pop()
		case "enter":
			v := strings.TrimSpace(p.input.Value())
			if v == "" {
				return p, nil
			}
			return p, p.onSubmit(v)
		}
	}
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return p, cmd
}

func (p *Prompt) View(w, h int) string {
	s := p.env.S
	p.input.Width = max(w-6, 10)
	var b strings.Builder
	b.WriteString("\n" + s.Title.Render(" "+p.title) + "\n\n")
	b.WriteString(s.Prompt.Render(" ❯ ") + p.input.View() + "\n\n")
	if p.extra != nil {
		b.WriteString(p.extra())
	}
	return b.String()
}

// NewSearchPrompt shows filters and recent history; "!N" picks history item N.
func NewSearchPrompt(env *Env, title string, onSubmit func(string) tea.Cmd) *Prompt {
	hist := env.Store.History()
	if len(hist) > 10 {
		hist = hist[:10]
	}
	p := NewPrompt(env, title, "", func(v string) tea.Cmd {
		if n, err := strconv.Atoi(strings.TrimPrefix(v, "!")); err == nil && strings.HasPrefix(v, "!") {
			if n >= 1 && n <= len(hist) {
				v = hist[n-1]
			} else {
				return toast("No such history item: " + strconv.Itoa(n))
			}
		}
		return onSubmit(v)
	})
	p.extra = func() string {
		s := env.S
		rule := func(t string) string {
			return " " + s.SectionRule.Render("┈┈┈┈┈┈┈┈┈ ") + s.Section.Render(t) + " " + s.SectionRule.Render("┈┈┈┈┈┈┈┈┈") + "\n"
		}
		var b strings.Builder
		b.WriteString(rule("Search Filters"))
		b.WriteString(" " + s.Info.Render("Date:   ") + " :hour  :today  :week  :month  :year\n")
		b.WriteString(" " + s.Info.Render("Type:   ") + " :video  :movie  :live  :short  :long  :playlist\n")
		b.WriteString(" " + s.Info.Render("Quality:") + " :hd  :4k  :hdr  :subtitles  :360  :vr\n")
		b.WriteString(" " + s.Info.Render("Sort:   ") + " :newest  :views  :rating\n")
		if len(hist) > 0 {
			b.WriteString(rule("Search History") + s.Dim.Render(" type !N to reuse") + "\n")
			for i, h := range hist {
				b.WriteString(s.Dim.Render(" "+pad2(i+1)+". ") + s.Fg.Render(h) + "\n")
			}
		}
		return b.String()
	}
	return p
}

func pad2(n int) string {
	if n < 10 {
		return " " + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

// NewConfirm shows a Yes/No menu.
func NewConfirm(env *Env, question string, onYes func() tea.Cmd) *Menu {
	m := NewMenu(env, question, func() []Item {
		return []Item{{Label: "Yes", Value: "y", Icon: "", IconColor: env.Theme.Green}, {Label: "No", Value: "n", IconColor: env.Theme.Red}}
	}, func(it Item) tea.Cmd {
		if it.Value == "y" {
			return tea.Sequence(pop(), onYes())
		}
		return pop()
	})
	m.cursor = 1
	return m
}
