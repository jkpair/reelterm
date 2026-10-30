package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jkpair/yt-x-plus/internal/cache"
	"github.com/jkpair/yt-x-plus/internal/player"
	"github.com/jkpair/yt-x-plus/internal/youtube"
	"github.com/jkpair/yt-x-plus/internal/ytdlp"
)

type playerExitMsg struct {
	title  string
	stderr string
	err    error
}

// NewVideoActions is the per-video action menu (playlist_explorer's media actions).
func NewVideoActions(env *Env, l *VideoList, e ytdlp.Entry) *Menu {
	t := env.Theme
	build := func() []Item {
		saved := env.Store.IsSaved(e.ID)
		enum := "OFF"
		if l.enumerate {
			enum = "ON"
		}
		items := []Item{
			{Icon: "\U000F040E", IconColor: t.Green, Label: "Watch", Desc: "Stream the video in " + env.Cfg.Player},
			{Icon: "\U000F040A", IconColor: t.Green, Label: "Play All", Desc: "Play every video in this list"},
			section("Audio"),
			{Icon: "\U000F0386", IconColor: t.Cyan, Label: "Listen", Desc: "Play audio only in the terminal"},
			{Icon: "\U000F0CB8", IconColor: t.Cyan, Label: "Listen To All", Desc: "Play audio of every video in this list"},
			{Icon: "\U000F049F", IconColor: t.Cyan, Label: "Mix", Desc: "Browse the YouTube mix (radio) for this video"},
			section("Library"),
		}
		if saved {
			items = append(items, Item{Icon: "\U000F09CE", IconColor: t.Blue, Label: "UnSave", Desc: "Remove from Saved Videos"})
		} else {
			items = append(items, Item{Icon: "\U000F00C0", IconColor: t.Blue, Label: "Save", Desc: "Add to Saved Videos"})
		}
		items = append(items,
			Item{Icon: "\U000F0412", IconColor: t.Blue, Label: "Save Playlist", Desc: "Save this list as a custom playlist"},
			Item{Icon: "\U000F0D40", IconColor: t.Blue, Label: "Subscribe To Channel", Desc: "Add " + e.ChannelName() + " to your local channels"},
			section("Download"),
			Item{Icon: "\U000F1464", IconColor: t.Orange, Label: "Download", Desc: "Download to " + env.Cfg.DownloadDirectory + "/videos (runs in the background)"},
			Item{Icon: "\U000F0997", IconColor: t.Orange, Label: "Download All", Desc: "Download every video in this list"},
			Item{Icon: "\U000F1464", IconColor: t.Orange, Label: "Download (Audio Only)", Desc: "Download as mp3"},
			Item{Icon: "\U000F0997", IconColor: t.Orange, Label: "Download All (Audio Only)", Desc: "Download every video in this list as mp3"},
			section("Other"),
			Item{Icon: "\U000F059F", IconColor: t.Magenta, Label: "Open in Browser"},
			Item{Icon: "\U000F018F", IconColor: t.Magenta, Label: "Copy Link"},
			Item{Icon: "\U000F00FB", IconColor: t.Magenta, Label: "Toggle Enumerate Downloads", Suffix: "[" + enum + "]", Desc: "Prefix playlist downloads with their index"},
			Item{Icon: "\U000F018D", IconColor: t.Magenta, Label: "Shell", Desc: "Open a shell with this video's details in the environment"},
		)
		if env.Hooks.VideoActions != nil {
			if extra := env.Hooks.VideoActions(); len(extra) > 0 {
				items = append(items, section("Plugins"))
				for i, p := range extra {
					items = append(items, Item{Icon: p.Icon, IconColor: t.Yellow, Label: p.Label, Value: fmt.Sprint("plugin:", i)})
				}
			}
		}
		return append(items, section(""),
			Item{Icon: "\U000F030D", IconColor: t.Red, Label: "Back"},
			Item{Icon: "\U000F0206", IconColor: t.Red, Label: "Exit"},
		)
	}
	var m *Menu
	m = NewMenu(env, "Select Media Action", build, func(it Item) tea.Cmd {
		if strings.HasPrefix(it.Value, "plugin:") {
			var i int
			fmt.Sscan(strings.TrimPrefix(it.Value, "plugin:"), &i)
			return runPlugin(env, env.Hooks.VideoActions()[i], &e)
		}
		return videoAction(env, l, e, it.Label, m)
	})
	return m
}

