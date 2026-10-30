// Package cache persists feeds, metadata, history and the local library.
// File names and JSON shapes match the Bash version so existing data carries over.
package cache

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jkpair/reelterm/internal/app"
	"github.com/jkpair/reelterm/internal/ytdlp"
)

// Store gives typed access to everything reelterm keeps on disk.
type Store struct {
	P  app.Paths
	mu sync.Mutex
}

func New(p app.Paths) *Store { return &Store{P: p} }

// WriteAtomic writes via a temp file and rename so readers never see partial data.
func WriteAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func writeJSON(path string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return WriteAtomic(path, data)
}

// Hash returns the sha256 hex of s (generate_sha256).
func Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// --- Feed cache ---

func (s *Store) feedPath() string   { return filepath.Join(s.P.CacheDir, "feed_cache.json") }
func (s *Store) feedTSPath() string { return filepath.Join(s.P.CacheDir, "feed_cache.timestamp") }

// LoadFeed returns the cached home feed.
func (s *Store) LoadFeed() (*ytdlp.Playlist, error) {
	var pl ytdlp.Playlist
	if err := readJSON(s.feedPath(), &pl); err != nil {
		return nil, err
	}
	if pl.Entries == nil {
		return nil, os.ErrNotExist
	}
	return &pl, nil
}

// SaveFeed stores the feed and stamps the time.
func (s *Store) SaveFeed(pl *ytdlp.Playlist) error {
	if err := writeJSON(s.feedPath(), pl); err != nil {
		return err
	}
	return os.WriteFile(s.feedTSPath(), []byte(strconv.FormatInt(time.Now().Unix(), 10)), 0o644)
}

// FeedStale reports whether the feed cache is older than ttl.
func (s *Store) FeedStale(ttl time.Duration) bool {
	data, err := os.ReadFile(s.feedTSPath())
	if err != nil {
		return true
	}
	ts, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return true
	}
	return time.Since(time.Unix(ts, 0)) >= ttl
}

// MergeFeed prepends entries from fresh that aren't already in old (merge_feed_entries).
func MergeFeed(old, fresh []ytdlp.Entry) []ytdlp.Entry {
	seen := map[string]bool{}
	for _, e := range old {
		seen[e.Key()] = true
	}
	var out []ytdlp.Entry
	for _, e := range fresh {
		if !seen[e.Key()] {
			out = append(out, e)
		}
	}
	return append(out, old...)
}

// --- Metadata cache ---

func (s *Store) metaPath(id string) string {
	return filepath.Join(s.P.CacheDir, "metadata_"+id+".json")
}

// Meta loads cached full metadata for a video.
func (s *Store) Meta(id string) (*ytdlp.Entry, bool) {
	var e ytdlp.Entry
	if err := readJSON(s.metaPath(id), &e); err != nil {
		return nil, false
	}
	return &e, true
}

// SaveMetaRaw stores raw `yt-dlp -j` output.
func (s *Store) SaveMetaRaw(id string, raw []byte) error { return WriteAtomic(s.metaPath(id), raw) }

// --- Thumbnails ---

// ThumbPath returns where a thumbnail is cached. Keyed by entry key (video or
// channel ID) so different videos with identical titles never collide.
func (s *Store) ThumbPath(key string) string {
	return filepath.Join(s.P.ThumbsDir, Hash(key)[:32]+".img")
}

// --- Entry lists (recent.json, saved_videos.json, subscriptions.json) ---

type entryFile struct {
	Entries []ytdlp.Entry `json:"entries"`
}

func (s *Store) loadEntries(path string) []ytdlp.Entry {
	var f entryFile
	_ = readJSON(path, &f)
	return f.Entries
}

func (s *Store) saveEntries(path string, e []ytdlp.Entry) error {
	if e == nil {
		e = []ytdlp.Entry{}
	}
	return writeJSON(path, entryFile{Entries: e})
}

func (s *Store) recentPath() string { return filepath.Join(s.P.ConfigDir, "recent.json") }
func (s *Store) savedPath() string  { return filepath.Join(s.P.ConfigDir, "saved_videos.json") }
func (s *Store) subsPath() string   { return filepath.Join(s.P.ConfigDir, "subscriptions.json") }

func upsert(list []ytdlp.Entry, e ytdlp.Entry) []ytdlp.Entry {
	list = slices.DeleteFunc(list, func(x ytdlp.Entry) bool { return x.ID == e.ID })
	return append(list, e)
}

func reversed(e []ytdlp.Entry) []ytdlp.Entry {
	out := slices.Clone(e)
	slices.Reverse(out)
	return out
}

// Recent returns recently played videos, newest first.
func (s *Store) Recent() []ytdlp.Entry { return reversed(s.loadEntries(s.recentPath())) }

// AddRecent records a played video, keeping at most limit.
func (s *Store) AddRecent(e ytdlp.Entry, limit int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := upsert(s.loadEntries(s.recentPath()), e)
	if len(list) > limit {
		list = list[len(list)-limit:]
	}
	return s.saveEntries(s.recentPath(), list)
}

