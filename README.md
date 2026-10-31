# reelterm

Browse, stream and download YouTube from your terminal, plus other sites
yt-dlp supports.

A single static binary with a built-in interface. reelterm started as a fork
of [yt-x](https://github.com/Benexl/yt-x) and was rewritten in Go; it was
called yt-x-plus until v1.1.

[demo.webm](https://github.com/user-attachments/assets/027057fa-7c11-4361-a9b9-6a8d3bcd9776)

## Installation

### Any Linux distro or macOS (one line)

```bash
curl -fsSL https://github.com/jkpair/reelterm/releases/latest/download/install.sh | sh
```

This installs a static binary to `/usr/local/bin` (it asks for sudo if
needed) and then checks your dependencies. Install for your user only with
`| sh -s -- --user` (to `~/.local/bin`).

### Packages

Each [release](https://github.com/jkpair/reelterm/releases/latest) has
packages for the common package managers. They include shell completions, a
desktop entry and the example plugin:

```bash
# Debian / Ubuntu / Mint / Pop!_OS
sudo apt install ./reelterm_*_linux_amd64.deb

# Fedora / openSUSE / RHEL
sudo dnf install ./reelterm_*_linux_amd64.rpm

# Arch / Manjaro / EndeavourOS / CachyOS
sudo pacman -U ./reelterm_*_linux_amd64.pkg.tar.zst

# Alpine
sudo apk add --allow-untrusted ./reelterm_*_linux_amd64.apk
```

Builds are available for `amd64`, `arm64` and `armv7` (e.g. Raspberry Pi).
Plain binaries (`reelterm-linux-amd64`, …) and tarballs are attached too.

### Windows (experimental)

> [!WARNING]
> The Windows build is **experimental**. It's built and its tests pass, but
> it hasn't been tried on a real Windows machine yet. Bug reports are welcome.

Download `reelterm-windows-amd64.exe` from the
[latest release](https://github.com/jkpair/reelterm/releases/latest), rename
it to `reelterm.exe` and put it somewhere on your `PATH`. Use Windows Terminal with
a Nerd Font.

`reelterm --install` installs yt-dlp and ffmpeg with winget. Install
[mpv](https://mpv.io/installation/) (or VLC) yourself. Config and cache live
in `%APPDATA%\reelterm` and `%LOCALAPPDATA%\reelterm`.

Thumbnails default to colour blocks on Windows. On Windows Terminal 1.22 or
newer you can try real images with `IMAGE_RENDERER: sixel`, though they may
be sized incorrectly.

### Nix

```bash
nix run github:jkpair/reelterm
nix profile install github:jkpair/reelterm
```

### From source

```bash
git clone https://github.com/jkpair/reelterm.git
cd reelterm
go build -o bin/reelterm ./cmd/reelterm
```

Requires Go 1.26+. The `yt-x` file at the repository root is the migration
script for old Bash installs (see below), not the program.

### Updating

```bash
reelterm --update
```

This replaces the binary with the latest release. Installs from distro
packages or Nix should update through the package manager instead.

#### Coming from yt-x / yt-x-plus

`yt-x --update` (or "Check for Updates" in the Bash version) installs
reelterm in place of the old binary, keeping the `yt-x` file name so the old
command still works. On first start reelterm moves `~/.config/yt-x` and
`~/.cache/yt-x` to `~/.config/reelterm` and `~/.cache/reelterm`, and renames
`yt-x.conf` to `reelterm.conf`. Your config, themes, saved videos, recent
list, subscriptions, custom playlists, custom commands and search history
carry over; an existing `DOWNLOAD_DIRECTORY` (e.g. `~/Videos/yt-x`) is kept.

From the Bash version, two things are gone: rofi support, and sourced Bash
extensions (replaced by [Lua plugins](#plugins)).

## Dependencies

Required:

- [yt-dlp](https://github.com/yt-dlp/yt-dlp): fetching YouTube data and streams
- [mpv](https://mpv.io/) (or vlc): playback
- [ffmpeg](https://www.ffmpeg.org/): merging and converting downloads
- a [Nerd Font](https://www.nerdfonts.com/) for the icons

Optional: `chafa` (alternative thumbnail renderer), `notify-send`
(notifications), and `wl-copy`, `xclip` or `xsel` (Copy Link).

`reelterm --check-deps` shows what is installed and which image protocol your
terminal gets. `reelterm --install` installs anything missing with your package
manager (pacman, apt, dnf, zypper, xbps, apk, emerge, brew, Termux, or winget
on Windows).

## Features

- **Your YouTube:** feed, subscriptions feed, playlists, watch later, liked
  videos, watch history and clips (using your browser's cookies).
- **Search** with [filters](#search-filters) such as `:week`, `:4k` and
  `:newest`.
- **Channels:** videos, shorts, streams, podcasts, playlists, featured and
  in-channel search. Keep a local channel list or import your subscriptions.
- **Channel Feed:** the latest uploads from your local channels, merged and
  cached, without signing in (RSS, with yt-dlp as a fallback).
- **Thumbnails** in a preview pane, drawn with the best protocol your terminal
  supports:
  - Kitty graphics: kitty, Ghostty
  - Sixel: foot, WezTerm, Konsole, Windows Terminal
  - iTerm2 inline images
  - truecolor half-blocks everywhere else (including tmux)

  They load in the background, visible rows first, and appear as soon as they
  arrive.
- **Playback** in mpv or vlc. Watch, listen (audio only), play or listen to a
  whole list, and YouTube mixes. Resumes where you stopped.
- **Background downloads** with a progress queue: single videos, whole lists,
  channels, search results or any URL, as video or mp3. SponsorBlock,
  embedded subtitles, chapters and thumbnails, and codec/HDR preferences are
  set in the config.
- **Local library:** browse and play your downloads, and build local mixes
  from them.
- **Library:** saved videos, recently played, custom playlists, search
  history, and custom yt-dlp commands that load any playlist-like JSON.
- **Multi-select:** `tab` marks videos, `enter` plays, downloads or saves them
  together.
- **Lua plugins** for your own menus, video actions and hooks.
- **21 themes**, switchable live from Settings.
- Shell completions, a desktop entry, and self-update.

### Keys

| Key | Action |
| --- | --- |
| type | filter the current list |
| `↑` `↓` / `ctrl+p` `ctrl+n` | move |
| `enter` / `→` | select |
| `esc` | clear the filter, or go back |
| `tab` | mark a video (multi-select) |
| `shift+←` `shift+→` / `pgup` `pgdn` | page up / down |
| `ctrl+d` | download |
| `ctrl+s` | save |
| `ctrl+o` | open in browser |
| `ctrl+y` | copy link |
| `ctrl+r` | reload |
| `ctrl+c` | quit |

## Usage

```bash
reelterm                          # launch
reelterm -S ":week linux news"    # search directly
reelterm -e                       # edit the config
reelterm -p vlc                   # use vlc for this run
reelterm --theme nord             # pick a theme for this run (--list-themes to see all)
reelterm --no-preview             # hide the preview pane
reelterm -x example               # load a Lua plugin
reelterm completions --fish       # print shell completions (--bash, --zsh)
reelterm --help
```

## Configuration

The config lives at `~/.config/reelterm/reelterm.conf` (`reelterm -e` or Settings → Edit
Config). Every option is documented in the file. Highlights:

```ini
THEME: everforest-dark-hard
ENABLE_PREVIEW: true
IMAGE_RENDERER: auto        # auto, kitty, sixel, iterm, halfblock or chafa
PLAYER: mpv
PREFERRED_BROWSER: firefox  # browser to read YouTube cookies from
COOKIE_FILE:                # cookies.txt fallback
DOWNLOAD_DIRECTORY: ~/Videos/reelterm
SPONSORBLOCK: false
RESUME_PLAYBACK: true
```

Themes: see [themes/README.md](themes/README.md). Put your own `*.theme` files
in `~/.config/reelterm/themes/`.

### Enabling subscriptions and private playlists

Set your browser in the config, or pick it from Settings → Select Browser
(reelterm tests that cookie extraction works):

```ini
PREFERRED_BROWSER: firefox
```

If yt-dlp reports "cannot decrypt cookies", include the profile and keyring,
e.g. `PREFERRED_BROWSER: brave+gnomekeyring:Default`. Check a value with:

```bash
yt-dlp --cookies-from-browser "brave+gnomekeyring:Default" --skip-download --flat-playlist "https://www.youtube.com/feed/channels"
```

> [!TIP]
> Setting up a PO token makes YouTube much less likely to block yt-dlp. See the
> [PO Token Guide](https://github.com/yt-dlp/yt-dlp/wiki/PO-Token-Guide) and
> [bgutil-ytdlp-pot-provider](https://github.com/Brainicism/bgutil-ytdlp-pot-provider).

### mpv and yt-dlp tips

reelterm passes your cookies and its yt-dlp options (SponsorBlock etc.) to mpv
automatically. You can still tune mpv in `mpv.conf` (Settings → Edit MPV
Config):

```ini
ytdl-format="bestvideo[height<=1080]+bestaudio/best"
hwdec=auto
slang=en,eng
```

Download defaults can go in yt-dlp's own config (Settings → Edit yt-dlp
Config), for example `--embed-metadata` or `--merge-output-format mkv`.

For a nicer player, try [uosc](https://github.com/tomasklaen/uosc) and
[thumbfast](https://github.com/po5/thumbfast).

### Custom playlists

Save any playlist from its action menu (Save Playlist), or edit
`~/.config/reelterm/custom_playlists.json`:

```json
[
  {
    "name": "<playlist name>",
    "playlistUrl": "https://www.youtube.com/playlist?list=<playlist-id>",
    "playlistWatchUrl": "https://www.youtube.com/watch?list=<playlist-id>"
  }
]
```

## Plugins

Plugins are Lua files in `~/.config/reelterm/plugins/`. Load one for a run with
`reelterm -x <name>`, or on every start with `AUTO_LOADED_EXTENSIONS: name1,name2`
in the config. A plugin can add main-menu entries and video actions, hook
`play` / `download` / `search`, run searches and open the results in the
normal list (with previews), and play, download or open URLs.

```lua
yt.video_action("Short videos from this channel", function(video)
  local videos = yt.ytdlp(video.channel_url .. "/videos", 60) or {}
  local short = {}
  for _, v in ipairs(videos) do
    if v.duration_seconds and v.duration_seconds < 600 then table.insert(short, v) end
  end
  yt.show_list(video.channel .. " · under 10 min", short)
end)
```

The full API is documented in [plugins/example.lua](plugins/example.lua).

## Search filters

Start a search with one `:filter`: `:filter your query`.

| Filter | Description |
| --- | --- |
| `:hour` `:today` `:week` `:month` `:year` | upload date |
| `:video` `:playlist` `:movie` `:live` | result type |
| `:short` `:long` | under / over 4 minutes |
| `:hd` `:4k` `:hdr` `:360` `:vr` `:3d` | quality and format |
| `:subtitles` | has subtitles/CC |
| `:local` | filtered by location |
| `:views` `:rating` `:newest` | sort order |

## Development

```bash
nix develop               # or install Go 1.26+
go test -race ./...
go build -o bin/reelterm ./cmd/reelterm
```

Releases are built by GitHub Actions with [GoReleaser](https://goreleaser.com)
when a `v*` tag is pushed (`.goreleaser.yaml`).

## Credits

reelterm began as a fork of [yt-x](https://github.com/Benexl/yt-x) by
[Benexl](https://github.com/Benexl), whose design, config format and themes
it builds on. If you enjoy it, consider
[supporting Benexl](https://buymeacoffee.com/benexl).

## Support

Found a bug or have an idea? Open an
[issue](https://github.com/jkpair/reelterm/issues).