func videoAction(env *Env, l *VideoList, e ytdlp.Entry, action string, m *Menu) tea.Cmd {
	switch action {
	case "Watch":
		return play(env, e, false)
	case "Listen":
		return play(env, e, true)
	case "Play All":
		return playAll(env, l, false)
	case "Listen To All":
		return playAll(env, l, true)
	case "Mix":
		id := strings.TrimPrefix(e.ID, "RD")
		return push(NewVideoList(env, Source{Title: "Mix: " + e.Title, URL: youtube.MixURL(id), URLForAll: youtube.MixURL(id), Kind: KindPlaylist}))
	case "Save":
		return saveVideo(env, e)
	case "UnSave":
		if err := env.Store.Unsave(e.ID); err != nil {
			return toastErr(err)
		}
		return toast("Removed from saved videos")
	case "Save Playlist":
		u := l.src.URLForAll
		if u == "" {
			u = l.src.URL
		}
		i := strings.Index(u, "list=")
		if i < 0 {
			return toast("This list is not a YouTube playlist")
		}
		pid := strings.SplitN(u[i+5:], "&", 2)[0]
		return push(NewPrompt(env, "Enter the name of the playlist", l.src.Title, func(name string) tea.Cmd {
			if err := env.Store.AddCustomPlaylist(name, pid); err != nil {
				return tea.Batch(pop(), toastErr(err))
			}
			return tea.Batch(pop(), toast("Successfully added to custom playlists"))
		}))
	case "Subscribe To Channel":
		if e.ChannelBase() == "" || e.ChannelName() == "" {
			return toast("Channel information is not available for this video")
		}
		ch := ytdlp.Entry{ID: e.ChannelID, Channel: e.ChannelName(), UploaderURL: e.ChannelBase(), URL: e.ChannelBase(), Title: e.ChannelName()}
		if ch.ID == "" {
			ch.ID = e.ChannelBase()
		}
		if err := env.Store.Subscribe(ch); err != nil {
			return toastErr(err)
		}
		return toast("Subscribed to " + ch.Channel)
	case "Download":
		return startDownloads(env, l, []ytdlp.Entry{e}, false)
	case "Download (Audio Only)":
		return startDownloads(env, l, []ytdlp.Entry{e}, true)
	case "Download All", "Download All (Audio Only)":
		audio := strings.Contains(action, "Audio")
		return push(NewPrompt(env, "Name of the playlist", cleanName(l.src.Title), func(name string) tea.Cmd {
			return tea.Batch(pop(), downloadAll(env, l, name, audio))
		}))
	case "Open in Browser":
		return openInBrowser(e.WatchURL())
	case "Copy Link":
		return copyLink(e.WatchURL())
	case "Toggle Enumerate Downloads":
		l.enumerate = !l.enumerate
		m.reset()
		return nil
	case "Shell":
		return openShell(env, l, e)
	case "Back":
		return pop()
	case "Exit":
		return quit()
	}
	return nil
}

// NewBatchActions acts on multi-selected videos.
func NewBatchActions(env *Env, l *VideoList, entries []ytdlp.Entry) *Menu {
	t := env.Theme
	n := len(entries)
	return NewMenu(env, fmt.Sprintf("%d videos selected", n), func() []Item {
		return []Item{
			{Icon: "\U000F040A", IconColor: t.Green, Label: "Play Selected"},
			{Icon: "\U000F0CB8", IconColor: t.Cyan, Label: "Listen To Selected"},
			{Icon: "\U000F0997", IconColor: t.Orange, Label: "Download Selected"},
			{Icon: "\U000F0997", IconColor: t.Orange, Label: "Download Selected (Audio Only)"},
			{Icon: "\U000F00C0", IconColor: t.Blue, Label: "Save Selected"},
			section(""),
			{Icon: "\U000F030D", IconColor: t.Red, Label: "Back"},
		}
	}, func(it Item) tea.Cmd {
		done := func(c tea.Cmd) tea.Cmd { l.marked = map[string]bool{}; return tea.Batch(pop(), c) }
		switch it.Label {
		case "Play Selected", "Listen To Selected":
			return done(playEntries(env, entries, fmt.Sprintf("selection-%d", time.Now().UnixNano()), it.Label != "Play Selected"))
		case "Download Selected", "Download Selected (Audio Only)":
			return done(startDownloads(env, l, entries, strings.Contains(it.Label, "Audio")))
		case "Save Selected":
			for _, e := range entries {
				_ = env.Store.Save(e)
			}
			return done(toast(fmt.Sprintf("Saved %d videos", n)))
		}
		return pop()
	})
}

func cleanName(s string) string {
	return strings.NewReplacer("/", "-", "\\", "-").Replace(strings.TrimSpace(s))
}

func saveVideo(env *Env, e ytdlp.Entry) tea.Cmd {
	if err := env.Store.Save(e); err != nil {
		return toastErr(err)
	}
	return toast("Saved: " + e.Title)
}

