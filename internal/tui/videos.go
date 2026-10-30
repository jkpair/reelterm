package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/jkpair/reelterm/internal/cache"
	"github.com/jkpair/reelterm/internal/thumb"
	"github.com/jkpair/reelterm/internal/youtube"
	"github.com/jkpair/reelterm/internal/ytdlp"
)

// ListKind controls how a list loads more entries.
type ListKind int

const (
	KindPlaylist ListKind = iota // --playlist-start/--playlist-end paging
	KindFeed                     // feeds only grow from the top: refetch 1..N
	KindHome                     // home feed: cached, refreshed in the background
	KindStatic                   // local data (saved, recent, plugins)
)

// Source describes what a VideoList shows.
type Source struct {
	Title     string
	URL       string
	URLForAll string
	Kind      ListKind
	Entries   []ytdlp.Entry
	Args      []string // custom yt-dlp command instead of URL
}

type (
	listLoadedMsg struct {
		list   *VideoList
		pl     *ytdlp.Playlist
		err    error
		more   bool
		silent bool
	}
	enrichedMsg struct {
		key  string
		meta *ytdlp.Entry
	}
	enrichTickMsg struct {
		list *VideoList
		key  string
	}
	feedTickMsg struct{ list *VideoList }
)

// VideoList is the main browsing screen: list on the right, preview on the left.
type VideoList struct {
	env       *Env
	src       Source
	entries   []ytdlp.Entry
	visible   []int
	marked    map[string]bool
	cursor    int
	offset    int
	filter    string
	loading   bool
	more      bool
	exhausted bool
	err       error
	spin      spinner.Model
	enumerate bool
	channelPL bool // the URL is a channel/playlist page (Play All can stream it directly)
	enriching map[string]bool
	onSelect  func(l *VideoList, e ytdlp.Entry) tea.Cmd
	onMarked  func(l *VideoList, es []ytdlp.Entry) tea.Cmd
	// alwaysMarked sends enter to onMarked even with nothing marked.
	alwaysMarked bool
	itemKind     string // "video", "channel", "playlist" or "file"
	w, h         int
}

func NewVideoList(env *Env, src Source) *VideoList {
	sp := spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(env.S.Info))
	l := &VideoList{env: env, src: src, spin: sp, marked: map[string]bool{}, enriching: map[string]bool{}, itemKind: "video"}
	if src.Kind == KindStatic {
		l.setEntries(src.Entries)
	}
	return l
}

func (l *VideoList) pageSize() int {
	if l.src.Kind == KindHome {
		return l.env.Cfg.FeedPageSize
	}
	return l.env.Cfg.NoOfSearchResults
}

func (l *VideoList) Init() tea.Cmd {
	switch l.src.Kind {
	case KindStatic:
		return l.afterLoad()
	case KindHome:
		if pl, err := l.env.Store.LoadFeed(); err == nil && len(pl.Entries) > 0 {
			l.setEntries(pl.Entries)
			cmds := []tea.Cmd{l.afterLoad(), l.feedTick()}
			if l.env.Store.FeedStale(time.Duration(l.env.Cfg.FeedCacheTTL) * time.Second) {
				cmds = append(cmds, l.fetch(ytdlp.Range{End: l.pageSize() * 2}, false, true))
			}
			return tea.Batch(cmds...)
		}
		return tea.Batch(l.startLoading(), l.fetch(ytdlp.Range{End: l.pageSize() * 2}, false, false), l.feedTick())
	}
	return tea.Batch(l.startLoading(), l.fetch(ytdlp.Range{End: l.pageSize()}, false, false))
}

func (l *VideoList) startLoading() tea.Cmd {
	l.loading = true
	return l.spin.Tick
}

// fetch loads entries in the background.
func (l *VideoList) fetch(r ytdlp.Range, more, silent bool) tea.Cmd {
	env, src := l.env, l.src
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		var pl *ytdlp.Playlist
		var err error
		if len(src.Args) > 0 {
			pl, err = env.YT.FetchArgs(ctx, src.Args)
		} else {
			pl, err = env.YT.Fetch(ctx, src.URL, r)
		}
		return listLoadedMsg{list: l, pl: pl, err: err, more: more, silent: silent}
	}
}

