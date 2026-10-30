package tui

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jkpair/yt-x-plus/internal/player"
	"github.com/jkpair/yt-x-plus/internal/ytdlp"
)

// Local library: browsing downloads and local mixes (replaces the
// downloads*, mix and mixer Bash extensions).

var (
	audioExt = map[string]bool{".mp3": true, ".flac": true, ".aac": true, ".ogg": true, ".opus": true, ".wav": true, ".m4a": true}
	videoExt = map[string]bool{".mp4": true, ".mkv": true, ".webm": true, ".avi": true, ".mov": true, ".flv": true}
)

func isMedia(p string) bool {
	e := strings.ToLower(filepath.Ext(p))
	return audioExt[e] || videoExt[e]
}

func fileEntry(path string, dir bool) ytdlp.Entry {
	title := filepath.Base(path)
	e := ytdlp.Entry{ID: path, Title: title, URL: path}
	if dir {
		e.Title += "/"
		e.Type = "dir"
		// Preview a media file from inside the folder.
		_ = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && isMedia(p) {
				e.Thumbnails = []ytdlp.Thumbnail{{URL: p}}
				return fs.SkipAll
			}
			return nil
		})
	} else {
		e.Thumbnails = []ytdlp.Thumbnail{{URL: path}}
		if st, err := os.Stat(path); err == nil {
			ts := st.ModTime().Unix()
			e.Timestamp = &ts
		}
	}
	return e
}

// NewLocalBrowser lists a directory: folders first, then media files.
func NewLocalBrowser(env *Env, dir string) Screen {
	items, err := os.ReadDir(dir)
	if err != nil {
		return NewMessage(env, "Downloads", "Nothing downloaded yet.\n\n"+dir)
	}
	var dirs, files []ytdlp.Entry
	for _, it := range items {
		p := filepath.Join(dir, it.Name())
		switch {
		case strings.HasPrefix(it.Name(), "."):
		case it.IsDir():
			dirs = append(dirs, fileEntry(p, true))
		case isMedia(p):
			files = append(files, fileEntry(p, false))
		}
	}
	entries := append(dirs, files...)
	if len(entries) == 0 {
		return NewMessage(env, "Downloads", "No media files in "+dir)
	}
	l := NewVideoList(env, Source{Title: strings.TrimPrefix(dir, filepath.Dir(env.Cfg.DownloadDirectory)+"/"), Kind: KindStatic, Entries: entries})
	l.itemKind = "file"
	l.onSelect = func(_ *VideoList, e ytdlp.Entry) tea.Cmd {
		if e.Type == "dir" {
			return push(NewMenu(env, e.Title, func() []Item {
				return []Item{
					{Icon: env.Theme.Icon("playlist"), IconColor: env.Theme.Cyan, Label: "Open", Desc: e.URL},
					{Icon: env.Theme.Icon("play"), IconColor: env.Theme.Green, Label: "Play All", Desc: "Play every file in this folder"},
					{Icon: env.Theme.Icon("audio"), IconColor: env.Theme.Green, Label: "Listen To All", Desc: "Play every file in this folder without video"},
				}
			}, func(it Item) tea.Cmd {
				switch it.Label {
				case "Open":
					return replace(NewLocalBrowser(env, e.URL))
				case "Play All":
					return tea.Batch(pop(), runPlayer(env, player.Request{URL: e.URL, Title: e.Title}))
				}
				return tea.Batch(pop(), runPlayer(env, player.Request{URL: e.URL, Title: e.Title, AudioOnly: true}))
			}))
		}
		return runPlayer(env, player.Request{URL: e.URL, Title: e.Title, AudioOnly: audioExt[strings.ToLower(filepath.Ext(e.URL))]})
	}
	l.onMarked = func(_ *VideoList, es []ytdlp.Entry) tea.Cmd {
		return playFiles(env, es, false)
	}
	return l
}

func playFiles(env *Env, es []ytdlp.Entry, audio bool) tea.Cmd {
	var files []ytdlp.Entry
	for _, e := range es {
		if e.Type != "dir" {
			files = append(files, e)
		}
	}
	if len(files) == 0 {
		return toast("No files selected")
	}
	p, err := env.Store.M3U("local-"+files[0].URL+string(rune(len(files))), files)
	if err != nil {
		return toastErr(err)
	}
	return runPlayer(env, player.Request{URL: p, Title: "local mix", AudioOnly: audio})
}