func openInBrowser(u string) tea.Cmd {
	return func() tea.Msg {
		if err := player.OpenURL(u); err != nil {
			return toastMsg{text: err.Error(), err: true}
		}
		return nil
	}
}

func copyLink(u string) tea.Cmd {
	return func() tea.Msg {
		if err := player.Copy(u); err != nil {
			return toastMsg{text: err.Error(), err: true}
		}
		return toastMsg{text: "Link copied to clipboard"}
	}
}

func unavailable(e ytdlp.Entry) tea.Cmd {
	st := e.Status()
	return func() tea.Msg {
		return toastMsg{text: "Video unavailable [" + ytdlp.StatusLabel(st) + "]: " + ytdlp.StatusHelp(st), err: true}
	}
}

// play streams one video. Video plays in the background (the player window
// has its own controls); audio runs in the foreground so mpv's terminal
// controls work.
func play(env *Env, e ytdlp.Entry, audio bool) tea.Cmd {
	if e.Status() != "available" {
		return unavailable(e)
	}
	if env.Cfg.UpdateRecent {
		_ = env.Store.AddRecent(e, env.Cfg.NoOfRecent)
	}
	if env.Hooks.Emit != nil {
		env.Hooks.Emit("play", &e)
	}
	url := e.WatchURL()
	if e.IsMix() {
		id := strings.TrimPrefix(e.ID, "RD")
		return playMix(env, youtube.MixURL(id), e.Title, audio)
	}
	return runPlayer(env, player.Request{URL: url, Title: e.Title, AudioOnly: audio})
}

func runPlayer(env *Env, req player.Request) tea.Cmd {
	cmd, err := env.Player.Command(context.Background(), req)
	if err != nil {
		return toastErr(err)
	}
	verb := "Now watching: "
	if req.AudioOnly {
		verb = "Now listening to: "
	}
	if req.AudioOnly && env.Cfg.Player == "mpv" {
		buf := player.Capture(cmd, os.Stderr)
		return tea.Sequence(toast(verb+req.Title), tea.ExecProcess(cmd, func(err error) tea.Msg {
			return playerExitMsg{title: req.Title, stderr: buf.String(), err: err}
		}))
	}
	buf := player.Capture(cmd, nil)
	if err := env.Player.Start(cmd, env.Cfg.DisownStreaming); err != nil {
		return toastErr(err)
	}
	wait := func() tea.Msg {
		err := cmd.Wait()
		return playerExitMsg{title: req.Title, stderr: buf.String(), err: err}
	}
	return tea.Batch(toast(verb+req.Title), wait)
}

// handlePlayerExit turns player failures into a message.
func handlePlayerExit(env *Env, m playerExitMsg) tea.Cmd {
	if m.err == nil {
		return nil
	}
	var ee *exec.ExitError
	if !errors.As(m.err, &ee) {
		return toastErr(m.err)
	}
	msg, bot := player.Classify(m.stderr)
	if bot {
		return tea.Batch(toast(msg), push(NewBrowserPicker(env, "YouTube requires authentication (bot detection). Select a browser to extract cookies from:")))
	}
	if msg == "" {
		return nil
	}
	return toast(msg)
}

func playMix(env *Env, mixURL, title string, audio bool) tea.Cmd {
	if p, ok := env.Store.HasM3U(mixURL); ok {
		return runPlayer(env, player.Request{URL: p, Title: title, AudioOnly: audio})
	}
	return func() tea.Msg {
		pl, err := env.YT.Fetch(context.Background(), mixURL, ytdlp.Range{})
		if err != nil {
			return toastMsg{text: "Failed to get mix data", err: true}
		}
		p, err := env.Store.M3U(mixURL, pl.Entries)
		if err != nil {
			return toastMsg{text: err.Error(), err: true}
		}
		return runPlayerMsg{player.Request{URL: p, Title: title, AudioOnly: audio}}
	}
}

type runPlayerMsg struct{ req player.Request }

// playAll mirrors "Play All": real playlists stream directly, everything
// else (search results, feeds, mixes) goes through a generated m3u.
func playAll(env *Env, l *VideoList, audio bool) tea.Cmd {
	u := l.src.URLForAll
	if u == "" {
		u = l.src.URL
	}
	isMix := strings.Contains(u, "list=RD")
	if isMix {
		u = strings.Replace(strings.Replace(u, "&start_radio=0", "", 1), "&start_radio=1", "", 1)
		return playMix(env, u, l.src.Title, audio)
	}
	if l.channelPL && u != "" && l.src.Kind == KindPlaylist {
		return runPlayer(env, player.Request{URL: u, Title: l.src.Title, AudioOnly: audio})
	}
	key := u
	if key == "" {
		key = l.src.Title
	}
	return playEntries(env, l.entries, key+fmt.Sprint(len(l.entries)), audio)
}

