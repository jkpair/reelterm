package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jkpair/yt-x-plus/internal/app"
	"github.com/jkpair/yt-x-plus/internal/player"
	"github.com/jkpair/yt-x-plus/internal/selfupdate"
	"github.com/jkpair/yt-x-plus/internal/theme"
	"github.com/jkpair/yt-x-plus/internal/youtube"
	"github.com/jkpair/yt-x-plus/internal/ytdlp"
)

var settingsDescs = map[string]string{
	"Toggle Previews":            "Enable or disable video thumbnail previews in lists",
	"Select Theme":               "Change the color theme for the interface",
	"Select Browser":             "Browser to read YouTube cookies from (feeds, subscriptions, age-restricted videos)",
	"Edit Config":                "Open the yt-x configuration file in your editor",
	"Explore Channels":           "Search for and browse YouTube channels",
	"Explore Playlists":          "Search for and browse YouTube playlists",
	"Search History":             "Browse and re-run previous searches",
	"New Custom Command":         "Create a new custom yt-dlp command for quick access",
	"Custom Commands":            "Run previously saved custom yt-dlp commands",
	"Edit Search History":        "Open search history file in your editor",
	"Edit Custom Playlists":      "Open custom playlists file in your editor",
	"Edit MPV Config":            "Open mpv player configuration in your editor",
	"Edit yt-dlp Config":         "Open yt-dlp configuration in your editor",
	"Edit Custom Commands":       "Open custom commands file in your editor",
	"Sync YouTube Subscriptions": "Import your YouTube subscriptions from your browser",
	"Check yt-dlp Version":       "Check if yt-dlp is up to date",
	"Check for Updates":          "Check if a new version of yt-x is available",
	"Clear Search History":       "Delete all saved search history",
	"Back":                       "Return to the main menu",
	"Exit":                       "Quit yt-x",
}

func onOff(b bool) string {
	if b {
		return "ON"
	}
	return "OFF"
}

// NewSettings is the settings menu.
func NewSettings(env *Env) *Menu {
	build := func() []Item {
		t := env.Theme
		browser := env.Cfg.PreferredBrowser
		if browser == "" {
			browser = "none"
		}
		items := []Item{
			{Icon: "\U000F02F2", IconColor: t.Green, Label: "Toggle Previews", Suffix: "[" + onOff(env.Cfg.EnablePreview) + "]"},
			{Icon: "\U000F050E", IconColor: t.Green, Label: "Select Theme", Suffix: "[" + env.Theme.Key + "]"},
			{Icon: "\U000F059F", IconColor: t.Green, Label: "Select Browser", Suffix: "[" + browser + "]"},
			{Icon: "\U000F03EB", IconColor: t.Green, Label: "Edit Config"},
			section("Explore"),
			{Icon: env.Theme.Icon("subscription"), IconColor: t.Cyan, Label: "Explore Channels"},
			{Icon: env.Theme.Icon("playlist"), IconColor: t.Cyan, Label: "Explore Playlists"},
			{Icon: "\U000F1622", IconColor: t.Cyan, Label: "Search History"},
			section("Commands"),
			{Icon: "\U000F01BA", IconColor: t.Orange, Label: "New Custom Command"},
			{Icon: "\U000F0866", IconColor: t.Orange, Label: "Custom Commands"},
			section("Edit"),
			{Icon: "\U000F12E2", IconColor: t.Blue, Label: "Edit Search History"},
			{Icon: "\U000F0900", IconColor: t.Blue, Label: "Edit Custom Playlists"},
			{Icon: "\U000F1122", IconColor: t.Blue, Label: "Edit MPV Config"},
			{Icon: "\U000F0B86", IconColor: t.Blue, Label: "Edit yt-dlp Config"},
			{Icon: "\U000F162B", IconColor: t.Blue, Label: "Edit Custom Commands"},
			section("Sync"),
			{Icon: "\U000F04E6", IconColor: t.Magenta, Label: "Sync YouTube Subscriptions"},
			{Icon: "\U000F03D7", IconColor: t.Magenta, Label: "Check yt-dlp Version"},
			{Icon: "\U000F06B0", IconColor: t.Magenta, Label: "Check for Updates"},
			{Icon: "\U000F01B4", IconColor: t.Red, Label: "Clear Search History"},
			section(""),
			{Icon: env.Theme.Icon("back"), IconColor: t.Red, Label: "Back"},
			{Icon: env.Theme.Icon("exit"), IconColor: t.Red, Label: "Exit"},
		}
		for i := range items {
			items[i].Desc = settingsDescs[items[i].Label]
		}
		return items
	}
	return NewMenu(env, "Settings", build, func(it Item) tea.Cmd { return settingsAction(env, it.Label) })
}