func (l *VideoList) feedTick() tea.Cmd {
	d := time.Duration(max(l.env.Cfg.FeedCacheTTL, 300)) * time.Second
	return tea.Tick(d, func(time.Time) tea.Msg { return feedTickMsg{l} })
}

func (l *VideoList) setEntries(e []ytdlp.Entry) {
	var key string
	if cur, ok := l.Selected(); ok {
		key = cur.Key()
	}
	// Cached and local data may predate sanitising, and feeds can repeat items.
	seen := make(map[string]bool, len(e))
	l.entries = l.entries[:0:0]
	for _, x := range e {
		if seen[x.Key()] {
			continue
		}
		seen[x.Key()] = true
		if m, ok := l.env.Store.Meta(x.ID); ok && x.NeedsEnrich() {
			m.Clean()
			x.Merge(m)
		}
		x.Clean()
		l.entries = append(l.entries, x)
	}
	l.applyFilter()
	if key != "" {
		for vi, i := range l.visible {
			if l.entries[i].Key() == key {
				l.cursor = vi
				break
			}
		}
	}
}

func (l *VideoList) applyFilter() {
	l.visible = l.visible[:0]
	q := strings.ToLower(l.filter)
	for i, e := range l.entries {
		if q == "" || strings.Contains(strings.ToLower(e.Title), q) || strings.Contains(strings.ToLower(e.ChannelName()), q) {
			l.visible = append(l.visible, i)
		}
	}
	l.cursor = max(min(l.cursor, len(l.visible)-1), 0)
}

// Selected returns the highlighted entry.
func (l *VideoList) Selected() (ytdlp.Entry, bool) {
	if len(l.visible) == 0 {
		return ytdlp.Entry{}, false
	}
	return l.entries[l.visible[l.cursor]], true
}

// Marked returns multi-selected entries, or the highlighted one.
func (l *VideoList) Marked() []ytdlp.Entry {
	var out []ytdlp.Entry
	for _, e := range l.entries {
		if l.marked[e.Key()] {
			out = append(out, e)
		}
	}
	if len(out) == 0 {
		if e, ok := l.Selected(); ok {
			out = append(out, e)
		}
	}
	return out
}

// afterLoad queues thumbnails (visible first) and enrichment of the selection.
func (l *VideoList) afterLoad() tea.Cmd {
	if l.env.Cfg.EnablePreview {
		for i, e := range l.entries {
			l.env.Thumbs.Request(e.Key(), e.ThumbnailURL(), i < 3)
		}
	}
	return l.selectionChanged()
}

func (l *VideoList) selectionChanged() tea.Cmd {
	e, ok := l.Selected()
	if !ok {
		return nil
	}
	if l.env.Cfg.EnablePreview {
		l.env.Thumbs.Request(e.Key(), e.ThumbnailURL(), true)
	}
	if l.itemKind != "video" || !e.NeedsEnrich() || l.enriching[e.Key()] || e.ID == "" {
		return nil
	}
	key := e.Key()
	return tea.Tick(250*time.Millisecond, func(time.Time) tea.Msg { return enrichTickMsg{l, key} })
}

// enrich fetches missing metadata for one video (oEmbed, then yt-dlp -j).
func (l *VideoList) enrich(e ytdlp.Entry) tea.Cmd {
	l.enriching[e.Key()] = true
	env := l.env
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		meta, raw, err := env.YT.Video(ctx, e.ID)
		if err != nil {
			author, _ := youtube.OEmbedAuthor(ctx, e.ID)
			if author == "" {
				return enrichedMsg{key: e.Key()}
			}
			return enrichedMsg{key: e.Key(), meta: &ytdlp.Entry{Channel: author}}
		}
		_ = env.Store.SaveMetaRaw(e.ID, raw)
		return enrichedMsg{key: e.Key(), meta: meta}
	}
}

