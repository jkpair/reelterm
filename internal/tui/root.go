package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jkpair/reelterm/internal/config"
	"github.com/jkpair/reelterm/internal/download"
	"github.com/jkpair/reelterm/internal/theme"
	"github.com/jkpair/reelterm/internal/thumb"
)

// Screen is one entry in the navigation stack.
type Screen interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (Screen, tea.Cmd)
	View(w, h int) string
}

// Helper is implemented by screens that show key hints.
type Helper interface{ Help() string }

// Navigation and notification messages handled by Root.
type (
	pushMsg    struct{ s Screen }
	replaceMsg struct{ s Screen }
	popMsg     struct{ n int }
	homeMsg    struct{}
	quitMsg    struct{}
	focusMsg   struct{}
	toastMsg   struct {
		text string
		err  bool
	}
	toastExpireMsg struct{ id int }
)

func push(s Screen) tea.Cmd    { return func() tea.Msg { return pushMsg{s} } }
func replace(s Screen) tea.Cmd { return func() tea.Msg { return replaceMsg{s} } }
func pop() tea.Cmd             { return func() tea.Msg { return popMsg{1} } }
func home() tea.Cmd            { return func() tea.Msg { return homeMsg{} } }
func quit() tea.Cmd            { return func() tea.Msg { return quitMsg{} } }
func toast(s string) tea.Cmd   { return func() tea.Msg { return toastMsg{text: s} } }
func toastErr(err error) tea.Cmd {
	return func() tea.Msg { return toastMsg{text: err.Error(), err: true} }
}

// Root owns the screen stack, header and status line.
type Root struct {
	env     *Env
	stack   []Screen
	w, h    int
	toast   toastMsg
	toastID int
	Quitted bool

	confirmingExit bool

	// Startup runs once after the first screen initialises (e.g. --search).
	Startup []tea.Cmd
}

func NewRoot(env *Env, first Screen) *Root {
	return &Root{env: env, stack: []Screen{first}}
}

func (r *Root) top() Screen { return r.stack[len(r.stack)-1] }

func (r *Root) Init() tea.Cmd {
	cmds := []tea.Cmd{r.top().Init(), tea.SetWindowTitle(r.env.Paths.Name), r.env.Downloads.Wait()}
	if r.env.Thumbs != nil {
		cmds = append(cmds, r.env.Thumbs.Wait())
	}
	return tea.Batch(append(cmds, tea.Sequence(r.Startup...))...)
}

func (r *Root) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		r.w, r.h = m.Width, m.Height
		if r.env.Thumbs != nil {
			r.env.Thumbs.Invalidate()
		}
	case tea.KeyMsg:
		if m.String() == "ctrl+c" {
			return r, r.exit()
		}
		s, cmd := r.top().Update(m)
		r.stack[len(r.stack)-1] = s
		return r, cmd
	case pushMsg:
		r.stack = append(r.stack, m.s)
		return r, tea.Batch(m.s.Init(), r.sizeCmd())
	case replaceMsg:
		r.stack[len(r.stack)-1] = m.s
		return r, tea.Batch(m.s.Init(), r.sizeCmd())
	case popMsg:
		r.confirmingExit = false
		n := min(m.n, len(r.stack)-1)
		if n <= 0 {
			return r, r.exit()
		}
		r.stack = r.stack[:len(r.stack)-n]
		return r, r.focus()
	case homeMsg:
		r.stack = r.stack[:1]
		return r, r.focus()
	case quitMsg:
		return r, r.exit()
	case toastMsg:
		r.toast = m
		r.toastID++
		id := r.toastID
		d := time.Duration(r.env.Cfg.NotificationDuration) * time.Second
		return r, tea.Tick(d, func(time.Time) tea.Msg { return toastExpireMsg{id} })
	case toastExpireMsg:
		if m.id == r.toastID {
			r.toast = toastMsg{}
		}
		return r, nil
	case download.Msg:
		var cmds []tea.Cmd
		if m.Done {
			if m.Err != nil {
				cmds = append(cmds, toast("Download failed: "+m.Title+": "+m.Err.Error()))
			} else {
				cmds = append(cmds, toast("Completed downloading of "+m.Title))
			}
		}
		cmds = append(cmds, r.env.Downloads.Wait())
		cmds = append(cmds, r.broadcast(msg))
		return r, tea.Batch(cmds...)
	case thumb.ReadyMsg:
		return r, tea.Batch(r.broadcast(msg), r.env.Thumbs.Wait())
	case playerExitMsg:
		return r, handlePlayerExit(r.env, m)
	case runPlayerMsg:
		return r, runPlayer(r.env, m.req)
	case pluginResultMsg:
		return r, handlePluginResult(r.env, m.r)
	case configReloadMsg:
		return r, r.reloadConfig()
	}
	if thumb.IsDraw(msg) {
		r.env.Thumbs.HandleDraw(msg)
		return r, nil
	}
	return r, r.broadcast(msg)
}