func editFile(env *Env, path string, after func() tea.Cmd) tea.Cmd {
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	cmd, err := player.EditorCmd(env.Cfg.Editor, path)
	if err != nil {
		return toastErr(err)
	}
	return tea.ExecProcess(cmd, func(error) tea.Msg {
		if after != nil {
			return after()()
		}
		return nil
	})
}

type configReloadMsg struct{}

func settingsAction(env *Env, action string) tea.Cmd {
	cfgDir := os.Getenv("XDG_CONFIG_HOME")
	if cfgDir == "" {
		cfgDir = app.ExpandHome("~/.config")
	}
	switch action {
	case "Toggle Previews":
		v := !env.Cfg.EnablePreview
		if err := env.Cfg.Set("ENABLE_PREVIEW", fmt.Sprint(v)); err != nil {
			return toastErr(err)
		}
		return tea.Batch(func() tea.Msg { return focusMsg{} }, toast("Previews "+map[bool]string{true: "enabled", false: "disabled"}[v]))
	case "Select Theme":
		return push(NewThemePicker(env))
	case "Select Browser":
		return push(NewBrowserPicker(env, "Select the browser to extract YouTube cookies from:"))
	case "Edit Config":
		return editFile(env, env.Cfg.Path(), func() tea.Cmd { return func() tea.Msg { return configReloadMsg{} } })
	case "Explore Channels":
		return push(NewPrompt(env, "What channel would you like to explore?", "", func(q string) tea.Cmd {
			return replace(NewChannelList(env, Source{Title: "Select Channel", URL: youtube.ChannelSearchURL(q), Kind: KindPlaylist}))
		}))
	case "Explore Playlists":
		return push(NewPrompt(env, "What playlist would you like to explore?", "", func(q string) tea.Cmd {
			return replace(NewPlaylistList(env, "Playlists: "+q, youtube.PlaylistSearchURL(q), env.Cfg.NoOfSearchResults))
		}))
	case "Search History":
		hist := env.Store.History()
		if len(hist) == 0 {
			return toast("No search history or disabled")
		}
		return push(NewMenu(env, "Search for", func() []Item {
			var items []Item
			for _, h := range env.Store.History() {
				items = append(items, Item{Icon: env.Theme.Icon("history"), IconColor: env.Theme.Cyan, Label: h})
			}
			return items
		}, func(it Item) tea.Cmd {
			return push(NewVideoList(env, Source{Title: "Search: " + it.Label, URL: youtube.HistorySearchURL(it.Label), Kind: KindPlaylist}))
		}))
	case "New Custom Command":
		def := "yt-dlp -J --flat-playlist --extractor-args youtubetab:approximate_date " + strings.Join(env.YT.CookieOpts(), " ") + " https://youtube.com"
		return push(NewPrompt(env, "Enter the custom yt-dlp command", def, func(cmdline string) tea.Cmd {
			return replace(NewPrompt(env, "Enter the name of the new custom command", "", func(name string) tea.Cmd {
				if err := env.Store.AddCustomCmd(name, cmdline); err != nil {
					return tea.Batch(pop(), toastErr(err))
				}
				return tea.Batch(toast("Successfully added to custom cmds"), replace(customCmdList(env, name, cmdline)))
			}))
		}))
	case "Custom Commands":
		cmds := env.Store.CustomCmds()
		if len(cmds) == 0 {
			return toast("You don't have any custom cmds. Create them with New Custom Command")
		}
		return push(NewMenu(env, "Select Custom Data Loader Command To Run", func() []Item {
			var items []Item
			for _, c := range env.Store.CustomCmds() {
				items = append(items, Item{Icon: "\U000F0866", IconColor: env.Theme.Orange, Label: c.Name, Value: c.Cmd, Desc: c.Cmd})
			}
			return items
		}, func(it Item) tea.Cmd { return push(customCmdList(env, it.Label, it.Value)) }))
	case "Edit Search History":
		return editFile(env, env.Paths.SearchHistory, nil)
	case "Edit Custom Playlists":
		return editFile(env, env.Paths.CustomLists, nil)
	case "Edit Custom Commands":
		return editFile(env, env.Paths.CustomCmds, nil)
	case "Edit MPV Config":
		return editFile(env, filepath.Join(cfgDir, "mpv", "mpv.conf"), nil)
	case "Edit yt-dlp Config":
		return editFile(env, filepath.Join(cfgDir, "yt-dlp", "config"), nil)
	case "Clear Search History":
		return push(NewConfirm(env, "Clear your search history?", func() tea.Cmd {
			if err := env.Store.ClearHistory(); err != nil {
				return toastErr(err)
			}
			return toast("Search history cleared")
		}))
	case "Sync YouTube Subscriptions":
		if !env.YT.HasCookies() {
			return toast("Failed to fetch subscriptions (please set a browser in Settings)")
		}
		return push(NewConfirm(env, "This will erase your local subs, proceed?", func() tea.Cmd {
			return tea.Batch(toast("Syncing subscriptions…"), func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
				defer cancel()
				pl, err := env.YT.Fetch(ctx, youtube.Channels, ytdlp.Range{})
				if err != nil {
					return toastMsg{text: "Failed to fetch subscriptions: " + err.Error(), err: true}
				}
				if err := env.Store.SetSubscriptions(pl.Entries); err != nil {
					return toastMsg{text: err.Error(), err: true}
				}
				return toastMsg{text: fmt.Sprintf("Successfully imported %d subscriptions", len(pl.Entries))}
			})
		}))
	case "Check yt-dlp Version":
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			local, err := env.YT.Version(ctx)
			if err != nil {
				return toastMsg{text: "yt-dlp not found: " + err.Error(), err: true}
			}
			latest, err := selfupdate.LatestTag(ctx, "yt-dlp/yt-dlp")
			switch {
			case err != nil:
				return toastMsg{text: "yt-dlp " + local + " (could not check latest: " + err.Error() + ")"}
			case latest == local:
				return toastMsg{text: "yt-dlp " + local + " is up to date"}
			}
			return toastMsg{text: "yt-dlp " + local + " is outdated (latest " + latest + "). Run: yt-dlp -U or update via your package manager", err: true}
		}
	case "Check for Updates":
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			latest, err := selfupdate.LatestTag(ctx, app.Repo)
			if err != nil {
				return toastMsg{text: "Update check failed: " + err.Error(), err: true}
			}
			if selfupdate.Newer(latest, app.Version) {
				return toastMsg{text: "Update available: " + latest + " (you have v" + app.Version + "). Run: " + env.Paths.Name + " --update"}
			}
			return toastMsg{text: "You are on the latest version (v" + app.Version + ")"}
		}
	case "Back":
		return pop()
	case "Exit":
		return quit()
	}
	return nil
}