func (l *VideoList) maybeLoadMore() tea.Cmd {
	if l.src.Kind == KindStatic || len(l.src.Args) > 0 || l.loading || l.more || l.exhausted || l.filter != "" {
		return nil
	}
	if l.cursor < len(l.visible)-5 {
		return nil
	}
	l.more = true
	n := len(l.entries)
	if l.src.Kind == KindPlaylist {
		return tea.Batch(l.spin.Tick, l.fetch(ytdlp.Range{Start: n + 1, End: n + l.pageSize()}, true, false))
	}
	return tea.Batch(l.spin.Tick, l.fetch(ytdlp.Range{End: n + l.pageSize()}, true, false))
}

func (l *VideoList) Help() string {
	if len(l.marked) > 0 {
		return hints(l.env, "enter", fmt.Sprintf("act on %d marked", len(l.marked)), "tab", "mark", "esc", "clear marks")
	}
	return hints(l.env, "enter", "actions", "tab", "mark", "^r", "reload", "^d", "download", "^o", "browser", "esc", "back")
}

func (l *VideoList) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case listLoadedMsg:
		if msg.list != l {
			return l, nil
		}
		return l, l.handleLoaded(msg)
	case enrichTickMsg:
		if msg.list != l {
			return l, nil
		}
		if e, ok := l.Selected(); ok && e.Key() == msg.key && !l.enriching[msg.key] {
			return l, l.enrich(e)
		}
	case enrichedMsg:
		if msg.meta == nil {
			return l, nil
		}
		for i := range l.entries {
			if l.entries[i].Key() == msg.key {
				l.entries[i].Merge(msg.meta)
			}
		}
	case feedTickMsg:
		if msg.list != l {
			return l, nil
		}
		return l, tea.Batch(l.fetch(ytdlp.Range{End: l.pageSize() * 2}, false, true), l.feedTick())
	case spinner.TickMsg:
		if l.loading || l.more {
			var cmd tea.Cmd
			l.spin, cmd = l.spin.Update(msg)
			return l, cmd
		}
	case thumb.ReadyMsg:
		// Re-render happens automatically; nothing to store.
	case focusMsg:
		l.marked = map[string]bool{}
	case tea.KeyMsg:
		return l, l.handleKey(msg)
	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			return l, l.move(-1)
		case tea.MouseButtonWheelDown:
			return l, l.move(1)
		}
	}
	return l, nil
}

func (l *VideoList) handleLoaded(msg listLoadedMsg) tea.Cmd {
	wasLoading := l.loading
	l.loading, l.more = false, false
	if msg.err != nil {
		if msg.silent {
			return nil
		}
		if wasLoading && len(l.entries) == 0 {
			l.err = msg.err
		}
		return toast("Failed to fetch data: " + msg.err.Error())
	}
	pl := msg.pl
	l.err = nil
	if pl.UploaderURL != "" {
		l.channelPL = true
	}
	switch {
	case msg.silent && l.src.Kind == KindHome:
		merged := cache.MergeFeed(l.entries, pl.Entries)
		added := len(merged) - len(l.entries)
		_ = l.env.Store.SaveFeed(&ytdlp.Playlist{Entries: merged})
		if added == 0 {
			return nil
		}
		l.setEntries(merged)
		return tea.Batch(l.afterLoad(), toast(fmt.Sprintf("Feed updated: %d new videos", added)))
	case msg.more && l.src.Kind == KindPlaylist:
		// Search results shift between requests, so pages can overlap.
		seen := map[string]bool{}
		for _, e := range l.entries {
			seen[e.Key()] = true
		}
		var fresh []ytdlp.Entry
		for _, e := range pl.Entries {
			if !seen[e.Key()] {
				seen[e.Key()] = true
				fresh = append(fresh, e)
			}
		}
		if len(fresh) == 0 {
			l.exhausted = true
			return toast("No more content available")
		}
		l.setEntries(append(l.entries, fresh...))
	case msg.more:
		if len(pl.Entries) <= len(l.entries) {
			l.exhausted = true
			return toast("No more content available")
		}
		l.setEntries(pl.Entries)
	default:
		l.setEntries(pl.Entries)
		if len(pl.Entries) < l.pageSize() && l.src.Kind == KindPlaylist {
			l.exhausted = true
		}
	}
	if l.src.Kind == KindHome {
		_ = l.env.Store.SaveFeed(&ytdlp.Playlist{Entries: l.entries})
	}
	return l.afterLoad()
}

