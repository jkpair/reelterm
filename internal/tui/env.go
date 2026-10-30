// Package tui is the Bubble Tea interface.
package tui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/jkpair/reelterm/internal/app"
	"github.com/jkpair/reelterm/internal/cache"
	"github.com/jkpair/reelterm/internal/config"
	"github.com/jkpair/reelterm/internal/download"
	"github.com/jkpair/reelterm/internal/player"
	"github.com/jkpair/reelterm/internal/theme"
	"github.com/jkpair/reelterm/internal/thumb"
	"github.com/jkpair/reelterm/internal/ytdlp"
)

// Hooks lets the plugin layer extend menus without the tui importing it.
type Hooks struct {
	MenuItems    func() []PluginItem
	VideoActions func() []PluginItem
	Emit         func(event string, e *ytdlp.Entry)
}

// PluginItem is a menu entry contributed by a plugin. Run receives the
// selected video for video actions (nil for main-menu items).
type PluginItem struct {
	Label string
	Icon  string
	Run   func(e *ytdlp.Entry) PluginResult
}

// PluginResult tells the TUI what to do after a plugin callback.
type PluginResult struct {
	Toast   string
	Title   string
	Entries []ytdlp.Entry
	URL     string
}

// Env bundles everything screens need.
type Env struct {
	Paths     app.Paths
	Cfg       *config.Config
	Theme     *theme.Theme
	S         Styles
	YT        *ytdlp.Client
	Store     *cache.Store
	Player    *player.Player
	Thumbs    *thumb.Manager
	Downloads *download.Manager
	Hooks     Hooks
	Header    string
	// BodyTop is the screen row where the current screen's body starts.
	BodyTop int
}

// SetTheme swaps the active theme and rebuilds styles.
func (e *Env) SetTheme(t *theme.Theme) {
	e.Theme = t
	e.S = NewStyles(t, e.Cfg.PrettyPrint)
}

// Styles are the lipgloss styles derived from a theme.
type Styles struct {
	T           *theme.Theme
	Fg          lipgloss.Style
	Dim         lipgloss.Style
	Header      lipgloss.Style
	Prompt      lipgloss.Style
	Pointer     lipgloss.Style
	Selected    lipgloss.Style
	Match       lipgloss.Style
	Section     lipgloss.Style
	SectionRule lipgloss.Style
	Border      lipgloss.Style
	Title       lipgloss.Style
	Label       lipgloss.Style
	Info        lipgloss.Style
	Warn        lipgloss.Style
	Error       lipgloss.Style
	OK          lipgloss.Style
	Status      lipgloss.Style
	Key         lipgloss.Style
}

func NewStyles(t *theme.Theme, color bool) Styles {
	n := lipgloss.NewStyle()
	if !color {
		return Styles{T: t, Fg: n, Dim: n, Header: n, Prompt: n, Pointer: n, Selected: n.Reverse(true), Match: n.Underline(true),
			Section: n, SectionRule: n, Border: n, Title: n.Bold(true), Label: n.Bold(true), Info: n, Warn: n, Error: n, OK: n, Status: n, Key: n.Bold(true)}
	}
	return Styles{
		T:           t,
		Fg:          n.Foreground(t.Fg),
		Dim:         n.Foreground(t.Dim),
		Header:      n.Foreground(t.Header),
		Prompt:      n.Foreground(t.Prompt).Bold(true),
		Pointer:     n.Foreground(t.Pointer).Background(t.BgSel),
		Selected:    n.Foreground(t.FgSel).Background(t.BgSel).Bold(true),
		Match:       n.Foreground(t.Highlight).Bold(true),
		Section:     n.Foreground(t.Dim),
		SectionRule: n.Foreground(t.Gray),
		Border:      n.Foreground(t.Border),
		Title:       n.Foreground(t.Cyan).Bold(true),
		Label:       n.Foreground(t.Magenta).Bold(true),
		Info:        n.Foreground(t.Info),
		Warn:        n.Foreground(t.Yellow).Bold(true),
		Error:       n.Foreground(t.Red).Bold(true),
		OK:          n.Foreground(t.Green).Bold(true),
		Status:      n.Foreground(t.Dim),
		Key:         n.Foreground(t.Blue).Bold(true),
	}
}

// StatusStyle colors an availability status.
func (s Styles) StatusStyle(status string) lipgloss.Style {
	switch status {
	case "members_only", "age_restricted", "login_required":
		return s.Warn
	}
	return s.Error
}
