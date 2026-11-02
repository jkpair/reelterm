// Command reelterm-setup installs reelterm on Windows for the current user:
// the reelterm.exe embedded in it, its dependencies through winget, a
// Windows Terminal profile, shortcuts and an entry in Settings → Apps.
// It needs no admin rights (winres/winres.json asks for asInvoker).
package main

//go:generate go run github.com/tc-hib/go-winres@v0.3.3 make --in winres/winres.json --out rsrc

import (
	"bufio"
	"embed"
	"errors"
	"fmt"
	"os"
	"strings"
)

// version is set at build time with -ldflags "-X main.version=x.y.z".
var version = "dev"

// payload holds reelterm.exe, built into payload/ by a GoReleaser hook.
//
//go:embed all:payload
var payload embed.FS

func reeltermExe() ([]byte, error) {
	b, err := payload.ReadFile("payload/reelterm.exe")
	if err != nil {
		return nil, errors.New("this setup was built without reelterm.exe in cmd/reelterm-setup/payload")
	}
	return b, nil
}

type options struct {
	silent    bool
	deps      bool
	player    string // mpv, vlc, none, or the path of the user's own player
	desktop   bool
	font      bool
	uninstall bool
	purge     bool
}

const usage = `Usage: reelterm-setup [options]

  /S, --silent        install without asking anything
  --no-deps           don't install dependencies with winget
  --player <p>        mpv (default), vlc, none, or the path of a player .exe
  --no-desktop        don't create a desktop shortcut
  --no-font           don't install JetBrainsMono Nerd Font
  --uninstall         remove reelterm (keeps settings and dependencies)
  --purge             with --uninstall, also remove settings and cache
`

func parseArgs(args []string) (options, error) {
	o := options{deps: true, player: "mpv", desktop: true, font: true}
	for i := 0; i < len(args); i++ {
		switch a := args[i]; strings.ToLower(a) {
		case "/s", "-s", "--silent":
			o.silent = true
		case "--no-deps":
			o.deps = false
		case "--player":
			if i+1 >= len(args) || args[i+1] == "" {
				return o, errors.New("--player requires a value")
			}
			i++
			o.player = args[i]
		case "--no-desktop":
			o.desktop = false
		case "--no-font":
			o.font = false
		case "--uninstall":
			o.uninstall = true
		case "--purge":
			o.purge = true
		case "-h", "--help", "/?":
			fmt.Print(usage)
			os.Exit(0)
		default:
			return o, fmt.Errorf("unknown argument %q\n\n%s", a, usage)
		}
	}
	return o, nil
}

var stdin = bufio.NewReader(os.Stdin)

func readLine(prompt string) string {
	fmt.Print(prompt)
	s, _ := stdin.ReadString('\n')
	return strings.TrimSpace(s)
}

// yes asks a [Y/n] question; Enter means yes.
func yes(prompt string) bool {
	a := strings.ToLower(readLine(prompt + " [Y/n]: "))
	return a == "" || a == "y" || a == "yes"
}

// playerChoice maps an answer to the player prompt to mpv, vlc or "" (own
// player); ok is false for anything else.
func playerChoice(answer string) (player string, ok bool) {
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "", "1", "mpv":
		return "mpv", true
	case "2", "vlc":
		return "vlc", true
	case "3":
		return "", true
	}
	return "", false
}

// ask fills o from prompts.
func ask(o *options) {
	if o.deps {
		o.deps = yes("Install dependencies (yt-dlp, ffmpeg, a video player, Windows Terminal) with winget?")
	}
	for {
		p, ok := playerChoice(readLine("Video player: [1] mpv (recommended) [2] VLC [3] skip, I have my own: "))
		if !ok {
			continue
		}
		if p == "" {
			p = strings.Trim(readLine("Path to your player's .exe (Enter to keep the current setting): "), `"`)
			if p == "" {
				p = "none"
			}
		}
		o.player = p
		break
	}
	o.desktop = yes("Create a desktop shortcut?")
	if o.font {
		o.font = yes("Install JetBrainsMono Nerd Font so icons show correctly?")
	}
}

func main() {
	o, err := parseArgs(os.Args[1:])
	if err == nil {
		if o.uninstall {
			err = uninstall(o)
		} else {
			err = install(o)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "\nerror:", err)
	}
	if !o.silent {
		readLine("\nPress Enter to close.")
	}
	if err != nil {
		os.Exit(1)
	}
}
