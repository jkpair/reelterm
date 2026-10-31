package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jkpair/reelterm/internal/youtube"
	"github.com/jkpair/reelterm/internal/ytdlp"
)

var mainDescs = map[string]string{
	"Your Feed":         "Browse your YouTube home feed",
	"Search":            "Search YouTube for videos, channels, and more",
	"Playlists":         "View and manage your YouTube playlists",
	"Watch Later":       "Videos saved to your Watch Later list",
	"Liked Videos":      "Videos you've liked on YouTube",
	"Saved Videos":      "Locally saved videos for offline browsing",
	"Watch History":     "Your YouTube watch history",
	"Recent":            "Recently played videos",
	"Subscription Feed": "Latest videos from your subscribed channels",
	"Channel Feed":      "Latest videos from your local channels (no sign-in needed)",
	"Channels":          "Browse and manage your subscribed channels",
	"Custom Playlists":  "User-created custom playlists",
	"Clips":             "Browse your YouTube clips",
	"Downloads":         "Download queue and channel/playlist/search downloaders",
	"Settings":          "Configure reelterm preferences, themes, and more",
	"Exit":              "Quit reelterm",
}

// NewMainMenu builds the root menu.
func NewMainMenu(env *Env) *Menu {
	t := env.Theme
	ic := env.Theme.Icon
	build := func() []Item {
		t = env.Theme
		items := []Item{
			{Icon: ic("home"), IconColor: t.Cyan, Label: "Your Feed"},
			{Icon: ic("search"), IconColor: t.Cyan, Label: "Search"},
			section("Library"),
			{Icon: ic("playlist"), IconColor: t.Blue, Label: "Playlists"},
			{Icon: ic("watch_later"), IconColor: t.Blue, Label: "Watch Later"},
			{Icon: ic("liked"), IconColor: t.Blue, Label: "Liked Videos"},
			{Icon: ic("saved"), IconColor: t.Blue, Label: "Saved Videos"},
			{Icon: ic("history"), IconColor: t.Blue, Label: "Watch History"},
			{Icon: ic("recent"), IconColor: t.Blue, Label: "Recent"},
			section("Subscriptions"),
			{Icon: ic("subscription"), IconColor: t.Magenta, Label: "Subscription Feed"},
			{Icon: ic("rss"), IconColor: t.Magenta, Label: "Channel Feed"},
			{Icon: ic("channel"), IconColor: t.Magenta, Label: "Channels"},
			section("Custom"),
			{Icon: ic("custom"), IconColor: t.Orange, Label: "Custom Playlists"},
			{Icon: ic("clips"), IconColor: t.Orange, Label: "Clips"},
			{Icon: ic("download"), IconColor: t.Orange, Label: "Downloads"},
		}
		if env.Hooks.MenuItems != nil {
			if extra := env.Hooks.MenuItems(); len(extra) > 0 {
				items = append(items, section("Plugins"))
				for i, p := range extra {
					items = append(items, Item{Icon: p.Icon, IconColor: t.Yellow, Label: p.Label, Value: fmt.Sprint("plugin:", i)})
				}
			}
		}
		items = append(items, section("Settings"),
			Item{Icon: ic("settings"), IconColor: t.Green, Label: "Settings"},
			Item{Icon: ic("exit"), IconColor: t.Red, Label: "Exit"},
		)
		for i := range items {
			if d, ok := mainDescs[items[i].Label]; ok {
				items[i].Desc = d
			}
		}
		return items
	}
	m := NewMenu(env, "Select Action", build, func(it Item) tea.Cmd { return mainAction(env, it) })
	m.onBack = func() tea.Cmd { return push(NewConfirm(env, "Are you sure you want to exit?", quit)) }
	return m
}

func listOf(env *Env, title, url string, kind ListKind) tea.Cmd {
	return push(NewVideoList(env, Source{Title: title, URL: url, Kind: kind}))
}