func customCmdList(env *Env, name, cmdline string) *VideoList {
	args := strings.Fields(cmdline)
	if len(args) > 0 && (args[0] == "yt-dlp" || strings.HasSuffix(args[0], "/yt-dlp")) {
		args = args[1:]
	}
	return NewVideoList(env, Source{Title: name, Args: args, Kind: KindPlaylist})
}

// NewThemePicker lists themes grouped like the Bash version and applies live.
func NewThemePicker(env *Env) *Menu {
	build := func() []Item {
		var dark, light, contrast []Item
		for _, n := range theme.List(env.Paths.ThemesDir) {
			it := Item{Icon: "\U000F050E", Label: n, Value: n}
			if n == env.Theme.Key {
				it.Icon, it.Suffix, it.IconColor = "", "(current)", env.Theme.Green
			}
			switch {
			case strings.HasSuffix(n, "-light") || strings.HasSuffix(n, "-latte") || strings.HasSuffix(n, "-dawn"):
				if it.IconColor == nil {
					it.IconColor = env.Theme.Yellow
				}
				light = append(light, it)
			case n == "modus-vivendi" || n == "modus-operandi":
				if it.IconColor == nil {
					it.IconColor = env.Theme.Orange
				}
				contrast = append(contrast, it)
			default:
				if it.IconColor == nil {
					it.IconColor = env.Theme.Cyan
				}
				dark = append(dark, it)
			}
		}
		items := append([]Item{section("Dark")}, dark...)
		items = append(items, section("Light"))
		items = append(items, light...)
		items = append(items, section("High Contrast"))
		items = append(items, contrast...)
		return append(items, section(""), Item{Icon: env.Theme.Icon("back"), IconColor: env.Theme.Red, Label: "Back", Value: ""})
	}
	var m *Menu
	m = NewMenu(env, "Select Theme", build, func(it Item) tea.Cmd {
		if it.Value == "" {
			return pop()
		}
		t, err := theme.Load(it.Value, env.Paths.ThemesDir)
		if err != nil {
			return toastErr(err)
		}
		env.SetTheme(t)
		if err := env.Cfg.Set("THEME", it.Value); err != nil {
			return toastErr(err)
		}
		m.reset()
		return toast("Theme: " + t.Name)
	})
	return m
}

