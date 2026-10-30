// Package player builds and classifies mpv / vlc invocations.
package player

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"unicode/utf16"

	"github.com/jkpair/reelterm/internal/app"
	"github.com/jkpair/reelterm/internal/config"
	"github.com/jkpair/reelterm/internal/ytdlp"
)

// Request describes one playback.
type Request struct {
	URL       string
	Title     string
	AudioOnly bool
}

// Player turns requests into commands for the configured player.
type Player struct {
	Cfg *config.Config
	YT  *ytdlp.Client

	mu   sync.Mutex
	kids []*exec.Cmd
}

func New(cfg *config.Config, yt *ytdlp.Client) *Player { return &Player{Cfg: cfg, YT: yt} }

// MpvBin returns mpv or mpv.exe, whichever exists.
func MpvBin() string {
	if _, err := exec.LookPath("mpv"); err == nil {
		return "mpv"
	}
	if _, err := exec.LookPath("mpv.exe"); err == nil {
		return "mpv.exe"
	}
	return "mpv"
}

// Command builds the command for req. Players other than mpv (and Android)
// need a resolved stream URL, which costs one yt-dlp call.
func (p *Player) Command(ctx context.Context, req Request) (*exec.Cmd, error) {
	url := req.URL
	isFile := !strings.HasPrefix(url, "http")
	if (p.Cfg.Player != "mpv" || app.Platform() == "android") && !isFile {
		format := ""
		if req.AudioOnly {
			format = "bestaudio/best[height<=" + strconv.Itoa(p.Cfg.VideoQuality) + "]/best"
		}
		u, err := p.YT.StreamURL(ctx, url, format)
		if err != nil {
			return nil, errors.New("failed to get stream: the video may be private, unavailable, or region-locked")
		}
		url = u
	}
	if app.Platform() == "android" {
		if p.Cfg.Player == "vlc" {
			return exec.Command("am", "start", "--user", "0", "-a", "android.intent.action.VIEW", "-d", url,
				"-n", "org.videolan.vlc/org.videolan.vlc.gui.video.VideoPlayerActivity", "-e", "title", req.Title), nil
		}
		return exec.Command("am", "start", "--user", "0", "-a", "android.intent.action.VIEW", "-d", url, "-n", "is.xyz.mpv/.MPVActivity"), nil
	}
	if p.Cfg.Player == "vlc" {
		args := []string{url, "--video-title", req.Title}
		if req.AudioOnly {
			args = append(args, "--no-video")
		}
		return exec.Command(vlcBin(), args...), nil
	}
	args := append([]string{}, p.YT.MpvOpts()...)
	args = append(args, p.YT.BuildOpts(p.Cfg, ytdlp.ModeStream)...)
	args = mpvYtdlRaw(args)
	if p.Cfg.ResumePlayback {
		args = append(args, "--save-position-on-quit")
	}
	if req.Title != "" && !isFile {
		args = append(args, "--force-media-title="+req.Title)
	}
	args = append(args, url)
	if req.AudioOnly {
		args = append(args, "--no-video", "--force-window=no")
	}
	return exec.Command(MpvBin(), args...), nil
}

// mpvYtdlRaw folds yt-dlp style options (from BuildOpts) into mpv's
// --ytdl-raw-options so SponsorBlock etc. apply while streaming.
func mpvYtdlRaw(args []string) []string {
	var keep, raw []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "--ytdl") || !strings.HasPrefix(a, "-") {
			keep = append(keep, a)
			continue
		}
		name := strings.TrimLeft(a, "-")
		if name == "N" {
			name = "concurrent-fragments"
		}
		if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			raw = append(raw, name+"="+args[i+1])
			i++
		} else {
			raw = append(raw, name+"=")
		}
	}
	// One pair per flag: -append adds a single key=value, so values may contain commas.
	for _, r := range raw {
		keep = append(keep, "--ytdl-raw-options-append="+r)
	}
	return keep
}

// Capture tees stderr into a buffer so failures can be classified.
func Capture(cmd *exec.Cmd, echo io.Writer) *bytes.Buffer {
	var buf bytes.Buffer
	if echo != nil {
		cmd.Stderr = io.MultiWriter(echo, &buf)
	} else {
		cmd.Stderr = &buf
	}
	return &buf
}

// Start runs cmd in the background. With disown it survives reelterm exiting;
// otherwise it is killed by KillAll.
func (p *Player) Start(cmd *exec.Cmd, disown bool) error {
	cmd.Stdin = nil
	if cmd.Stdout == nil {
		cmd.Stdout = io.Discard
	}
	if disown {
		detach(cmd)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	if !disown {
		p.mu.Lock()
		p.kids = append(p.kids, cmd)
		p.mu.Unlock()
	}
	return nil
}

// KillAll stops non-disowned background players.
func (p *Player) KillAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, c := range p.kids {
		if c.Process != nil && c.ProcessState == nil {
			_ = c.Process.Kill()
		}
	}
	p.kids = nil
}