func mainAction(env *Env, it Item) tea.Cmd {
	if strings.HasPrefix(it.Value, "plugin:") {
		var i int
		fmt.Sscan(strings.TrimPrefix(it.Value, "plugin:"), &i)
		return runPlugin(env, env.Hooks.MenuItems()[i], nil)
	}
	switch it.Label {
	case "Your Feed":
		return listOf(env, "Feed", youtube.Home, KindHome)
	case "Search":
		return push(NewSearchPrompt(env, "Enter term to search for", func(q string) tea.Cmd {
			return replace(searchList(env, q))
		}))
	case "Subscription Feed":
		if !env.YT.HasCookies() {
			return push(NewChannelFeed(env))
		}
		return listOf(env, "Subscriptions", youtube.Subscriptions, KindFeed)
	case "Channel Feed":
		return push(NewChannelFeed(env))
	case "Playlists":
		return push(NewPlaylistList(env, "Playlists", youtube.Playlists, 0))
	case "Saved Videos":
		saved := env.Store.Saved()
		if len(saved) == 0 {
			return toast("No saved videos")
		}
		return push(NewVideoList(env, Source{Title: "Saved Videos", Kind: KindStatic, Entries: saved}))
	case "Recent":
		recent := env.Store.Recent()
		if len(recent) == 0 {
			return toast("No recent videos")
		}
		return push(NewVideoList(env, Source{Title: "Recent", Kind: KindStatic, Entries: recent}))
	case "Channels":
		return push(NewSubscriptionsList(env))
	case "Custom Playlists":
		return push(NewCustomPlaylists(env))
	case "Liked Videos":
		return listOf(env, "Liked Videos", youtube.Liked, KindPlaylist)
	case "Watch Later":
		return listOf(env, "Watch Later", youtube.WatchLater, KindPlaylist)
	case "Watch History":
		return listOf(env, "Watch History", youtube.History, KindFeed)
	case "Clips":
		return listOf(env, "Clips", youtube.Clips, KindFeed)
	case "Downloads":
		return push(NewDownloadsMenu(env))
	case "Settings":
		return push(NewSettings(env))
	case "Exit":
		return quit()
	}
	return nil
}

// searchList records history and opens results for q.
func searchList(env *Env, q string) *VideoList {
	if env.Cfg.SearchHistory {
		term, _ := youtube.ParseQuery(q)
		_ = env.Store.AddHistory(term)
	}
	if env.Hooks.Emit != nil {
		env.Hooks.Emit("search", &ytdlp.Entry{Title: q})
	}
	return NewVideoList(env, Source{Title: "Search: " + q, URL: youtube.SearchURL(q), Kind: KindPlaylist})
}

// NewCustomPlaylists lists custom_playlists.json.
func NewCustomPlaylists(env *Env) Screen {
	lists := env.Store.CustomPlaylists()
	if len(lists) == 0 {
		return NewMessage(env, "Custom Playlists", "You don't have any custom playlists.\nCreate them from a playlist's action menu (Save Playlist) or edit:\n"+env.Paths.CustomLists)
	}
	return NewMenu(env, "Select Custom Playlist To Play", func() []Item {
		var items []Item
		for _, p := range env.Store.CustomPlaylists() {
			items = append(items, Item{Icon: env.Theme.Icon("playlist"), IconColor: env.Theme.Orange, Label: p.Name, Value: p.PlaylistWatchURL, Desc: p.PlaylistURL})
		}
		return append(items, section(""), Item{Icon: env.Theme.Icon("back"), IconColor: env.Theme.Red, Label: "Back", Value: "back"})
	}, func(it Item) tea.Cmd {
		if it.Value == "back" {
			return pop()
		}
		return push(NewVideoList(env, Source{Title: it.Label, URL: it.Value, URLForAll: it.Desc, Kind: KindPlaylist}))
	})
}

// Message is a simple read-only screen.
type Message struct {
	env         *Env
	title, body string
}

func NewMessage(env *Env, title, body string) *Message { return &Message{env, title, body} }
func (m *Message) Init() tea.Cmd                       { return nil }
func (m *Message) Help() string                        { return hints(m.env, "esc", "back") }
func (m *Message) Update(msg tea.Msg) (Screen, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok && (k.String() == "esc" || k.String() == "enter" || k.String() == "q") {
		return m, pop()
	}
	return m, nil
}
func (m *Message) View(w, h int) string {
	return "\n" + m.env.S.Title.Render(" "+m.title) + "\n\n" + m.env.S.Fg.Render(indent(wrap(m.body, w-4), " "))
}

func indent(s, pad string) string {
	return pad + strings.ReplaceAll(s, "\n", "\n"+pad)
}

// runPlugin executes a plugin callback and acts on its result.
func runPlugin(env *Env, p PluginItem, e *ytdlp.Entry) tea.Cmd {
	return tea.Batch(toast("Running "+p.Label+"…"), func() tea.Msg {
		return pluginResultMsg{p.Run(e)}
	})
}

type pluginResultMsg struct{ r PluginResult }

func handlePluginResult(env *Env, r PluginResult) tea.Cmd {
	var cmds []tea.Cmd
	if r.Toast != "" {
		cmds = append(cmds, toast(r.Toast))
	}
	switch {
	case len(r.Entries) > 0:
		cmds = append(cmds, push(NewVideoList(env, Source{Title: r.Title, Kind: KindStatic, Entries: r.Entries})))
	case r.URL != "":
		cmds = append(cmds, push(NewVideoList(env, Source{Title: r.Title, URL: r.URL, Kind: KindPlaylist})))
	}
	return tea.Batch(cmds...)
}