func (l *VideoList) move(d int) tea.Cmd {
	n := len(l.visible)
	if n == 0 {
		return nil
	}
	old := l.cursor
	l.cursor = max(min(l.cursor+d, n-1), 0)
	if old == l.cursor {
		return l.maybeLoadMore()
	}
	return tea.Batch(l.selectionChanged(), l.maybeLoadMore())
}

func (l *VideoList) handleKey(k tea.KeyMsg) tea.Cmd {
	listH := max(l.h-1, 1)
	switch k.String() {
	case "up", "ctrl+p", "ctrl+k":
		return l.move(-1)
	case "down", "ctrl+n", "ctrl+j":
		return l.move(1)
	case "pgup", "shift+left":
		return l.move(-listH)
	case "pgdown", "shift+right":
		return l.move(listH)
	case "home":
		return l.move(-len(l.visible))
	case "end":
		return l.move(len(l.visible))
	case "tab":
		if e, ok := l.Selected(); ok {
			if l.marked[e.Key()] {
				delete(l.marked, e.Key())
			} else {
				l.marked[e.Key()] = true
			}
		}
		return l.move(1)
	case "enter", "right":
		if len(l.marked) > 0 || l.alwaysMarked {
			if l.onMarked != nil {
				return l.onMarked(l, l.Marked())
			}
			return push(NewBatchActions(l.env, l, l.Marked()))
		}
		e, ok := l.Selected()
		if !ok {
			return nil
		}
		if l.onSelect != nil {
			return l.onSelect(l, e)
		}
		switch e.Kind() {
		case "channel":
			return push(NewChannelActions(l.env, e))
		case "playlist":
			return push(NewVideoList(l.env, Source{Title: e.Title, URL: e.WatchURL(), Kind: KindPlaylist}))
		}
		return push(NewVideoActions(l.env, l, e))
	case "esc":
		switch {
		case l.filter != "":
			l.filter = ""
			l.applyFilter()
			return l.selectionChanged()
		case len(l.marked) > 0:
			l.marked = map[string]bool{}
			return nil
		}
		return pop()
	case "ctrl+r":
		if l.src.Kind == KindStatic {
			return nil
		}
		l.exhausted = false
		return tea.Batch(l.startLoading(), l.fetch(ytdlp.Range{End: l.pageSize()}, false, false))
	case "ctrl+d":
		if l.itemKind == "video" {
			return startDownloads(l.env, l, l.Marked(), false)
		}
	case "ctrl+o":
		if e, ok := l.Selected(); ok {
			return openInBrowser(e.WatchURL())
		}
	case "ctrl+y":
		if e, ok := l.Selected(); ok {
			return copyLink(e.WatchURL())
		}
	case "ctrl+s":
		if e, ok := l.Selected(); ok && l.itemKind == "video" {
			return saveVideo(l.env, e)
		}
	case "backspace":
		if l.filter != "" {
			r := []rune(l.filter)
			l.filter = string(r[:len(r)-1])
			l.applyFilter()
			return l.selectionChanged()
		}
	case "ctrl+u":
		l.filter = ""
		l.applyFilter()
		return l.selectionChanged()
	default:
		if k.Type == tea.KeyRunes || k.Type == tea.KeySpace {
			l.filter += string(k.Runes)
			l.cursor = 0
			l.applyFilter()
			return l.selectionChanged()
		}
	}
	return nil
}

// --- View ---

func (l *VideoList) prompt() string {
	p := l.src.Title
	if p == "" {
		p = "select video"
	}
	if n := len(l.entries); n > 0 {
		p += fmt.Sprintf(" [%d/%d]", min(l.cursor+1, len(l.visible)), len(l.visible))
	}
	return p
}

