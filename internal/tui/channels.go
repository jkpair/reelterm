package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jkpair/reelterm/internal/youtube"
	"github.com/jkpair/reelterm/internal/ytdlp"
)

// NewChannelList shows channels; selecting one opens its action menu.
func NewChannelList(env *Env, src Source) *VideoList {
	l := NewVideoList(env, src)
	l.itemKind = "channel"
	l.onSelect = func(_ *VideoList, e ytdlp.Entry) tea.Cmd { return push(NewChannelActions(env, e)) }
	return l
}

// NewSubscriptionsList shows subscriptions.json, importing from YouTube on first use.
func NewSubscriptionsList(env *Env) Screen {
	if subs, ok := env.Store.Subscriptions(); ok && len(subs) > 0 {
		return NewChannelList(env, Source{Title: "Select Channel", Kind: KindStatic, Entries: subs})
	}
	if !env.YT.HasCookies() {
		return NewMessage(env, "Channels", "No local channels yet, and no browser is configured to import your YouTube subscriptions.\n\nSet a browser in Settings → Select Browser, or subscribe to channels from a video's action menu.")
	}
	return NewLoader(env, "Loading subscriptions…", func(ctx context.Context) (Screen, error) {
		pl, err := env.YT.Fetch(ctx, youtube.Channels, ytdlp.Range{})
		if err != nil {
			return nil, err
		}
		_ = env.Store.SetSubscriptions(pl.Entries)
		return NewChannelList(env, Source{Title: "Select Channel", Kind: KindStatic, Entries: pl.Entries}), nil
	})
}

// NewChannelActions is channels_explorer's tab menu.
func NewChannelActions(env *Env, ch ytdlp.Entry) *Menu {
	t := env.Theme
	base := ch.ChannelBase()
	name := ch.ChannelName()
	if name == "" {
		name = ch.Title
	}
	return NewMenu(env, name, func() []Item {
		return []Item{
			{Icon: "\U000F0567", IconColor: t.Cyan, Label: "Videos"},
			{Icon: "\U000F0A49", IconColor: t.Cyan, Label: "Featured"},
			{Icon: env.Theme.Icon("search"), IconColor: t.Cyan, Label: "Search"},
			section(""),
			{Icon: env.Theme.Icon("playlist"), IconColor: t.Blue, Label: "Playlists"},
			{Icon: "\U000F1513", IconColor: t.Blue, Label: "Shorts"},
			{Icon: "\U000F083F", IconColor: t.Blue, Label: "Streams"},
			{Icon: "\U000F0994", IconColor: t.Blue, Label: "Podcasts"},
			section(""),
			{Icon: env.Theme.Icon("subscription"), IconColor: t.Magenta, Label: "Subscribe"},
			{Icon: env.Theme.Icon("download"), IconColor: t.Orange, Label: "Download Channel", Desc: "Download every video from this channel (background)"},
			{Icon: env.Theme.Icon("back"), IconColor: t.Red, Label: "Back"},
			{Icon: env.Theme.Icon("exit"), IconColor: t.Red, Label: "Exit"},
		}
	}, func(it Item) tea.Cmd {
		switch it.Label {
		case "Videos", "Shorts", "Streams":
			return push(NewVideoList(env, Source{Title: name + " · " + it.Label, URL: base + "/" + lower(it.Label), Kind: KindPlaylist}))
		case "Featured", "Playlists", "Podcasts":
			return push(NewPlaylistList(env, name+" · "+it.Label, base+"/"+lower(it.Label), 0))
		case "Search":
			return push(NewPrompt(env, "Search "+name, "", func(q string) tea.Cmd {
				return replace(NewVideoList(env, Source{Title: name + " · " + q, URL: youtube.ChannelTabSearch(base, q), Kind: KindPlaylist}))
			}))
		case "Subscribe":
			if err := env.Store.Subscribe(ch); err != nil {
				return toastErr(err)
			}
			return toast("Successfully subscribed to " + name)
		case "Download Channel":
			return downloadChannel(env, name, base)
		case "Back":
			return pop()
		case "Exit":
			return quit()
		}
		return nil
	})
}

func lower(s string) string {
	b := []byte(s)
	if len(b) > 0 && b[0] >= 'A' && b[0] <= 'Z' {
		b[0] += 'a' - 'A'
	}
	return string(b)
}

// NewPlaylistList shows playlists (playlists_explorer); limit>0 caps results.
func NewPlaylistList(env *Env, title, url string, limit int) *VideoList {
	l := NewVideoList(env, Source{Title: title, URL: url, Kind: KindPlaylist})
	l.itemKind = "playlist"
	l.onSelect = func(_ *VideoList, e ytdlp.Entry) tea.Cmd {
		return push(NewVideoList(env, Source{Title: e.Title, URL: e.WatchURL(), Kind: KindPlaylist}))
	}
	return l
}

// Loader shows a spinner while building the next screen.
type Loader struct {
	env    *Env
	label  string
	run    func(ctx context.Context) (Screen, error)
	cancel context.CancelFunc
	err    error
}

type loaderDoneMsg struct {
	l   *Loader
	s   Screen
	err error
}

func NewLoader(env *Env, label string, run func(ctx context.Context) (Screen, error)) *Loader {
	return &Loader{env: env, label: label, run: run}
}

func (l *Loader) Init() tea.Cmd {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	l.cancel = cancel
	return func() tea.Msg {
		s, err := l.run(ctx)
		return loaderDoneMsg{l, s, err}
	}
}

func (l *Loader) Help() string { return hints(l.env, "esc", "cancel") }

func (l *Loader) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch m := msg.(type) {
	case loaderDoneMsg:
		if m.l != l {
			return l, nil
		}
		if m.err != nil {
			l.err = m.err
			return l, nil
		}
		return l, replace(m.s)
	case tea.KeyMsg:
		if m.String() == "esc" || m.String() == "enter" {
			if l.cancel != nil {
				l.cancel()
			}
			return l, pop()
		}
	}
	return l, nil
}

func (l *Loader) View(w, h int) string {
	s := l.env.S
	if l.err != nil {
		return "\n " + s.Error.Render("Failed") + "\n\n " + s.Dim.Render(wrap(l.err.Error(), w-4))
	}
	return "\n " + s.Info.Render("⋯ ") + s.Dim.Render(l.label)
}