func (e *Env) mixesDir(kind string) string { return filepath.Join(e.Paths.ConfigDir, "mixes", kind) }

// NewMixCreator lists every audio or video file under the download
// directory; mark files with tab and press enter to save them as a mix.
func NewMixCreator(env *Env, kind string) Screen {
	root := filepath.Join(env.Cfg.DownloadDirectory, map[string]string{"audio": "audio", "video": "videos"}[kind])
	exts := audioExt
	if kind == "video" {
		exts = videoExt
	}
	var entries []ytdlp.Entry
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && exts[strings.ToLower(filepath.Ext(p))] {
			entries = append(entries, fileEntry(p, false))
		}
		return nil
	})
	if len(entries) == 0 {
		return NewMessage(env, "Create Mix", "No "+kind+" files found in "+root)
	}
	l := NewVideoList(env, Source{Title: "Mark files for the " + kind + " mix (tab), enter to save", Kind: KindStatic, Entries: entries})
	l.itemKind = "file"
	l.alwaysMarked = true
	l.onMarked = func(_ *VideoList, es []ytdlp.Entry) tea.Cmd {
		return push(NewPrompt(env, "Name of the mix", "", func(name string) tea.Cmd {
			dir := env.mixesDir(kind)
			_ = os.MkdirAll(dir, 0o755)
			var b strings.Builder
			b.WriteString("#EXTM3U\n")
			for _, e := range es {
				b.WriteString("#EXTINF:-1," + e.Title + "\n" + e.URL + "\n")
			}
			if err := os.WriteFile(filepath.Join(dir, cleanName(name)+".m3u8"), []byte(b.String()), 0o644); err != nil {
				return tea.Batch(pop(), toastErr(err))
			}
			return tea.Sequence(pop(), pop(), toast("Mix saved: "+name))
		}))
	}
	return l
}

// NewMixes lists saved local mixes with play/delete actions.
func NewMixes(env *Env) *Menu {
	var m *Menu
	m = NewMenu(env, "Local Mixes", func() []Item {
		var items []Item
		for _, kind := range []string{"audio", "video"} {
			files, _ := filepath.Glob(filepath.Join(env.mixesDir(kind), "*.m3u8"))
			sort.Strings(files)
			if len(files) == 0 {
				continue
			}
			items = append(items, section(strings.ToUpper(kind[:1])+kind[1:]))
			for _, f := range files {
				items = append(items, Item{Icon: env.Theme.Icon(map[string]string{"audio": "audio", "video": "play"}[kind]), IconColor: env.Theme.Cyan,
					Label: strings.TrimSuffix(filepath.Base(f), ".m3u8"), Value: f, Desc: f})
			}
		}
		if len(items) == 0 {
			items = append(items, Item{Label: "No mixes yet — create one from the Downloads menu", Value: ""})
		}
		return items
	}, func(it Item) tea.Cmd {
		if it.Value == "" {
			return pop()
		}
		audio := strings.Contains(it.Value, string(filepath.Separator)+"audio"+string(filepath.Separator))
		return push(NewMenu(env, it.Label, func() []Item {
			return []Item{
				{Icon: env.Theme.Icon("play"), IconColor: env.Theme.Green, Label: "Play Mix"},
				{Icon: "\U000F01B4", IconColor: env.Theme.Red, Label: "Delete Mix"},
				{Icon: env.Theme.Icon("back"), IconColor: env.Theme.Red, Label: "Back"},
			}
		}, func(a Item) tea.Cmd {
			switch a.Label {
			case "Play Mix":
				return tea.Batch(pop(), runPlayer(env, player.Request{URL: it.Value, Title: it.Label, AudioOnly: audio}))
			case "Delete Mix":
				return replace(NewConfirm(env, "Delete mix "+it.Label+"?", func() tea.Cmd {
					if err := os.Remove(it.Value); err != nil {
						return toastErr(err)
					}
					return toast("Deleted " + it.Label)
				}))
			}
			return pop()
		}))
	})
	return m
}
