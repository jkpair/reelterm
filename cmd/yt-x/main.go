// Command yt-x browses YouTube from the terminal.
package main

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jkpair/yt-x-plus/internal/app"
	"github.com/jkpair/yt-x-plus/internal/cache"
	"github.com/jkpair/yt-x-plus/internal/config"
	"github.com/jkpair/yt-x-plus/internal/deps"
	"github.com/jkpair/yt-x-plus/internal/download"
	"github.com/jkpair/yt-x-plus/internal/player"
	"github.com/jkpair/yt-x-plus/internal/plugin"
	"github.com/jkpair/yt-x-plus/internal/selfupdate"
	"github.com/jkpair/yt-x-plus/internal/theme"
	"github.com/jkpair/yt-x-plus/internal/thumb"
	"github.com/jkpair/yt-x-plus/internal/tui"
	"github.com/jkpair/yt-x-plus/internal/ytdlp"
)

const header = `╭──────────────────────────────────────╮
│                                      │
│   ▄▄▄       ▄▄▄▄▄▄▄                  │
│  █▀██  ██  █▀▀██▀▀▀▀                 │
│    ██  ██     ██                     │
│    ██  ██     ██          ▀██ ██▀    │
│    ██  ██     ██     ▀▀▀▀   ███      │
│    ▀█████▄    ▀██▄        ▄██ ██▄    │
│    ▄   ██                            │
│    ▀████▀                            │
│                                      │
│      Kurl_Marx Edition  v%-12s│
╰──────────────────────────────────────╯`

type options struct {
	search     string
	hasSearch  bool
	plugins    []string
	editConfig bool
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	paths := app.NewPaths()
	if err := paths.Ensure(); err != nil {
		return err
	}
	cfg, firstRun, err := config.Load(paths.ConfigFile)
	if err != nil {
		return err
	}
	opts, exit, err := parseArgs(os.Args[1:], cfg, paths)
	if err != nil || exit {
		return err
	}
	if opts.editConfig {
		cmd, err := player.EditorCmd(cfg.Editor, paths.ConfigFile)
		if err != nil {
			return err
		}
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		return cmd.Run()
	}

	if missing := deps.Missing(false); len(missing) > 0 {
		deps.Report(os.Stderr)
		fmt.Fprintln(os.Stderr)
		if err := deps.Install(os.Stdin, os.Stderr, false); err != nil {
			return fmt.Errorf("missing dependencies: %w (run %s --check-deps)", err, paths.Name)
		}
	}

	store := cache.New(paths)
	store.Cleanup(7 * 24 * time.Hour)
	yt := ytdlp.New(cfg)
	pl := player.New(cfg, yt)
	dl := download.New()
	proto := thumb.Detect(cfg.ImageRenderer)
	thumbs := thumb.New(proto, store.ThumbPath, os.Stdout, 8)

	env := &tui.Env{
		Paths:     paths,
		Cfg:       cfg,
		YT:        yt,
		Store:     store,
		Player:    pl,
		Thumbs:    thumbs,
		Downloads: dl,
		Header:    fmt.Sprintf(header, app.Version),
	}
	env.SetTheme(theme.MustLoad(cfg.Theme, paths.ThemesDir))

	rt := plugin.New(cfg, yt, pl, dl.Start)
	defer rt.Close()
	var pluginErrs []string
	for _, name := range append(append([]string{}, cfg.AutoLoadedPlugins...), opts.plugins...) {
		p, err := plugin.Resolve(paths.PluginsDir, name)
		if err == nil {
			err = rt.Load(p)
		}
		if err != nil {
			pluginErrs = append(pluginErrs, err.Error())
		}
	}
	env.Hooks = rt.Hooks()

	var first tui.Screen = tui.NewMainMenu(env)
	root := tui.NewRoot(env, first)
	var startup []tea.Cmd
	if opts.hasSearch {
		startup = append(startup, tui.Push(tui.SearchList(env, opts.search)))
	}
	if cfg.WelcomeScreen && dueMonthly(filepath.Join(paths.CacheDir, ".last_welcome")) {
		startup = append(startup, tui.Push(tui.NewWelcome(env)))
	}
	if firstRun {
		startup = append(startup, tui.Push(tui.NewBrowserPicker(env, "yt-dlp needs browser cookies to access YouTube feeds, subscriptions, and age-restricted content.")))
	}
	for _, e := range pluginErrs {
		startup = append(startup, tui.Toast(e))
	}
	root.Startup = startup

	p := tea.NewProgram(root, tea.WithAltScreen(), tea.WithMouseCellMotion())
	thumbs.Send = p.Send
	if _, err := p.Run(); err != nil {
		return err
	}
	byebye(env)
	return nil
}

func dueMonthly(stamp string) bool {
	data, _ := os.ReadFile(stamp)
	last, _ := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	now := time.Now().Unix()
	if now-last < 30*24*3600 {
		return false
	}
	_ = os.WriteFile(stamp, []byte(strconv.FormatInt(now, 10)), 0o644)
	return true
}

func byebye(env *tui.Env) {
	name := "User"
	if u, err := user.Current(); err == nil && u.Username != "" {
		name = u.Username
	}
	s := env.S
	l1, l2 := "Thanks for using yt-x", "Have a good day, "+name
	w := max(len([]rune(l1)), len([]rune(l2)))
	border := strings.Repeat("─", w+4)
	fmt.Println(s.Title.Render("┌" + border + "┐"))
	fmt.Println(s.Title.Render("│") + "  " + s.Dim.Render("Thanks for using ") + s.Fg.Bold(true).Render("yt-x") + strings.Repeat(" ", w-len([]rune(l1))) + "  " + s.Title.Render("│"))
	fmt.Println(s.Title.Render("│") + "  " + s.Dim.Render("Have a good day, ") + name + strings.Repeat(" ", w-len([]rune(l2))) + "  " + s.Title.Render("│"))
	fmt.Println(s.Title.Render("└" + border + "┘"))
}

