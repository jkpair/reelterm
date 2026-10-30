package ytdlp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/jkpair/yt-x-plus/internal/config"
)

// ErrBotCheck is returned when YouTube asks to "confirm you're not a bot".
var ErrBotCheck = errors.New("youtube bot detection triggered: configure PREFERRED_BROWSER or COOKIE_FILE")

// Client runs yt-dlp with the user's cookie settings.
type Client struct {
	Bin        string
	Browser    string
	CookieFile string
	Debug      bool
	usingFile  bool
}

func New(cfg *config.Config) *Client {
	return &Client{
		Bin:        "yt-dlp",
		Browser:    cfg.PreferredBrowser,
		CookieFile: cfg.CookieFile,
		Debug:      os.Getenv("YT_X_DEBUG") != "",
	}
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// CookieOpts returns --cookies-from-browser / --cookies args (PREFERRED_BROWSER_OPTS).
func (c *Client) CookieOpts() []string {
	switch {
	case c.Browser != "" && !c.usingFile:
		return []string{"--cookies-from-browser", c.Browser}
	case c.CookieFile != "" && fileExists(c.CookieFile):
		return []string{"--cookies", c.CookieFile}
	}
	return nil
}

// HasCookies reports whether any authentication is configured.
func (c *Client) HasCookies() bool { return len(c.CookieOpts()) > 0 }

// MpvOpts returns the --ytdl-* options passed to mpv (MPV_YTDL_OPTS).
func (c *Client) MpvOpts() []string {
	opts := []string{"--ytdl-format=bestvideo+bestaudio/best"}
	switch {
	case c.Browser != "" && !c.usingFile:
		opts = append([]string{"--ytdl-raw-options=cookies-from-browser=" + c.Browser}, opts...)
	case c.CookieFile != "" && fileExists(c.CookieFile):
		opts = append([]string{"--ytdl-raw-options=cookies=" + c.CookieFile}, opts...)
	}
	return opts
}

// IsFeed mirrors is_youtube_feed.
func IsFeed(url string) bool {
	return strings.Contains(url, "/feed/") || url == "https://www.youtube.com" || strings.HasPrefix(url, "ytsearch")
}

// Range limits how many entries are fetched. Zero values mean unbounded.
type Range struct{ Start, End int }

func (c *Client) run(ctx context.Context, args []string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, c.Bin, args...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if c.Debug {
		fmt.Fprintln(os.Stderr, "[DEBUG] yt-dlp", strings.Join(args, " "))
	}
	err := cmd.Run()
	return out.Bytes(), errb.Bytes(), err
}

// Fetch runs `yt-dlp URL -J --flat-playlist` (run_yt_dlp), falling back to the
// cookie file when browser cookie extraction fails.
func (c *Client) Fetch(ctx context.Context, url string, r Range) (*Playlist, error) {
	args := []string{url, "-J", "--flat-playlist", "--extractor-args", "youtubetab:approximate_date"}
	if r.Start > 1 {
		args = append(args, "--playlist-start", strconv.Itoa(r.Start))
	}
	if r.End > 0 {
		args = append(args, "--playlist-end", strconv.Itoa(r.End))
	}
	pl, stderr, err := c.fetch(ctx, append(args, c.CookieOpts()...))
	if err != nil && !c.usingFile && c.Browser != "" && c.CookieFile != "" && fileExists(c.CookieFile) && ctx.Err() == nil {
		c.usingFile = true
		pl, stderr, err = c.fetch(ctx, append(args, c.CookieOpts()...))
	}
	if err != nil && bytes.Contains(stderr, []byte("not a bot")) {
		return nil, ErrBotCheck
	}
	return pl, err
}

// FetchArgs runs an arbitrary yt-dlp argument list that prints playlist JSON (custom commands).
func (c *Client) FetchArgs(ctx context.Context, args []string) (*Playlist, error) {
	pl, _, err := c.fetch(ctx, args)
	return pl, err
}

func (c *Client) fetch(ctx context.Context, args []string) (*Playlist, []byte, error) {
	out, stderr, runErr := c.run(ctx, args)
	var pl Playlist
	if err := json.Unmarshal(out, &pl); err != nil || pl.Entries == nil {
		if runErr == nil {
			runErr = errors.New("no entries in yt-dlp output")
		}
		msg := lastLine(stderr)
		if msg != "" {
			runErr = fmt.Errorf("%w: %s", runErr, msg)
		}
		return nil, stderr, runErr
	}
	pl.FillChannel()
	for i := range pl.Entries {
		pl.Entries[i].Clean()
	}
	return &pl, stderr, nil
}

func lastLine(b []byte) string {
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// Video fetches full metadata for one video (`yt-dlp -j --skip-download`).
func (c *Client) Video(ctx context.Context, id string) (*Entry, []byte, error) {
	args := append([]string{"https://www.youtube.com/watch?v=" + id, "-j", "--no-download", "--skip-download", "--no-warnings"}, c.CookieOpts()...)
	out, _, err := c.run(ctx, args)
	if err != nil {
		return nil, nil, err
	}
	var e Entry
	if err := json.Unmarshal(out, &e); err != nil {
		return nil, nil, err
	}
	e.Clean()
	return &e, out, nil
}

// StreamURL resolves a direct media URL for players other than mpv.
func (c *Client) StreamURL(ctx context.Context, url, format string) (string, error) {
	args := []string{url, "-q", "--no-warnings", "--get-url"}
	if format != "" {
		args = append(args, "--format", format)
	}
	out, _, err := c.run(ctx, append(args, c.CookieOpts()...))
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if lines[len(lines)-1] == "" {
		return "", errors.New("no stream url")
	}
	return lines[len(lines)-1], nil
}

// Version returns `yt-dlp --version`.
func (c *Client) Version(ctx context.Context) (string, error) {
	out, _, err := c.run(ctx, []string{"--version"})
	return strings.TrimSpace(string(out)), err
}

// Mode selects which options BuildOpts emits.
type Mode int

const (
	ModeStream Mode = iota
	ModeDownload
	ModeAudio
)

// BuildOpts ports build_ytdlp_opts.
func (c *Client) BuildOpts(cfg *config.Config, mode Mode) []string {
	var o []string
	if cfg.SponsorBlock {
		o = append(o, "--sponsorblock-remove", cfg.SponsorBlockCats)
	}
	if cfg.ConcurrentFragments > 1 {
		o = append(o, "-N", strconv.Itoa(cfg.ConcurrentFragments))
	}
	if cfg.DownloadRetries > 0 {
		r := strconv.Itoa(cfg.DownloadRetries)
		o = append(o, "--retries", r, "--fragment-retries", r)
	}
	if cfg.LiveFromStart {
		o = append(o, "--live-from-start")
	}
	if cfg.MarkWatched && c.HasCookies() {
		o = append(o, "--mark-watched")
	}
	if mode == ModeDownload || mode == ModeAudio {
		o = append(o, "--windows-filenames")
		if cfg.EmbedThumbnail {
			o = append(o, "--embed-thumbnail")
		}
		if cfg.EmbedChapters {
			o = append(o, "--embed-chapters")
		}
		if cfg.EmbedSubs && mode == ModeDownload {
			o = append(o, "--embed-subs")
			if cfg.SubtitleLangs != "" {
				o = append(o, "--sub-langs", cfg.SubtitleLangs)
			}
		}
	}
	if mode == ModeDownload {
		switch cfg.PreferredCodec {
		case "av1", "vp9", "h264":
			o = append(o, "--format-sort", "vcodec:"+cfg.PreferredCodec)
		}
		if cfg.PreferHDR {
			o = append(o, "--format-sort", "hdr")
		}
	}
	return o
}
