// Package config reads and writes yt-x.conf ("KEY: value" lines, # comments).
// The format is identical to the Bash version so existing configs keep working.
package config

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"github.com/jkpair/yt-x-plus/internal/app"
)

type Config struct {
	AutoLoadedPlugins    []string
	PrettyPrint          bool
	Theme                string
	Editor               string
	VideoQuality         int
	EnablePreview        bool
	ImageRenderer        string
	DisownStreaming      bool
	UpdateRecent         bool
	SearchHistory        bool
	NoOfRecent           int
	Player               string
	PreferredBrowser     string
	CookieFile           string
	NoOfSearchResults    int
	FeedPageSize         int
	FeedCacheTTL         int
	NotificationDuration int
	DownloadDirectory    string
	WelcomeScreen        bool
	SponsorBlock         bool
	SponsorBlockCats     string
	MarkWatched          bool
	LiveFromStart        bool
	ConcurrentFragments  int
	DownloadRetries      int
	EmbedThumbnail       bool
	EmbedSubs            bool
	EmbedChapters        bool
	PreferredCodec       string
	PreferHDR            bool
	SubtitleLangs        string
	ResumePlayback       bool

	path string
	raw  map[string]string
}

// Load parses path, creating it from the template (and reporting firstRun) when empty or missing.
func Load(path string) (cfg *Config, firstRun bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, false, err
	}
	raw := Parse(data)
	cfg = fromRaw(raw)
	cfg.path = path
	if len(bytes.TrimSpace(data)) == 0 {
		firstRun = true
		if err := cfg.Save(); err != nil {
			return cfg, firstRun, err
		}
	}
	return cfg, firstRun, nil
}

// Parse extracts KEY: value pairs, ignoring comments.
func Parse(data []byte) map[string]string {
	raw := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		if k == "" || strings.ContainsAny(k, " \t") {
			continue
		}
		raw[k] = strings.TrimSpace(v)
	}
	return raw
}

func str(raw map[string]string, k, def string) string {
	if v := raw[k]; v != "" {
		return v
	}
	return def
}

func boolean(raw map[string]string, k string, def bool) bool {
	switch strings.ToLower(raw[k]) {
	case "true":
		return true
	case "false":
		return false
	}
	return def
}

var nonDigit = regexp.MustCompile(`[^0-9]`)

// Numeric mirrors validate_numeric: strip non-digits, default if empty, clamp.
func Numeric(v string, def, lo, hi int) int {
	v = nonDigit.ReplaceAllString(v, "")
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return min(max(n, lo), hi)
}

func fromRaw(raw map[string]string) *Config {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "open"
	}
	dl := app.ExpandHome(raw["DOWNLOAD_DIRECTORY"])
	if dl == "" {
		vids := os.Getenv("XDG_VIDEOS_DIR")
		if vids == "" {
			vids = "~/Videos"
		}
		dl = filepath.Join(app.ExpandHome(vids), app.Name())
	}
	var plugins []string
	for _, p := range strings.Split(raw["AUTO_LOADED_EXTENSIONS"], ",") {
		if p = strings.TrimSpace(p); p != "" {
			plugins = append(plugins, p)
		}
	}
	return &Config{
		raw:                  raw,
		AutoLoadedPlugins:    plugins,
		PrettyPrint:          boolean(raw, "PRETTY_PRINT", true),
		Theme:                str(raw, "THEME", "everforest-dark-hard"),
		Editor:               str(raw, "EDITOR", editor),
		VideoQuality:         Numeric(raw["VIDEO_QUALITY"], 1080, 144, 4320),
		EnablePreview:        boolean(raw, "ENABLE_PREVIEW", false),
		ImageRenderer:        str(raw, "IMAGE_RENDERER", "auto"),
		DisownStreaming:      boolean(raw, "DISOWN_STREAMING_PROCESS", true),
		UpdateRecent:         boolean(raw, "UPDATE_RECENT", true),
		SearchHistory:        boolean(raw, "SEARCH_HISTORY", true),
		NoOfRecent:           Numeric(raw["NO_OF_RECENT"], 30, 1, 500),
		Player:               str(raw, "PLAYER", "mpv"),
		PreferredBrowser:     raw["PREFERRED_BROWSER"],
		CookieFile:           app.ExpandHome(raw["COOKIE_FILE"]),
		NoOfSearchResults:    Numeric(raw["NO_OF_SEARCH_RESULTS"], 30, 1, 100),
		FeedPageSize:         Numeric(raw["FEED_PAGE_SIZE"], 25, 1, 100),
		FeedCacheTTL:         Numeric(raw["FEED_CACHE_TTL"], 900, 0, 86400),
		NotificationDuration: Numeric(raw["NOTIFICATION_DURATION"], 5, 1, 30),
		DownloadDirectory:    dl,
		WelcomeScreen:        boolean(raw, "WELCOME_SCREEN", true),
		SponsorBlock:         boolean(raw, "SPONSORBLOCK", false),
		SponsorBlockCats:     str(raw, "SPONSORBLOCK_CATS", "sponsor,selfpromo"),
		MarkWatched:          boolean(raw, "MARK_WATCHED", false),
		LiveFromStart:        boolean(raw, "LIVE_FROM_START", false),
		ConcurrentFragments:  Numeric(raw["CONCURRENT_FRAGMENTS"], 1, 1, 16),
		DownloadRetries:      Numeric(raw["DOWNLOAD_RETRIES"], 3, 0, 10),
		EmbedThumbnail:       boolean(raw, "EMBED_THUMBNAIL", false),
		EmbedSubs:            boolean(raw, "EMBED_SUBS", false),
		EmbedChapters:        boolean(raw, "EMBED_CHAPTERS", false),
		PreferredCodec:       str(raw, "PREFERRED_CODEC", "any"),
		PreferHDR:            boolean(raw, "PREFER_HDR", false),
		SubtitleLangs:        raw["SUBTITLE_LANGS"],
		ResumePlayback:       boolean(raw, "RESUME_PLAYBACK", true),
	}
}

// Path returns the file this config was loaded from.
func (c *Config) Path() string { return c.path }

// Get returns a raw value by key (used by the plugin API).
func (c *Config) Get(key string) string { return c.raw[key] }

// Set updates key in place in the config file (appending it if absent) and reloads.
func (c *Config) Set(key, value string) error {
	data, err := os.ReadFile(c.path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	found := false
	for i, l := range lines {
		if strings.HasPrefix(l, key+":") {
			lines[i] = key + ": " + value
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, key+": "+value)
	}
	if err := os.WriteFile(c.path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return err
	}
	c.raw[key] = value
	path := c.path
	*c = *fromRaw(c.raw)
	c.path = path
	return nil
}

// Save writes the full commented template with current values.
func (c *Config) Save() error {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, c); err != nil {
		return err
	}
	return os.WriteFile(c.path, buf.Bytes(), 0o644)
}

var tmpl = template.Must(template.New("conf").Funcs(template.FuncMap{
	"join": func(s []string) string { return strings.Join(s, ",") },
}).Parse(confTemplate))

// String is for debugging.
func (c *Config) String() string { return fmt.Sprintf("%+v", *c) }
