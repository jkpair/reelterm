package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jkpair/reelterm/internal/cache"
	"github.com/jkpair/reelterm/internal/download"
	"github.com/jkpair/reelterm/internal/youtube"
	"github.com/jkpair/reelterm/internal/ytdlp"
)

// NewDownloadsMenu replaces the downloads* extensions.
func NewDownloadsMenu(env *Env) *Menu {
	t := env.Theme
	return NewMenu(env, "Downloads", func() []Item {
		q := "idle"
		if n := env.Downloads.Active(); n > 0 {
			q = fmt.Sprintf("%d active", n)
		}
		return []Item{
			{Icon: env.Theme.Icon("download"), IconColor: t.Orange, Label: "Download Queue", Suffix: "[" + q + "]", Desc: "Progress of background downloads"},
			section("Download"),
			{Icon: env.Theme.Icon("channel"), IconColor: t.Cyan, Label: "Download Channel", Desc: "Download all videos from a channel URL or @handle"},
			{Icon: env.Theme.Icon("playlist"), IconColor: t.Cyan, Label: "Download Playlist", Desc: "Download all videos from a playlist URL"},
			{Icon: env.Theme.Icon("search"), IconColor: t.Cyan, Label: "Download Search Results", Desc: "Download the top results for a search"},
			{Icon: env.Theme.Icon("audio"), IconColor: t.Cyan, Label: "Download URL (Audio Only)", Desc: "Download any URL yt-dlp supports as mp3"},
			{Icon: env.Theme.Icon("play"), IconColor: t.Cyan, Label: "Download URL", Desc: "Download any URL yt-dlp supports"},
			section("Local Library"),
			{Icon: env.Theme.Icon("saved"), IconColor: t.Blue, Label: "Browse Downloads", Desc: "Browse and play files in " + env.Cfg.DownloadDirectory},
			{Icon: env.Theme.Icon("custom"), IconColor: t.Blue, Label: "Local Mixes", Desc: "Play or delete mixes made from your downloads"},
			{Icon: env.Theme.Icon("audio"), IconColor: t.Blue, Label: "Create Audio Mix", Desc: "Pick downloaded audio files (tab) and save them as a mix"},
			{Icon: env.Theme.Icon("play"), IconColor: t.Blue, Label: "Create Video Mix", Desc: "Pick downloaded video files (tab) and save them as a mix"},
			section(""),
			{Icon: env.Theme.Icon("back"), IconColor: t.Red, Label: "Back"},
		}
	}, func(it Item) tea.Cmd {
		switch it.Label {
		case "Download Queue":
			return push(NewQueue(env))
		case "Browse Downloads":
			return push(NewLocalBrowser(env, env.Cfg.DownloadDirectory))
		case "Local Mixes":
			return push(NewMixes(env))
		case "Create Audio Mix":
			return push(NewMixCreator(env, "audio"))
		case "Create Video Mix":
			return push(NewMixCreator(env, "video"))
		case "Download Channel":
			return push(NewPrompt(env, "Channel URL or @handle", "", func(v string) tea.Cmd {
				if strings.HasPrefix(v, "@") {
					v = "https://www.youtube.com/" + v
				}
				name := strings.TrimPrefix(strings.TrimPrefix(v, "https://www.youtube.com/"), "@")
				return tea.Batch(pop(), downloadChannel(env, name, strings.TrimSuffix(v, "/")))
			}))
		case "Download Playlist":
			return push(NewPrompt(env, "Playlist URL", "", func(u string) tea.Cmd {
				return replace(NewPrompt(env, "Name of the playlist", "", func(name string) tea.Cmd {
					return tea.Batch(pop(), downloadURL(env, name, u, filepath.Join("videos", cleanName(name), "%(playlist_index)s - %(title)s.%(ext)s"), false))
				}))
			}))
		case "Download Search Results":
			return push(NewPrompt(env, "Search term", "", func(q string) tea.Cmd {
				term, _ := youtube.ParseQuery(q)
				u := fmt.Sprintf("ytsearch%d:%s", env.Cfg.NoOfSearchResults, term)
				return tea.Batch(pop(), downloadURL(env, "search: "+term, u, filepath.Join("videos", "search", cleanName(term), "%(channel)s", "%(title)s.%(ext)s"), false))
			}))
		case "Download URL", "Download URL (Audio Only)":
			audio := strings.Contains(it.Label, "Audio")
			return push(NewPrompt(env, "URL", "", func(u string) tea.Cmd {
				kind := "videos"
				if audio {
					kind = "audio"
				}
				return tea.Batch(pop(), downloadURL(env, u, u, filepath.Join(kind, "individual", "%(channel)s", "%(title)s.%(ext)s"), audio))
			}))
		case "Back":
			return pop()
		}
		return nil
	})
}