func parseArgs(args []string, cfg *config.Config, paths app.Paths) (opts options, exit bool, err error) {
	need := func(i int, flag string) (string, error) {
		if i+1 >= len(args) || args[i+1] == "" {
			return "", fmt.Errorf("%s requires a value (see --help)", flag)
		}
		return args[i+1], nil
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "-h", "--help":
			usage(paths)
			return opts, true, nil
		case "-v", "--version":
			fmt.Printf("%s v%s Copyright © 2024 %s projects\n", paths.Name, app.Version, app.Author)
			return opts, true, nil
		case "-e", "--edit-config":
			opts.editConfig = true
		case "-U", "--update":
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			tag, err := selfupdate.Update(ctx, app.Repo, paths.Name, app.Version)
			switch {
			case err != nil:
				return opts, true, err
			case tag == "":
				fmt.Println("yt-x is up to date :)")
			default:
				fmt.Println("Updated to", tag)
			}
			return opts, true, nil
		case "-S", "--search":
			v, err := need(i, a)
			if err != nil {
				return opts, true, err
			}
			opts.search, opts.hasSearch = v, true
			i++
		case "-p", "--player":
			v, err := need(i, a)
			if err != nil {
				return opts, true, err
			}
			cfg.Player = v
			i++
		case "-x", "--extension", "--plugin":
			v, err := need(i, a)
			if err != nil {
				return opts, true, err
			}
			opts.plugins = append(opts.plugins, v)
			i++
		case "--preview":
			cfg.EnablePreview = true
		case "--no-preview":
			cfg.EnablePreview = false
		case "--theme":
			v, err := need(i, a)
			if err != nil {
				return opts, true, err
			}
			cfg.Theme = v
			i++
		case "--list-themes":
			fmt.Println("Available themes:")
			for _, t := range theme.List(paths.ThemesDir) {
				if t == cfg.Theme {
					fmt.Printf("  * %s (current)\n", t)
				} else {
					fmt.Printf("    %s\n", t)
				}
			}
			return opts, true, nil
		case "--check-deps":
			if !deps.Report(os.Stdout) {
				return opts, true, fmt.Errorf("required dependencies are missing (run %s --install)", paths.Name)
			}
			fmt.Println("\nImage protocol:", thumb.Detect(cfg.ImageRenderer))
			return opts, true, nil
		case "--install":
			return opts, true, deps.Install(os.Stdin, os.Stdout, true)
		case "--install-self":
			dir := "/usr/local/bin"
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				dir = args[i+1]
				i++
			}
			return opts, true, deps.InstallSelf(dir, os.Stdout)
		case "--disown-streaming-process":
			cfg.DisownStreaming = true
		case "--no-disown-streaming-process":
			cfg.DisownStreaming = false
		case "--disown-downloading-process", "--no-disown-downloading-process":
			// Downloads now run inside yt-x with a progress queue.
		case "-s", "--preferred-selector", "--rofi-theme":
			v, err := need(i, a)
			if err != nil {
				return opts, true, err
			}
			if v != "fzf" {
				fmt.Fprintln(os.Stderr, "note: rofi support was removed; yt-x now has a built-in interface")
			}
			i++
		case "-E", "--generate-desktop-entry":
			exe, _ := os.Executable()
			home, _ := os.UserHomeDir()
			fmt.Printf("[Desktop Entry]\nName=%s\nType=Application\nVersion=%s\nPath=%s\nComment=Browse Youtube from the terminal\nTerminal=true\nExec=%s\nCategories=AudioVideo;Video;\n",
				paths.Name, app.Version, home, exe)
			return opts, true, nil
		case "completions":
			shell := ""
			if i+1 < len(args) {
				shell = args[i+1]
			}
			out, err := completions(paths.Name, shell)
			if err != nil {
				return opts, true, err
			}
			fmt.Print(out)
			return opts, true, nil
		default:
			usage(paths)
			return opts, true, fmt.Errorf("unknown argument %q", a)
		}
	}
	return opts, false, nil
}

func usage(paths app.Paths) {
	n := paths.Name
	fmt.Printf(`Browse YouTube from the terminal

Usage: %[1]s [arguments] [options]

Commandline options override the config

Options:
  -S, --search <terms>          search for a video (supports :filters, e.g. ":week lofi")
  -e, --edit-config             edit the %[1]s config file
  -p, --player <mpv|vlc>        set the video player
  -x, --extension <plugin>      load a Lua plugin (name in %[2]s or full path)
  --preview / --no-preview      enable or disable thumbnails and the preview pane
  --theme <name>                set the color theme
  --list-themes                 list all available themes and exit
  --disown-streaming-process    keep the player running after %[1]s exits
  --no-disown-streaming-process stop the player when %[1]s exits
  -E, --generate-desktop-entry  print a desktop entry and exit
  --check-deps                  check dependencies and the image protocol
  --install                     install missing dependencies
  --install-self [path]         install %[1]s to a directory (default: /usr/local/bin)
  -U, --update                  update to the latest release
  -h, --help                    show this help message and exit
  -v, --version                 print the version and exit

Arguments:
  completions --fish|--bash|--zsh
    print shell completions

Examples:
  %[1]s --search ":today linux news"
  %[1]s completions --fish > ~/.config/fish/completions/%[1]s.fish
`, n, paths.PluginsDir)
}
