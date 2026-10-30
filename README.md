# yt-x-plus

Browse, stream and download YouTube from your terminal, plus other sites
yt-dlp supports.

A fork of [yt-x](https://github.com/Benexl/yt-x), rewritten in Go as a single
static binary with a built-in interface.

[yt-x-plus-demo.webm](https://github.com/user-attachments/assets/027057fa-7c11-4361-a9b9-6a8d3bcd9776)

## Installation

### Any Linux distro or macOS (one line)

```bash
curl -fsSL https://github.com/jkpair/yt-x-plus/releases/latest/download/install.sh | sh
```

This installs a static binary to `/usr/local/bin` (it asks for sudo if
needed) and then checks your dependencies. Install for your user only with
`| sh -s -- --user` (to `~/.local/bin`).

### Packages

Each [release](https://github.com/jkpair/yt-x-plus/releases/latest) has
packages for the common package managers. They include shell completions, a
desktop entry and the example plugin:

```bash
# Debian / Ubuntu / Mint / Pop!_OS
sudo apt install ./yt-x_*_linux_amd64.deb

# Fedora / openSUSE / RHEL
sudo dnf install ./yt-x_*_linux_amd64.rpm

# Arch / Manjaro / EndeavourOS / CachyOS
sudo pacman -U ./yt-x_*_linux_amd64.pkg.tar.zst

# Alpine
sudo apk add --allow-untrusted ./yt-x_*_linux_amd64.apk
```

Builds are available for `amd64`, `arm64` and `armv7` (e.g. Raspberry Pi).
Plain binaries (`yt-x-linux-amd64`, …) and tarballs are attached too.

### Nix

```bash
nix run github:jkpair/yt-x-plus
nix profile install github:jkpair/yt-x-plus
```

### From source

```bash
git clone https://github.com/jkpair/yt-x-plus.git
cd yt-x-plus
go build -o bin/yt-x ./cmd/yt-x
```

Requires Go 1.26+. Build into `bin/`: the `yt-x` file at the repository root
is the migration script for old Bash installs (see below).

### Updating

```bash
yt-x --update
```

This replaces the binary with the latest release. Installs from distro
packages or Nix should update through the package manager instead.

#### Coming from the Bash version

Run "Check for Updates" (or `yt-x -U`) once. The old updater downloads a small
migration script, which swaps itself for the Go binary. Your config, themes,
saved videos, recent list, subscriptions, custom playlists, custom commands
and search history are read as they are. Two things are gone: rofi support,
and sourced Bash extensions (replaced by [Lua plugins](#plugins)).

## Dependencies

Required:

- [yt-dlp](https://github.com/yt-dlp/yt-dlp): fetching YouTube data and streams
- [mpv](https://mpv.io/) (or vlc): playback
- [ffmpeg](https://www.ffmpeg.org/): merging and converting downloads
- a [Nerd Font](https://www.nerdfonts.com/) for the icons

Optional: `chafa` (alternative thumbnail renderer), `notify-send`
(notifications), and `wl-copy`, `xclip` or `xsel` (Copy Link).

`yt-x --check-deps` shows what is installed and which image protocol your
terminal gets. `yt-x --install` installs anything missing with your package
manager (pacman, apt, dnf, zypper, xbps, apk, emerge, brew or Termux).

## Features

- **Your YouTube:** feed, subscriptions feed, playlists, watch later, liked
  videos, watch history and clips (using your browser's cookies).
- **Search** with [filters](#search-filters) such as `:week`, `:4k` and
  `:newest`.
- **Channels:** videos, shorts, streams, podcasts, playlists, featured and
  in-channel search. Keep a local channel list or import your subscriptions.
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
yt-x                          # launch
yt-x -S ":week linux news"    # search directly
yt-x -e                       # edit the config
yt-x -p vlc                   # use vlc for this run
yt-x --theme nord             # pick a theme for this run (--list-themes to see all)
yt-x --no-preview             # hide the preview pane
yt-x -x example               # load a Lua plugin
yt-x completions --fish       # print shell completions (--bash, --zsh)
yt-x --help
```

## Configuration

The config lives at `~/.config/yt-x/yt-x.conf` (`yt-x -e` or Settings → Edit
Config). Every option is documented in the file. Highlights:

```ini
THEME: everforest-dark-hard
ENABLE_PREVIEW: true
IMAGE_RENDERER: auto        # auto, kitty, sixel, iterm, halfblock or chafa
PLAYER: mpv
PREFERRED_BROWSER: firefox  # browser to read YouTube cookies from
COOKIE_FILE:                # cookies.txt fallback
DOWNLOAD_DIRECTORY: ~/Videos/yt-x
SPONSORBLOCK: false
RESUME_PLAYBACK: true
```

Themes: see [themes/README.md](themes/README.md). Put your own `*.theme` files
in `~/.config/yt-x/themes/`.

### Enabling subscriptions and private playlists

Set your browser in the config, or pick it from Settings → Select Browser
(yt-x tests that cookie extraction works):

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

yt-x passes your cookies and its yt-dlp options (SponsorBlock etc.) to mpv
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
`~/.config/yt-x/custom_playlists.json`:

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

Plugins are Lua files in `~/.config/yt-x/plugins/`. Load one for a run with
`yt-x -x <name>`, or on every start with `AUTO_LOADED_EXTENSIONS: name1,name2`
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
go build -o bin/yt-x ./cmd/yt-x
```

Releases are built by GitHub Actions with [GoReleaser](https://goreleaser.com)
when a `v*` tag is pushed (`.goreleaser.yaml`).

## Credits

Based on [yt-x](https://github.com/Benexl/yt-x) by
[Benexl](https://github.com/Benexl).

## Support

Need help with the upstream project? Join the community on Discord:

<p align="center">
<a href="https://discord.gg/HBEmAwvbHV">
<img src="https://invidget.switchblade.xyz/C4rhMA4mmK">
</a>
</p>
