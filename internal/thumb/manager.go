// Package thumb downloads thumbnails in the background and draws them with
// the best graphics protocol the terminal supports.
package thumb

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ReadyMsg is sent when a thumbnail finished downloading or rendering.
type ReadyMsg struct{ Key string }

// drawMsg triggers a debounced overlay draw.
type drawMsg struct{ seq int }

type job struct{ key, url string }

type cacheKey struct {
	key        string
	cols, rows int
}

// Manager owns the download queue, render cache and on-screen placement.
type Manager struct {
	Proto Protocol
	// Send delivers messages to the running program (set to tea.Program.Send).
	Send   func(tea.Msg)
	pathOf func(key string) string
	out    io.Writer
	client *http.Client
	ctx    context.Context
	cancel context.CancelFunc

	mu       sync.Mutex
	cond     *sync.Cond
	queue    []job
	pending  map[string]bool
	failed   map[string]bool
	urls     map[string]string
	rendered map[cacheKey]*Rendered
	order    []cacheKey
	working  map[cacheKey]bool
	ready    chan ReadyMsg
	cw, ch   int

	// placement state (event-loop goroutine only)
	cur       *placement
	want      *placement
	next      *placement // target of the scheduled draw
	scheduled bool
	stale     bool
	placedNow bool
	seq       int
	// kitty: images are transmitted once and then only placed.
	kittyIDs   map[cacheKey]int
	kittyOrder []cacheKey
	kittyNext  int
	kittyShown int
	kittyReset bool
}

type placement struct {
	key              string
	x, y, cols, rows int
	r                *Rendered
}

func (p *placement) same(o *placement) bool {
	return p != nil && o != nil && p.key == o.key && p.x == o.x && p.y == o.y && p.cols == o.cols && p.rows == o.rows
}

// New starts workers downloading into files named by pathOf.
func New(proto Protocol, pathOf func(string) string, out io.Writer, workers int) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		Proto:     proto,
		pathOf:    pathOf,
		out:       out,
		client:    &http.Client{Timeout: 15 * time.Second},
		ctx:       ctx,
		cancel:    cancel,
		pending:   map[string]bool{},
		failed:    map[string]bool{},
		urls:      map[string]string{},
		rendered:  map[cacheKey]*Rendered{},
		working:   map[cacheKey]bool{},
		ready:     make(chan ReadyMsg, 256),
		kittyIDs:  map[cacheKey]int{},
		kittyNext: 4242,
	}
	m.cond = sync.NewCond(&m.mu)
	m.cw, m.ch = cellSize()
	if m.cw == 0 || m.ch == 0 {
		m.cw, m.ch = 10, 20
	}
	for range workers {
		go m.worker()
	}
	return m
}

// Wait delivers the next ReadyMsg.
func (m *Manager) Wait() tea.Cmd {
	return func() tea.Msg {
		select {
		case r := <-m.ready:
			return r
		case <-m.ctx.Done():
			return nil
		}
	}
}

func (m *Manager) notify(key string) {
	select {
	case m.ready <- ReadyMsg{Key: key}:
	default:
	}
}

// CellAspect returns cell height / width.
func (m *Manager) CellAspect() float64 { return float64(m.ch) / float64(m.cw) }

func exists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Size() > 0
}