var browsers = []string{"firefox", "chromium", "chrome", "brave", "edge", "opera", "vivaldi", "safari", "zen", "librewolf"}

// NewBrowserPicker selects and verifies PREFERRED_BROWSER (select_browser).
func NewBrowserPicker(env *Env, question string) *Menu {
	return NewMenu(env, "Select your browser", func() []Item {
		var items []Item
		for _, b := range browsers {
			it := Item{Icon: "\U000F059F", IconColor: env.Theme.Cyan, Label: b, Value: b, Desc: question}
			if b == env.Cfg.PreferredBrowser {
				it.Suffix = "(current)"
			}
			items = append(items, it)
		}
		return append(items, section(""), Item{Icon: env.Theme.Icon("exit"), IconColor: env.Theme.Yellow, Label: "Skip (no cookies)", Value: "", Desc: "You can set it later in Settings or the config (PREFERRED_BROWSER / COOKIE_FILE)."})
	}, func(it Item) tea.Cmd {
		if it.Value == "" {
			return pop()
		}
		b := it.Value
		return tea.Batch(pop(), toast("Testing cookie extraction from "+b+"…"), func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			out, err := exec.CommandContext(ctx, env.YT.Bin, "--cookies-from-browser", b, "--dump-json", "--no-download", "--skip-download",
				"https://www.youtube.com/watch?v=jNQXAC9IVRw").CombinedOutput()
			if err != nil || !strings.Contains(string(out), `"id"`) {
				hint := "make sure " + b + " is installed, has visited YouTube, and is closed; you may need to unlock your keyring"
				return toastMsg{text: "Could not extract cookies from " + b + ": " + hint, err: true}
			}
			if err := env.Cfg.Set("PREFERRED_BROWSER", b); err != nil {
				return toastMsg{text: err.Error(), err: true}
			}
			env.YT.Browser = b
			return toastMsg{text: "Cookie extraction from " + b + " works!"}
		})
	})
}