func (r *Root) reloadConfig() tea.Cmd {
	cfg, _, err := config.Load(r.env.Cfg.Path())
	if err != nil {
		return toastErr(err)
	}
	*r.env.Cfg = *cfg
	r.env.YT.Browser, r.env.YT.CookieFile = cfg.PreferredBrowser, cfg.CookieFile
	r.env.SetTheme(theme.MustLoad(cfg.Theme, r.env.Paths.ThemesDir))
	return tea.Batch(toast("Config reloaded"), r.focus())
}

// broadcast sends non-key messages to every screen so background results
// (thumbnails, fetches) reach screens that are not on top.
func (r *Root) broadcast(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	for i, s := range r.stack {
		ns, cmd := s.Update(msg)
		r.stack[i] = ns
		cmds = append(cmds, cmd)
	}
	return tea.Batch(cmds...)
}

func (r *Root) focus() tea.Cmd {
	s, cmd := r.top().Update(focusMsg{})
	r.stack[len(r.stack)-1] = s
	return tea.Batch(cmd, r.sizeCmd(), tea.ClearScreen)
}

func (r *Root) sizeCmd() tea.Cmd {
	w, h := r.w, r.h
	return func() tea.Msg { return tea.WindowSizeMsg{Width: w, Height: h} }
}

func (r *Root) exit() tea.Cmd {
	if n := r.env.Downloads.Active(); n > 0 && !r.confirmingExit {
		r.confirmingExit = true
		return push(NewConfirm(r.env, fmt.Sprintf("%d downloads are still running. Quit and cancel them?", n), func() tea.Cmd {
			return r.forceExit()
		}))
	}
	return r.forceExit()
}

func (r *Root) forceExit() tea.Cmd {
	r.Quitted = true
	r.env.Downloads.KillAll()
	r.env.Player.KillAll()
	if r.env.Thumbs != nil {
		r.env.Thumbs.Close()
	}
	return tea.Quit
}

func (r *Root) headerView() string {
	if r.h < 34 || r.w < 44 || r.env.Header == "" {
		title := r.env.S.Title.Render(" " + r.env.Paths.Name)
		return title + r.env.S.Dim.Render("  Kurl_Marx Edition") + "\n"
	}
	return r.env.S.Header.Render(r.env.Header) + "\n"
}

func (r *Root) statusView() string {
	s := r.env.S
	var left string
	switch {
	case r.toast.text != "" && r.toast.err:
		left = s.Error.Render(" " + r.toast.text)
	case r.toast.text != "":
		left = s.OK.Render(" " + r.toast.text)
	default:
		if h, ok := r.top().(Helper); ok {
			left = " " + h.Help()
		}
	}
	right := ""
	if r.env.Downloads != nil {
		if sum := r.env.Downloads.Summary(); sum != "" {
			right = s.Info.Render(r.env.S.T.Icon("download")+" "+sum) + " "
		}
	}
	// Last line of the screen: never let it reach the final column, or a
	// mis-measured character would scroll the whole display.
	avail := r.w - 1
	gap := avail - cells(left) - cells(right)
	if gap < 1 {
		left = truncate(left, max(avail-cells(right)-1, 0))
		gap = max(avail-cells(left)-cells(right), 0)
	}
	return left + strings.Repeat(" ", gap) + right
}

func (r *Root) View() string {
	if r.Quitted || r.w == 0 {
		return ""
	}
	header := r.headerView()
	status := r.statusView()
	headerH := strings.Count(header, "\n")
	bodyH := max(r.h-headerH-1, 1)
	r.env.BodyTop = headerH
	if r.env.Thumbs != nil {
		r.env.Thumbs.BeginFrame()
		defer r.env.Thumbs.EndFrame()
	}
	// Lines are fitted by hand: lipgloss would pad lines containing
	// zero-width cursor-forward escapes (thumbnail overlays) and wrap them.
	lines := strings.Split(r.top().View(r.w, bodyH), "\n")
	if len(lines) > bodyH {
		lines = lines[:bodyH]
	}
	for len(lines) < bodyH {
		lines = append(lines, "")
	}
	return header + strings.Join(lines, "\n") + "\n" + status
}