// Request queues a download. front puts it ahead of the queue (selected item).
func (m *Manager) Request(key, url string, front bool) {
	if key == "" || url == "" || m.Proto == None {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.urls[key] = url
	if m.pending[key] || m.failed[key] || exists(m.pathOf(key)) {
		if front && m.pending[key] {
			for i, j := range m.queue {
				if j.key == key {
					m.queue = append(append([]job{j}, m.queue[:i]...), m.queue[i+1:]...)
					break
				}
			}
		}
		return
	}
	m.pending[key] = true
	if front {
		m.queue = append([]job{{key, url}}, m.queue...)
	} else {
		m.queue = append(m.queue, job{key, url})
	}
	m.cond.Signal()
}

func (m *Manager) worker() {
	for {
		m.mu.Lock()
		for len(m.queue) == 0 && m.ctx.Err() == nil {
			m.cond.Wait()
		}
		if m.ctx.Err() != nil {
			m.mu.Unlock()
			return
		}
		j := m.queue[0]
		m.queue = m.queue[1:]
		m.mu.Unlock()

		err := m.fetch(j)
		m.mu.Lock()
		delete(m.pending, j.key)
		if err != nil {
			m.failed[j.key] = true
		}
		m.mu.Unlock()
		m.notify(j.key) // on failure too, so "loading" turns into "no thumbnail"
	}
}

func (m *Manager) fetch(j job) error {
	if strings.HasPrefix(j.url, "/") {
		return m.local(j)
	}
	req, err := http.NewRequestWithContext(m.ctx, http.MethodGet, j.url, nil)
	if err != nil {
		return err
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	dst := m.pathOf(j.key)
	_ = os.MkdirAll(filepath.Dir(dst), 0o755)
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".dl-*")
	if err != nil {
		return err
	}
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	tmp.Close()
	return os.Rename(tmp.Name(), dst)
}

// local makes a thumbnail for a file on disk: images are used as-is,
// media files get a frame (or embedded cover art) extracted with ffmpeg.
func (m *Manager) local(j job) error {
	dst := m.pathOf(j.key)
	_ = os.MkdirAll(filepath.Dir(dst), 0o755)
	switch strings.ToLower(filepath.Ext(j.url)) {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
		data, err := os.ReadFile(j.url)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0o644)
	}
	ctx, cancel := context.WithTimeout(m.ctx, 20*time.Second)
	defer cancel()
	tmp := dst + ".tmp.jpg"
	defer os.Remove(tmp)
	// -ss before -i seeks fast; audio files fall back to their cover art stream.
	for _, args := range [][]string{
		{"-ss", "15", "-i", j.url, "-frames:v", "1", "-vf", "scale=480:-2"},
		{"-i", j.url, "-an", "-frames:v", "1", "-vf", "scale=480:-2"},
	} {
		cmd := exec.CommandContext(ctx, "ffmpeg", append(append([]string{"-v", "error", "-y"}, args...), tmp)...)
		if err := cmd.Run(); err == nil && exists(tmp) {
			return os.Rename(tmp, dst)
		}
	}
	return fmt.Errorf("no frame in %s", j.url)
}

// Status describes a key: "ready", "loading" or "failed".
func (m *Manager) Status(key string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch {
	case exists(m.pathOf(key)):
		return "ready"
	case m.failed[key]:
		return "failed"
	}
	return "loading"
}

// Get returns a rendered thumbnail for the cell area, rendering it in the
// background (and returning nil) the first time.
func (m *Manager) Get(key string, cols, rows int) *Rendered {
	if cols <= 0 || rows <= 0 || m.Proto == None {
		return nil
	}
	ck := cacheKey{key, cols, rows}
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.rendered[ck]; ok {
		return r
	}
	path := m.pathOf(key)
	if m.working[ck] || !exists(path) {
		return nil
	}
	m.working[ck] = true
	go func() {
		r, err := render(m.Proto, path, cols, rows, m.cw, m.ch)
		m.mu.Lock()
		delete(m.working, ck)
		if err != nil {
			m.failed[key] = true
			os.Remove(path)
		} else {
			m.rendered[ck] = r
			m.order = append(m.order, ck)
			if len(m.order) > 96 {
				delete(m.rendered, m.order[0])
				m.order = m.order[1:]
			}
		}
		m.mu.Unlock()
		m.notify(key)
	}()
	return nil
}

// Invalidate marks the on-screen placement stale (after resize or clear),
// so the next frame redraws it.
func (m *Manager) Invalidate() {
	m.mu.Lock()
	cw, ch := cellSize()
	if cw > 0 && ch > 0 && (cw != m.cw || ch != m.ch) {
		m.cw, m.ch = cw, ch
		m.rendered = map[cacheKey]*Rendered{}
		m.order = nil
	}
	m.mu.Unlock()
	// Keep cur so the old image is still erased (kitty images outlive a
	// screen clear); stale forces the next frame to redraw.
	m.stale = true
	// Some terminals (e.g. Ghostty) free stored images without placements
	// when the screen is cleared, so placing them by id would silently
	// fail. Re-transmit after any clear or resize.
	m.kittyReset = true
}

// BeginFrame must be called before rendering a view.
func (m *Manager) BeginFrame() { m.placedNow = false }

// Place reserves an overlay at absolute cell position (x, y) for this frame.
// It returns true when the area is covered by an overlay image, in which
// case the caller must emit cursor-forward escapes (not spaces) for those
// cells so redraws of the line don't erase the image.
func (m *Manager) Place(key string, x, y, cols, rows int) bool {
	if !m.Proto.Overlay() {
		return false
	}
	r := m.Get(key, cols, rows)
	if r == nil {
		return false
	}
	m.placedNow = true
	m.want = &placement{key: key, x: x, y: y, cols: cols, rows: rows, r: r}
	return true
}

