// Package plugin runs Lua plugins (the replacement for sourced Bash extensions).
//
// A plugin is a .lua file that registers menu items, video actions and
// event hooks through the global `yt` table. See plugins/example.lua.
package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"

	"github.com/jkpair/yt-x-plus/internal/config"
	"github.com/jkpair/yt-x-plus/internal/player"
	"github.com/jkpair/yt-x-plus/internal/tui"
	"github.com/jkpair/yt-x-plus/internal/youtube"
	"github.com/jkpair/yt-x-plus/internal/ytdlp"
)

type item struct {
	label, icon string
	fn          *lua.LFunction
}

// Runtime is a single Lua state shared by all loaded plugins.
type Runtime struct {
	mu      sync.Mutex // guards the Lua state (held while a callback runs)
	imu     sync.Mutex // guards menu, actions and hooks
	L       *lua.LState
	cfg     *config.Config
	yt      *ytdlp.Client
	player  *player.Player
	start   func(title string, args []string) int
	menu    []item
	actions []item
	hooks   map[string][]*lua.LFunction
	result  *tui.PluginResult
}

// New creates a runtime. startDownload is used by yt.download.
func New(cfg *config.Config, yt *ytdlp.Client, p *player.Player, startDownload func(string, []string) int) *Runtime {
	r := &Runtime{L: lua.NewState(), cfg: cfg, yt: yt, player: p, start: startDownload, hooks: map[string][]*lua.LFunction{}}
	r.register()
	return r
}

