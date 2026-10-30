// Package download runs yt-dlp downloads in the background and reports progress.
package download

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
)

// Msg reports progress or completion of a job.
type Msg struct {
	ID      int
	Title   string
	Item    string
	Percent float64
	Done    bool
	Err     error
}

// Job is one yt-dlp invocation.
type Job struct {
	ID      int
	Title   string
	Args    []string
	Item    string
	Percent float64
	Done    bool
	Err     error
	cmd     *exec.Cmd
}

type Manager struct {
	Bin  string
	mu   sync.Mutex
	jobs map[int]*Job
	next int
	ch   chan Msg
}

func New() *Manager {
	return &Manager{Bin: "yt-dlp", jobs: map[int]*Job{}, ch: make(chan Msg, 64)}
}

// Wait returns a command that delivers the next progress message.
func (m *Manager) Wait() tea.Cmd {
	return func() tea.Msg { return <-m.ch }
}

const marker = "YTX|"

// Start launches yt-dlp with args (URL and options) in the background.
func (m *Manager) Start(title string, args []string) int {
	m.mu.Lock()
	m.next++
	j := &Job{ID: m.next, Title: title, Args: args}
	m.jobs[j.ID] = j
	m.mu.Unlock()

	full := append([]string{"--newline", "--progress-template", "download:" + marker + "%(progress._percent_str)s|%(info.title)s"}, args...)
	cmd := exec.Command(m.Bin, full...)
	j.cmd = cmd
	go m.run(j, cmd)
	return j.ID
}

func (m *Manager) run(j *Job, cmd *exec.Cmd) {
	stdout, err := cmd.StdoutPipe()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err == nil {
		err = cmd.Start()
	}
	if err != nil {
		m.finish(j, err)
		return
	}
	sc := bufio.NewScanner(stdout)
	for sc.Scan() {
		line := sc.Text()
		rest, ok := strings.CutPrefix(line, marker)
		if !ok {
			continue
		}
		pct, item, _ := strings.Cut(rest, "|")
		p, _ := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(pct), "%")), 64)
		m.mu.Lock()
		j.Percent, j.Item = p, item
		m.mu.Unlock()
		select {
		case m.ch <- Msg{ID: j.ID, Title: j.Title, Item: item, Percent: p}:
		default:
		}
	}
	err = cmd.Wait()
	if err != nil {
		lines := strings.Split(strings.TrimSpace(stderr.String()), "\n")
		if last := lines[len(lines)-1]; last != "" {
			err = fmt.Errorf("%s", strings.TrimPrefix(last, "ERROR: "))
		}
	}
	m.finish(j, err)
}

func (m *Manager) finish(j *Job, err error) {
	m.mu.Lock()
	j.Done, j.Err = true, err
	if err == nil {
		j.Percent = 100
	}
	m.mu.Unlock()
	m.ch <- Msg{ID: j.ID, Title: j.Title, Done: true, Err: err, Percent: j.Percent}
}

// Jobs returns a snapshot, newest first.
func (m *Manager) Jobs() []Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Job, 0, len(m.jobs))
	for _, j := range m.jobs {
		out = append(out, *j)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].ID > out[b].ID })
	return out
}

// Active counts running jobs.
func (m *Manager) Active() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, j := range m.jobs {
		if !j.Done {
			n++
		}
	}
	return n
}

// Summary is a short status-bar string, empty when idle.
func (m *Manager) Summary() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	n, total := 0, 0.0
	for _, j := range m.jobs {
		if !j.Done {
			n++
			total += j.Percent
		}
	}
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("%d downloading · %.0f%%", n, total/float64(n))
}

// Cancel stops a running job.
func (m *Manager) Cancel(id int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if j, ok := m.jobs[id]; ok && !j.Done && j.cmd != nil && j.cmd.Process != nil {
		_ = j.cmd.Process.Kill()
	}
}

// ClearFinished removes completed jobs from the list.
func (m *Manager) ClearFinished() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, j := range m.jobs {
		if j.Done {
			delete(m.jobs, id)
		}
	}
}

// KillAll stops every running job.
func (m *Manager) KillAll() {
	for _, j := range m.Jobs() {
		m.Cancel(j.ID)
	}
}