// EndFrame schedules a draw or clear if the placement changed. View can't
// return commands, so the draw tick is delivered through Send. A draw is
// scheduled once per change: rescheduling on every frame would starve it,
// since each tick itself triggers another frame.
func (m *Manager) EndFrame() {
	if !m.Proto.Overlay() || m.Send == nil {
		return
	}
	if !m.placedNow {
		m.want = nil
	}
	target := m.cur
	if m.scheduled {
		target = m.next
	}
	unchanged := m.want.same(target) || (m.want == nil && target == nil)
	if unchanged && !(m.stale && !m.scheduled) {
		return
	}
	m.next, m.scheduled = m.want, true
	m.seq++
	seq, send := m.seq, m.Send
	// Delay so frames already queued in the renderer are flushed first;
	// this also debounces fast scrolling.
	time.AfterFunc(40*time.Millisecond, func() { send(drawMsg{seq}) })
}

// IsDraw reports whether msg is a draw tick for this manager.
func IsDraw(msg tea.Msg) bool { _, ok := msg.(drawMsg); return ok }

// HandleDraw performs a pending overlay draw.
func (m *Manager) HandleDraw(msg tea.Msg) {
	d, ok := msg.(drawMsg)
	if !ok || d.seq != m.seq {
		return
	}
	m.scheduled = false
	w := m.next
	if w.same(m.cur) && !m.stale {
		return
	}
	m.stale = false
	var b bytes.Buffer
	b.WriteString("\x1b7") // save cursor
	m.removeKitty(&b)
	if m.kittyReset && m.Proto == Kitty {
		for _, id := range m.kittyIDs {
			fmt.Fprintf(&b, "\x1b_Ga=d,d=I,i=%d,q=2\x1b\\", id) // free whatever survived
		}
		m.kittyIDs, m.kittyOrder = map[cacheKey]int{}, nil
	}
	m.kittyReset = false
	if w != nil {
		if m.Proto != Kitty {
			blank(&b, w)
		}
		fmt.Fprintf(&b, "\x1b[%d;%dH", w.y+1, w.x+w.r.OffX+1)
		if m.Proto == Kitty {
			m.kittyShown = m.kittyPlace(&b, cacheKey{w.key, w.cols, w.rows}, w.r)
		} else {
			b.Write(w.r.Data)
		}
	}
	b.WriteString("\x1b8") // restore cursor
	_, _ = m.out.Write(b.Bytes())
	m.cur = w
}

// kittyPlace transmits an image the first time it is shown and places it
// by id afterwards, keeping at most 32 images in the terminal's memory.
func (m *Manager) kittyPlace(b *bytes.Buffer, ck cacheKey, r *Rendered) int {
	id, ok := m.kittyIDs[ck]
	if !ok {
		m.kittyNext++
		id = m.kittyNext
		m.kittyIDs[ck] = id
		m.kittyOrder = append(m.kittyOrder, ck)
		b.Write(bytes.Replace(r.Data, []byte("%ID%"), []byte(strconv.Itoa(id)), 1))
		if len(m.kittyOrder) > 32 {
			old := m.kittyOrder[0]
			m.kittyOrder = m.kittyOrder[1:]
			fmt.Fprintf(b, "\x1b_Ga=d,d=I,i=%d,q=2\x1b\\", m.kittyIDs[old])
			delete(m.kittyIDs, old)
		}
	}
	fmt.Fprintf(b, "\x1b_Ga=p,i=%d,p=1,C=1,q=2\x1b\\", id)
	return id
}

// removeKitty deletes the visible kitty placement. Kitty images live on
// their own layer, so no cells need to be touched.
func (m *Manager) removeKitty(b *bytes.Buffer) {
	if m.Proto == Kitty && m.kittyShown != 0 {
		// lowercase i: remove the placement, keep the image data for reuse
		fmt.Fprintf(b, "\x1b_Ga=d,d=i,i=%d,q=2\x1b\\", m.kittyShown)
		m.kittyShown = 0
	}
}

// blank clears the cells of p before drawing into it, so a smaller image
// doesn't leave parts of the previous one behind. Only call it for the
// placement being drawn: those cells are reserved (cursor-forward) in the
// current frame. Any other cells belong to the renderer, and blanking them
// would erase text it has already drawn and won't redraw.
func blank(b *bytes.Buffer, p *placement) {
	spaces := strings.Repeat(" ", p.cols)
	for row := range p.rows {
		fmt.Fprintf(b, "\x1b[%d;%dH%s", p.y+row+1, p.x+1, spaces)
	}
}

// Close stops workers and removes any overlay.
func (m *Manager) Close() {
	if m.Proto == Kitty && len(m.kittyIDs) > 0 {
		// free every image we transmitted
		var b bytes.Buffer
		for _, id := range m.kittyIDs {
			fmt.Fprintf(&b, "\x1b_Ga=d,d=I,i=%d,q=2\x1b\\", id)
		}
		_, _ = m.out.Write(b.Bytes())
	}
	m.cancel()
	m.mu.Lock()
	m.cond.Broadcast()
	m.mu.Unlock()
}