// Resolve finds a plugin by name (in dir, .lua optional) or path.
func Resolve(dir, name string) (string, error) {
	if strings.ContainsRune(name, os.PathSeparator) {
		return name, nil
	}
	for _, c := range []string{name, name + ".lua"} {
		p := filepath.Join(dir, c)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("plugin %q not found in %s", name, dir)
}

// Load runs a plugin file.
func (r *Runtime) Load(path string) error {
	if !strings.HasSuffix(path, ".lua") {
		return fmt.Errorf("%s: Bash extensions are no longer supported; port it to a .lua plugin (see plugins/example.lua)", filepath.Base(path))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.L.DoFile(path); err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return nil
}

// Hooks exposes the runtime to the TUI.
func (r *Runtime) Hooks() tui.Hooks {
	return tui.Hooks{
		MenuItems:    func() []tui.PluginItem { return r.items(&r.menu) },
		VideoActions: func() []tui.PluginItem { return r.items(&r.actions) },
		Emit:         r.emit,
	}
}

func (r *Runtime) items(src *[]item) []tui.PluginItem {
	r.imu.Lock()
	defer r.imu.Unlock()
	out := make([]tui.PluginItem, len(*src))
	for i, it := range *src {
		fn := it.fn
		out[i] = tui.PluginItem{Label: it.label, Icon: it.icon, Run: func(e *ytdlp.Entry) tui.PluginResult {
			return r.call(fn, e)
		}}
	}
	return out
}

// call invokes fn with an optional video and collects the result.
func (r *Runtime) call(fn *lua.LFunction, e *ytdlp.Entry) tui.PluginResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	res := tui.PluginResult{}
	r.result = &res
	defer func() { r.result = nil }()
	var args []lua.LValue
	if e != nil {
		args = append(args, r.entryTable(*e))
	}
	if err := r.L.CallByParam(lua.P{Fn: fn, NRet: 1, Protect: true}, args...); err != nil {
		res.Toast = "plugin error: " + err.Error()
		return res
	}
	if ret := r.L.Get(-1); ret.Type() == lua.LTString {
		res.Toast = ret.String()
	}
	r.L.Pop(1)
	return res
}

func (r *Runtime) emit(event string, e *ytdlp.Entry) {
	r.imu.Lock()
	fns := r.hooks[event]
	r.imu.Unlock()
	for _, fn := range fns {
		go r.call(fn, e)
	}
}

func (r *Runtime) entryTable(e ytdlp.Entry) *lua.LTable {
	t := r.L.NewTable()
	set := func(k string, v lua.LValue) { t.RawSetString(k, v) }
	set("id", lua.LString(e.ID))
	set("title", lua.LString(e.Title))
	set("url", lua.LString(e.WatchURL()))
	set("channel", lua.LString(e.ChannelName()))
	set("channel_url", lua.LString(e.ChannelBase()))
	set("thumbnail", lua.LString(e.ThumbnailURL()))
	set("description", lua.LString(e.Desc()))
	set("duration", lua.LString(e.DurationText()))
	set("views", lua.LString(e.Views()))
	if e.Duration != nil {
		set("duration_seconds", lua.LNumber(*e.Duration))
	}
	if e.ViewCount != nil {
		set("view_count", lua.LNumber(*e.ViewCount))
	}
	return t
}

func tableEntry(t *lua.LTable) ytdlp.Entry {
	str := func(k string) string { return lua.LVAsString(t.RawGetString(k)) }
	e := ytdlp.Entry{ID: str("id"), Title: str("title"), URL: str("url"), Channel: str("channel"), UploaderURL: str("channel_url")}
	if th := str("thumbnail"); th != "" {
		e.Thumbnails = []ytdlp.Thumbnail{{URL: th}}
	}
	if d := str("description"); d != "" {
		e.Description = &d
	}
	if n, ok := t.RawGetString("duration_seconds").(lua.LNumber); ok {
		f := float64(n)
		e.Duration = &f
	}
	if e.Title == "" {
		e.Title = e.URL
	}
	return e
}

func (r *Runtime) entriesTable(es []ytdlp.Entry) *lua.LTable {
	t := r.L.NewTable()
	for _, e := range es {
		t.Append(r.entryTable(e))
	}
	return t
}

func optBool(L *lua.LState, n int, key string) bool {
	if t, ok := L.Get(n).(*lua.LTable); ok {
		return lua.LVAsBool(t.RawGetString(key))
	}
	return false
}

func (r *Runtime) register() {
	L := r.L
	yt := L.NewTable()
	fn := func(name string, f lua.LGFunction) { yt.RawSetString(name, L.NewFunction(f)) }
	addItem := func(dst *[]item) lua.LGFunction {
		return func(L *lua.LState) int {
			r.imu.Lock()
			defer r.imu.Unlock()
			if t, ok := L.Get(1).(*lua.LTable); ok {
				f, ok := t.RawGetString("run").(*lua.LFunction)
				if !ok {
					L.ArgError(1, "table needs a run function")
				}
				*dst = append(*dst, item{label: lua.LVAsString(t.RawGetString("label")), icon: lua.LVAsString(t.RawGetString("icon")), fn: f})
				return 0
			}
			label := L.CheckString(1)
			if f, ok := L.Get(2).(*lua.LFunction); ok {
				*dst = append(*dst, item{label: label, fn: f})
				return 0
			}
			*dst = append(*dst, item{label: label, icon: L.OptString(2, ""), fn: L.CheckFunction(3)})
			return 0
		}
	}
	fn("menu_item", addItem(&r.menu))
	fn("video_action", addItem(&r.actions))
	fn("on", func(L *lua.LState) int {
		ev, f := L.CheckString(1), L.CheckFunction(2)
		r.imu.Lock()
		r.hooks[ev] = append(r.hooks[ev], f)
		r.imu.Unlock()
		return 0
	})
	fn("config", func(L *lua.LState) int {
		L.Push(lua.LString(r.cfg.Get(L.CheckString(1))))
		return 1
	})
	fetch := func(L *lua.LState, url string, limit int) int {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		pl, err := r.yt.Fetch(ctx, url, ytdlp.Range{End: limit})
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(r.entriesTable(pl.Entries))
		return 1
	}
	fn("ytdlp", func(L *lua.LState) int { return fetch(L, L.CheckString(1), L.OptInt(2, r.cfg.NoOfSearchResults)) })
	fn("search", func(L *lua.LState) int {
		return fetch(L, youtube.SearchURL(L.CheckString(1)), L.OptInt(2, r.cfg.NoOfSearchResults))
	})
	fn("mix_url", func(L *lua.LState) int { L.Push(lua.LString(youtube.MixURL(L.CheckString(1)))); return 1 })
	fn("play", func(L *lua.LState) int {
		req := player.Request{URL: L.CheckString(1), Title: L.CheckString(1), AudioOnly: optBool(L, 2, "audio")}
		cmd, err := r.player.Command(context.Background(), req)
		if err == nil {
			err = r.player.Start(cmd, r.cfg.DisownStreaming)
		}
		if err != nil {
			L.Push(lua.LString(err.Error()))
			return 1
		}
		return 0
	})
	fn("download", func(L *lua.LState) int {
		url := L.CheckString(1)
		args := []string{url, "--output", filepath.Join(r.cfg.DownloadDirectory, "videos", "plugins", "%(channel)s", "%(title)s.%(ext)s")}
		mode := ytdlp.ModeDownload
		if optBool(L, 2, "audio") {
			args = []string{url, "-x", "-f", "bestaudio/best", "--audio-format", "mp3", "--output", filepath.Join(r.cfg.DownloadDirectory, "audio", "plugins", "%(channel)s", "%(title)s.%(ext)s")}
			mode = ytdlp.ModeAudio
		}
		args = append(args, r.yt.BuildOpts(r.cfg, mode)...)
		args = append(args, r.yt.CookieOpts()...)
		r.start(url, args)
		return 0
	})
	fn("notify", func(L *lua.LState) int {
		msg := L.CheckString(1)
		if r.result != nil {
			r.result.Toast = msg
		} else {
			player.Notify("yt-x", msg)
		}
		return 0
	})
	fn("show_list", func(L *lua.LState) int {
		if r.result == nil {
			return 0
		}
		r.result.Title = L.CheckString(1)
		switch v := L.Get(2).(type) {
		case lua.LString:
			r.result.URL = string(v)
		case *lua.LTable:
			v.ForEach(func(_, val lua.LValue) {
				if t, ok := val.(*lua.LTable); ok {
					r.result.Entries = append(r.result.Entries, tableEntry(t))
				}
			})
		}
		return 0
	})
	fn("open", func(L *lua.LState) int {
		if err := player.OpenURL(L.CheckString(1)); err != nil {
			L.Push(lua.LString(err.Error()))
			return 1
		}
		return 0
	})
	fn("shell", func(L *lua.LState) int {
		out, err := exec.Command("sh", "-c", L.CheckString(1)).CombinedOutput()
		L.Push(lua.LString(strings.TrimRight(string(out), "\n")))
		if err != nil {
			L.Push(lua.LString(err.Error()))
			return 2
		}
		return 1
	})
	L.SetGlobal("yt", yt)
}

// Close releases the Lua state.
func (r *Runtime) Close() { r.L.Close() }