func playEntries(env *Env, entries []ytdlp.Entry, key string, audio bool) tea.Cmd {
	var playable []ytdlp.Entry
	for _, e := range entries {
		if e.Status() == "available" {
			playable = append(playable, e)
		}
	}
	if len(playable) == 0 {
		return toast("Nothing playable in this list")
	}
	_ = os.Remove(filepath.Join(env.Paths.PlaylistsDir, cache.Hash(key)+".m3u8"))
	p, err := env.Store.M3U(key, playable)
	if err != nil {
		return toastErr(err)
	}
	return runPlayer(env, player.Request{URL: p, Title: fmt.Sprintf("%d videos", len(playable)), AudioOnly: audio})
}

// startDownloads downloads individual videos in the background.
func startDownloads(env *Env, l *VideoList, entries []ytdlp.Entry, audio bool) tea.Cmd {
	var started int
	for _, e := range entries {
		if e.Status() != "available" {
			continue
		}
		var args []string
		if audio {
			args = []string{e.WatchURL(), "-x", "-f", "bestaudio", "--audio-format", "mp3",
				"--output", filepath.Join(env.Cfg.DownloadDirectory, "audio", "individual", "%(channel)s", "%(title)s.%(ext)s")}
			args = append(args, env.YT.BuildOpts(env.Cfg, ytdlp.ModeAudio)...)
		} else {
			args = []string{e.WatchURL(), "--output", filepath.Join(env.Cfg.DownloadDirectory, "videos", "individual", "%(channel)s", "%(title)s.%(ext)s")}
			args = append(args, env.YT.BuildOpts(env.Cfg, ytdlp.ModeDownload)...)
		}
		args = append(args, env.YT.CookieOpts()...)
		env.Downloads.Start(e.Title, args)
		if env.Hooks.Emit != nil {
			env.Hooks.Emit("download", &e)
		}
		started++
	}
	if started == 0 {
		return toast("Video unavailable")
	}
	if started == 1 {
		return toast("Downloading: " + entries[0].Title)
	}
	return toast(fmt.Sprintf("Downloading %d videos", started))
}

func downloadAll(env *Env, l *VideoList, name string, audio bool) tea.Cmd {
	u := l.src.URLForAll
	if u == "" {
		u = l.src.URL
	}
	if u == "" {
		return startDownloads(env, l, l.entries, audio)
	}
	enum := ""
	if l.enumerate {
		enum = "%(playlist_index)s - "
	}
	kind := "videos"
	if audio {
		kind = "audio"
	}
	out := filepath.Join(env.Cfg.DownloadDirectory, kind, name, "%(channel)s", enum+"%(title)s.%(ext)s")
	args := []string{u, "--output", out}
	if audio {
		args = append(args, "--audio-format", "mp3", "-x", "-f", "bestaudio/best")
		args = append(args, env.YT.BuildOpts(env.Cfg, ytdlp.ModeAudio)...)
	} else {
		args = append(args, env.YT.BuildOpts(env.Cfg, ytdlp.ModeDownload)...)
	}
	args = append(args, env.YT.CookieOpts()...)
	args = append(args, "--download-archive", filepath.Join(env.Paths.ArchiveDir, cache.Hash(strings.Join(args, " "))))
	env.Downloads.Start(name, args)
	return toast("Downloading playlist: " + name)
}

// openShell drops into the user's shell with the video's details exported.
func openShell(env *Env, l *VideoList, e ytdlp.Entry) tea.Cmd {
	sh := os.Getenv("SHELL")
	if sh == "" {
		sh = "bash"
	}
	banner := env.Paths.Name + " shell. Variables: $url $urlForAll $video_url $video_id $video_title $playlist_title $DOWNLOAD_DIRECTORY $CLI_YT_DLP_ARCHIVE. Type exit to return."
	cmd := exec.Command("sh", "-c", `clear; printf '%s\n\n' "$YTX_BANNER"; exec "$YTX_SHELL"`)
	cmd.Env = append(os.Environ(),
		"YTX_BANNER="+banner, "YTX_SHELL="+sh,
		"url="+l.src.URL, "urlForAll="+l.src.URLForAll, "video_url="+e.WatchURL(), "video_id="+e.ID,
		"video_title="+e.Title, "playlist_title="+l.src.Title, "DOWNLOAD_DIRECTORY="+env.Cfg.DownloadDirectory,
		"CLI_YT_DLP_ARCHIVE="+env.Paths.ArchiveDir, "CLI_NAME="+env.Paths.Name,
	)
	return tea.ExecProcess(cmd, func(error) tea.Msg { return nil })
}