// Saved returns saved videos, newest first.
func (s *Store) Saved() []ytdlp.Entry { return reversed(s.loadEntries(s.savedPath())) }

// IsSaved reports whether id is in the saved list.
func (s *Store) IsSaved(id string) bool {
	return slices.ContainsFunc(s.loadEntries(s.savedPath()), func(x ytdlp.Entry) bool { return x.ID == id })
}

func (s *Store) Save(e ytdlp.Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveEntries(s.savedPath(), upsert(s.loadEntries(s.savedPath()), e))
}

func (s *Store) Unsave(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := slices.DeleteFunc(s.loadEntries(s.savedPath()), func(x ytdlp.Entry) bool { return x.ID == id })
	return s.saveEntries(s.savedPath(), list)
}

// Subscriptions returns locally stored channels, and whether the file exists.
func (s *Store) Subscriptions() ([]ytdlp.Entry, bool) {
	if _, err := os.Stat(s.subsPath()); err != nil {
		return nil, false
	}
	return s.loadEntries(s.subsPath()), true
}

func (s *Store) SetSubscriptions(e []ytdlp.Entry) error { return s.saveEntries(s.subsPath(), e) }

func (s *Store) Subscribe(ch ytdlp.Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, _ := s.Subscriptions()
	return s.saveEntries(s.subsPath(), upsert(list, ch))
}

// --- Search history ---

// History returns search terms, newest first.
func (s *Store) History() []string {
	f, err := os.Open(s.P.SearchHistory)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if t := strings.TrimSpace(sc.Text()); t != "" {
			out = append(out, t)
		}
	}
	slices.Reverse(out)
	return out
}

// AddHistory moves term to the end of the history file.
func (s *Store) AddHistory(term string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	h := s.History()
	slices.Reverse(h)
	h = slices.DeleteFunc(h, func(x string) bool { return x == term })
	h = append(h, term)
	return WriteAtomic(s.P.SearchHistory, []byte(strings.Join(h, "\n")+"\n"))
}

func (s *Store) ClearHistory() error {
	err := os.Remove(s.P.SearchHistory)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// --- Custom playlists and commands ---

type CustomPlaylist struct {
	Name             string `json:"name"`
	PlaylistURL      string `json:"playlistUrl"`
	PlaylistWatchURL string `json:"playlistWatchUrl"`
}

type CustomCmd struct {
	Name string `json:"name"`
	Cmd  string `json:"cmd"`
}

// CustomPlaylists returns saved playlists, newest first.
func (s *Store) CustomPlaylists() []CustomPlaylist {
	var v []CustomPlaylist
	_ = readJSON(s.P.CustomLists, &v)
	slices.Reverse(v)
	return v
}

func (s *Store) AddCustomPlaylist(name, playlistID string) error {
	var v []CustomPlaylist
	_ = readJSON(s.P.CustomLists, &v)
	v = append(v, CustomPlaylist{
		Name:             name,
		PlaylistURL:      "https://www.youtube.com/playlist?list=" + playlistID,
		PlaylistWatchURL: "https://www.youtube.com/watch?list=" + playlistID,
	})
	return writeJSON(s.P.CustomLists, v)
}

func (s *Store) CustomCmds() []CustomCmd {
	var v []CustomCmd
	_ = readJSON(s.P.CustomCmds, &v)
	return v
}

func (s *Store) AddCustomCmd(name, cmd string) error {
	v := s.CustomCmds()
	v = append(v, CustomCmd{Name: name, Cmd: cmd})
	return writeJSON(s.P.CustomCmds, v)
}

// --- Generated m3u playlists ---

// M3U writes (or reuses) a cached m3u8 for a playlist key and returns its path.
func (s *Store) M3U(key string, entries []ytdlp.Entry) (string, error) {
	p := filepath.Join(s.P.PlaylistsDir, Hash(key)+".m3u8")
	if st, err := os.Stat(p); err == nil && st.Size() > 0 {
		return p, nil
	}
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	for _, e := range entries {
		b.WriteString("#EXTINF:-1," + e.Title + "\n" + e.WatchURL() + "\n\n")
	}
	return p, WriteAtomic(p, []byte(b.String()))
}

// HasM3U returns the cached m3u path for key if present.
func (s *Store) HasM3U(key string) (string, bool) {
	p := filepath.Join(s.P.PlaylistsDir, Hash(key)+".m3u8")
	st, err := os.Stat(p)
	return p, err == nil && st.Size() > 0
}

// Cleanup removes thumbnails older than maxAge.
func (s *Store) Cleanup(maxAge time.Duration) {
	files, _ := os.ReadDir(s.P.ThumbsDir)
	for _, f := range files {
		if info, err := f.Info(); err == nil && time.Since(info.ModTime()) > maxAge {
			os.Remove(filepath.Join(s.P.ThumbsDir, f.Name()))
		}
	}
}