func downloadChannel(env *Env, name, base string) tea.Cmd {
	return downloadURL(env, "channel: "+name, base+"/videos", filepath.Join("videos", "channels", cleanName(name), "%(title)s.%(ext)s"), false)
}

// downloadURL downloads u into DOWNLOAD_DIRECTORY/out with a download archive.
func downloadURL(env *Env, title, u, out string, audio bool) tea.Cmd {
	args := []string{u, "--output", filepath.Join(env.Cfg.DownloadDirectory, out)}
	mode := ytdlp.ModeDownload
	if audio {
		args = append(args, "-x", "-f", "bestaudio/best", "--audio-format", "mp3")
		mode = ytdlp.ModeAudio
	}
	args = append(args, env.YT.BuildOpts(env.Cfg, mode)...)
	args = append(args, env.YT.CookieOpts()...)
	args = append(args, "--download-archive", filepath.Join(env.Paths.ArchiveDir, cache.Hash(strings.Join(args, " "))))
	env.Downloads.Start(title, args)
	return toast("Downloading: " + title)
}

// Queue lists background downloads with live progress.
type Queue struct {
	env    *Env
	cursor int
}

func NewQueue(env *Env) *Queue { return &Queue{env: env} }
func (q *Queue) Init() tea.Cmd { return nil }
func (q *Queue) Help() string {
	return hints(q.env, "x", "cancel", "c", "clear finished", "esc", "back")
}

func (q *Queue) Update(msg tea.Msg) (Screen, tea.Cmd) {
	jobs := q.env.Downloads.Jobs()
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc", "q":
			return q, pop()
		case "up", "k":
			q.cursor = max(q.cursor-1, 0)
		case "down", "j":
			q.cursor = min(q.cursor+1, max(len(jobs)-1, 0))
		case "x":
			if q.cursor < len(jobs) {
				q.env.Downloads.Cancel(jobs[q.cursor].ID)
			}
		case "c":
			q.env.Downloads.ClearFinished()
			q.cursor = 0
		}
	}
	return q, nil
}

func (q *Queue) View(w, h int) string {
	s := q.env.S
	jobs := q.env.Downloads.Jobs()
	var b strings.Builder
	b.WriteString(s.Prompt.Render(" Download Queue") + "\n\n")
	if len(jobs) == 0 {
		b.WriteString(" " + s.Dim.Render("No downloads yet. Start one from a video's action menu (or ctrl+d in a list)."))
		return b.String()
	}
	barW := max(min(w/3, 40), 10)
	for i, j := range jobs {
		if i*3 >= h-3 {
			break
		}
		ptr := "  "
		if i == q.cursor {
			ptr = s.Pointer.UnsetBackground().Render("▌ ")
		}
		state := s.Info.Render(fmt.Sprintf("%5.1f%%", j.Percent))
		switch {
		case j.Done && j.Err != nil:
			state = s.Error.Render("failed")
		case j.Done:
			state = s.OK.Render("done")
		}
		b.WriteString(ptr + s.Fg.Bold(true).Render(truncate(j.Title, w-14)) + "  " + state + "\n")
		filled := int(j.Percent / 100 * float64(barW))
		bar := s.OK.Render(strings.Repeat("━", filled)) + s.SectionRule.Render(strings.Repeat("━", barW-filled))
		detail := j.Item
		if j.Err != nil {
			detail = j.Err.Error()
		}
		b.WriteString("    " + bar + "  " + s.Dim.Render(truncate(detail, max(w-barW-8, 5))) + "\n\n")
	}
	return b.String()
}

var _ = download.Msg{}