// Classify maps player stderr to a friendly message (play_with_mpv).
func Classify(stderr string) (msg string, bot bool) {
	s := strings.ToLower(stderr)
	switch {
	case strings.Contains(s, "not a bot"):
		return "YouTube bot detection triggered. Set a browser for cookies in Settings.", true
	case strings.Contains(s, "private video"):
		return "Playback failed: this video is private.", false
	case strings.Contains(s, "members-only") || strings.Contains(s, "member-only") || strings.Contains(s, "join this channel"):
		return "Playback failed: this video is for channel members only.", false
	case strings.Contains(s, "video unavailable") || strings.Contains(s, "this video is unavailable") || strings.Contains(s, "no longer available"):
		return "Playback failed: this video is unavailable (removed or region-locked).", false
	case strings.Contains(s, "age-restricted") || strings.Contains(s, "age_restricted") || strings.Contains(s, "confirm your age"):
		return "Playback failed: this video is age-restricted. Configure browser cookies.", false
	case strings.Contains(s, "live event has ended") || strings.Contains(s, "premieres in"):
		return "Playback failed: this live stream/premiere is not currently available.", false
	case strings.Contains(s, "removed by the uploader") || strings.Contains(s, "has been removed"):
		return "Playback failed: this video has been removed.", false
	}
	return "", false
}

// OpenURL opens a URL with the desktop handler.
func OpenURL(u string) error {
	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		// rundll32 avoids cmd.exe's "start" mangling URLs that contain &.
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	} else {
		for _, bin := range []string{"xdg-open", "open", "termux-open-url"} {
			if _, err := exec.LookPath(bin); err == nil {
				c = exec.Command(bin, u)
				break
			}
		}
	}
	if c == nil {
		return errors.New("could not find xdg-open or open")
	}
	c.Stdout, c.Stderr = io.Discard, io.Discard
	return c.Start()
}

// Copy puts text on the clipboard.
func Copy(text string) error {
	if runtime.GOOS == "windows" {
		// clip.exe reads UTF-16LE with a BOM; plain UTF-8 garbles non-ASCII titles.
		u := utf16.Encode([]rune(text))
		buf := make([]byte, 2, 2+2*len(u))
		buf[0], buf[1] = 0xFF, 0xFE
		for _, r := range u {
			buf = append(buf, byte(r), byte(r>>8))
		}
		c := exec.Command("clip")
		c.Stdin = bytes.NewReader(buf)
		return c.Run()
	}
	tools := [][]string{{"wl-copy"}, {"xclip", "-selection", "clipboard"}, {"xsel", "--clipboard", "--input"}, {"pbcopy"}, {"termux-clipboard-set"}}
	for _, t := range tools {
		if _, err := exec.LookPath(t[0]); err == nil {
			c := exec.Command(t[0], t[1:]...)
			c.Stdin = strings.NewReader(text)
			return c.Run()
		}
	}
	return errors.New("no clipboard tool found (install wl-copy, xclip, or xsel)")
}

// ShellCommand runs a command line with the platform shell (sh or cmd.exe).
func ShellCommand(cmdline string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command(comspec(), "/C", cmdline)
	}
	return exec.Command("sh", "-c", cmdline)
}

// InteractiveShell opens the user's shell after printing banner.
func InteractiveShell(banner string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command(comspec(), "/K", "cls & echo "+banner+" & echo.")
	}
	sh := os.Getenv("SHELL")
	if sh == "" {
		sh = "bash"
	}
	c := exec.Command("sh", "-c", `clear; printf '%s\n\n' "$REELTERM_BANNER"; exec "$REELTERM_SHELL"`)
	c.Env = append(os.Environ(), "REELTERM_BANNER="+banner, "REELTERM_SHELL="+sh)
	return c
}

func comspec() string {
	if c := os.Getenv("COMSPEC"); c != "" {
		return c
	}
	return "cmd.exe"
}

// vlcBin finds vlc, including its default install folder on Windows.
func vlcBin() string {
	if _, err := exec.LookPath("vlc"); err == nil || runtime.GOOS != "windows" {
		return "vlc"
	}
	for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)"} {
		p := filepath.Join(os.Getenv(env), "VideoLAN", "VLC", "vlc.exe")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "vlc"
}

// Notify sends a desktop notification when a notifier is available.
func Notify(title, body string) {
	if _, err := exec.LookPath("notify-send"); err == nil {
		_ = exec.Command("notify-send", "-a", app.Name(), title, body).Start()
	} else if _, err := exec.LookPath("termux-notification"); err == nil {
		_ = exec.Command("termux-notification", "-t", title, "-c", body).Start()
	}
}

// EditorCmd returns a command opening file in the preferred editor.
func EditorCmd(editor, file string) (*exec.Cmd, error) {
	fallbacks := []string{"xdg-open", "open"}
	if runtime.GOOS == "windows" {
		fallbacks = []string{"notepad"}
	}
	for _, e := range append([]string{editor, os.Getenv("EDITOR")}, fallbacks...) {
		if e == "" {
			continue
		}
		fields := strings.Fields(e)
		if _, err := exec.LookPath(fields[0]); err == nil {
			return exec.Command(fields[0], append(fields[1:], file)...), nil
		}
	}
	return nil, errors.New("could not find an editor; set EDITOR in the config")
}