func (l *VideoList) View(w, h int) string {
	l.w, l.h = w, h
	s := l.env.S
	var b strings.Builder
	head := promptLine(l.env, l.prompt(), l.filter, w)
	if l.loading || l.more {
		head += "  " + l.spin.View() + s.Dim.Render(" loading…")
	}
	b.WriteString(ansi.Truncate(head, w, ""))
	b.WriteString("\n")
	bodyH := h - 1

	if l.err != nil && len(l.entries) == 0 {
		b.WriteString("\n " + s.Error.Render("Failed to fetch data") + "\n\n " + s.Dim.Render(wrap(l.err.Error(), w-4)) +
			"\n\n " + s.Dim.Render("Press ctrl+r to retry, esc to go back."))
		return b.String()
	}
	if l.loading && len(l.entries) == 0 {
		b.WriteString("\n " + l.spin.View() + s.Dim.Render(" Loading…"))
		return b.String()
	}
	if len(l.entries) == 0 {
		b.WriteString("\n " + s.Dim.Render("Nothing here."))
		return b.String()
	}

	paneW := 0
	if l.env.Cfg.EnablePreview && w >= 70 && bodyH >= 10 {
		paneW = max(w*35/100, 32)
	}
	// Keep the last column free: a full-width line puts the terminal in its
	// pending-wrap state, where the renderer's erase-line hits the last cell.
	listW := w - 1
	if paneW > 0 {
		listW = w - paneW - 2 // gap + free last column
	}
	var pane []string
	if paneW > 0 {
		pane = l.previewLines(paneW, bodyH)
	}
	l.offset = clampOffset(l.offset, l.cursor, bodyH, len(l.visible))
	for row := 0; row < bodyH; row++ {
		line := ""
		if paneW > 0 {
			line = pane[row] + " "
		}
		if i := l.offset + row; i < len(l.visible) {
			line += l.renderRow(l.visible[i], i == l.cursor, listW)
		}
		b.WriteString(line)
		if row < bodyH-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (l *VideoList) renderRow(idx int, sel bool, w int) string {
	s := l.env.S
	e := l.entries[idx]
	num := fmt.Sprintf("%02d ", idx+1)
	mark := "  "
	if l.marked[e.Key()] {
		mark = s.OK.Render("● ")
	}
	title := e.Title
	status := e.Status()
	label := ""
	if status != "available" {
		label = " " + s.StatusStyle(status).Render("["+ytdlp.StatusLabel(status)+"]")
	}
	right := ""
	if l.itemKind == "video" {
		parts := []string{}
		if c := e.ChannelName(); c != "" && w > 70 {
			parts = append(parts, c)
		}
		if c := e.Clock(); c != "" {
			parts = append(parts, c)
		}
		right = strings.Join(parts, " · ")
	}
	// Row layout: pointer(2) mark(2) num title label gap(>=1) right space(1).
	inner := w - 2 - 2 - len(num) - cells(label) - 1
	rightW := 0
	if right != "" {
		right = fit(right, max(inner/3, 0), "…")
		rightW = cells(right) + 1 // plus the minimum gap
	}
	title = fit(title, max(inner-rightW, 1), "…")
	base, dim := s.Fg, s.Dim
	if sel {
		base, dim = s.Selected, s.Dim.Background(s.T.BgSel)
	}
	left := mark + dim.Render(num) + highlight(s, title, l.filter, sel) + label
	gap := max(w-2-1-cells(left)-cells(right), 1)
	body := left + base.Render(strings.Repeat(" ", gap)) + dim.Render(right) + base.Render(" ")
	// Hard limit: rows sit beside overlay escapes the renderer can't measure,
	// so anything wider than w would wrap and scroll the whole screen.
	body = fit(body, w-2, "")
	if sel {
		return s.Pointer.Render("▌ ") + body
	}
	return "  " + body
}

// previewLines renders the preview pane as exactly h lines of width w.
// Overlay image rows use cursor-forward escapes so the renderer leaves the
// graphic alone.
func (l *VideoList) previewLines(w, h int) []string {
	s := l.env.S
	bc := s.Border
	iw := w - 2
	lines := make([]string, 0, h)
	lines = append(lines, bc.Render("╭"+strings.Repeat("─", iw)+"╮"))
	padTo := func(str string) string {
		str = fit(str, iw, "…")
		return bc.Render("│") + str + strings.Repeat(" ", max(iw-cells(str), 0)) + bc.Render("│")
	}

	e, ok := l.Selected()
	if !ok {
		for len(lines) < h-1 {
			lines = append(lines, padTo(""))
		}
		return append(lines, bc.Render("╰"+strings.Repeat("─", iw)+"╯"))
	}

	// Image area: 16:9 at the terminal's cell aspect.
	imgRows := int(float64(iw)*9/16/l.env.Thumbs.CellAspect() + 0.5)
	if l.itemKind == "channel" {
		imgRows = min(imgRows, iw/4)
	}
	imgRows = min(imgRows, (h-2)/2)
	if l.env.Thumbs.Proto == thumb.None {
		imgRows = 0
	}
	key := e.Key()
	if imgRows > 0 {
		absY := l.env.BodyTop + 2
		if l.env.Thumbs.Place(key, 1, absY, iw, imgRows) {
			for range imgRows {
				lines = append(lines, bc.Render("│")+ansi.CursorForward(iw)+bc.Render("│"))
			}
		} else if r := l.env.Thumbs.Get(key, iw, imgRows); r != nil && r.Text != "" {
			for _, tl := range strings.Split(r.Text, "\n") {
				lines = append(lines, padTo(tl))
			}
			for len(lines) < imgRows+1 {
				lines = append(lines, padTo(""))
			}
		} else {
			for range imgRows {
				lines = append(lines, padTo(""))
			}
		}
	}

	var t []string
	add := func(str string) { t = append(t, str) }
	if imgRows > 0 {
		add(s.SectionRule.Render(strings.Repeat("━", iw)))
		if st := l.env.Thumbs.Status(key); st != "ready" && e.ThumbnailURL() != "" {
			add(s.Dim.Render(map[string]string{"loading": " loading thumbnail…", "failed": " no thumbnail"}[st]))
		}
	}
	for _, tl := range strings.Split(wrap(e.Title, iw-1), "\n") {
		add(" " + s.Title.Render(tl))
	}
	add(s.SectionRule.Render(strings.Repeat("─", iw)))
	field := func(k, v string) {
		if v != "" {
			add(s.Label.Render(fmt.Sprintf("  %-10s", k+":")) + " " + s.Fg.Render(v))
		}
	}
	kind := l.itemKind
	if kind == "video" {
		kind = e.Kind()
	}
	switch kind {
	case "channel":
		if e.ChannelFollowers != nil {
			field("Followers", ytdlp.Commas(*e.ChannelFollowers))
		}
		field("Channel", e.ChannelName())
		field("URL", e.ChannelBase())
	case "file":
		field("Path", strings.TrimPrefix(filepath.Dir(e.URL), l.env.Cfg.DownloadDirectory+"/"))
		if st, err := os.Stat(e.URL); err == nil && !st.IsDir() {
			field("Size", fmt.Sprintf("%.1f MB", float64(st.Size())/1e6))
			field("Modified", e.Uploaded(time.Now()))
		}
	case "playlist":
		field("Channel", e.ChannelName())
		if e.PlaylistCount != nil {
			field("Videos", fmt.Sprint(*e.PlaylistCount))
		}
	default:
		ch := e.ChannelName()
		if ch == "" {
			ch = "Unknown"
		}
		field("Channel", ch)
		field("Duration", e.DurationText())
		views := e.Views()
		if views != "N/A" {
			views += " views"
		}
		field("Views", views)
		field("Live", e.Live())
		field("Uploaded", e.Uploaded(time.Now()))
		if l.env.Store.IsSaved(e.ID) {
			field("Saved", "yes")
		}
		if st := e.Status(); st != "available" {
			add("")
			add(s.StatusStyle(st).Render("  ⚠ Status:   " + ytdlp.StatusLabel(st)))
			add(s.StatusStyle(st).UnsetBold().Render("  This video cannot be played."))
		}
	}
	if d := strings.TrimSpace(e.Desc()); d != "" {
		add("")
		add(s.SectionRule.Render(strings.Repeat("─", iw)))
		for _, dl := range strings.Split(wrap(d, iw-1), "\n") {
			add(" " + s.Fg.Render(dl))
		}
	}
	for _, tl := range t {
		if len(lines) >= h-1 {
			break
		}
		lines = append(lines, padTo(tl))
	}
	for len(lines) < h-1 {
		lines = append(lines, padTo(""))
	}
	return append(lines[:h-1], bc.Render("╰"+strings.Repeat("─", iw)+"╯"))
}
