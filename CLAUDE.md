# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

reelterm is a terminal-based YouTube browser and downloader written in Go (`cmd/reelterm`, `internal/`, Bubble Tea UI). It began as a fork of Benexl/yt-x, was a single Bash script until v1.0.0, and was named yt-x-plus (binary `yt-x`) until the rename. yt-dlp, mpv/vlc and ffmpeg are external processes.

The root `yt-x` file is **not** the program: it is a small Bash migration script that old Bash installs download through their self-updater (from `raw.githubusercontent.com/jkpair/yt-x-plus/.../master/yt-x`, which GitHub redirects to this repo), which swaps itself for the release binary. Keep its file name and its `CLI_VERSION="x.y.z"` line (the old updater compares it).

**Rename compatibility:** `app.NewPaths` moves `~/.config/yt-x` and `~/.cache/yt-x` (and `yt-x.conf`) to the reelterm names on first start. Releases also publish `yt-x-<os>-<arch>` copies of the binaries because `--update` in 1.0.x looks for those asset names.

## Build, test, release

```bash
go build -o bin/reelterm ./cmd/reelterm   # build (bin/ is gitignored)
go test -race ./...                # tests
go vet ./...
```

Packages under `internal/`:

- `app`: version (set via `-ldflags -X .../internal/app.Version=`), XDG paths
- `config`: `reelterm.conf` parsing/writing (same `KEY: value` format as Bash); `Set()` edits in place
- `theme`: parses the shell-variable `themes/*.theme` files (embedded via `themes/embed.go`) into lipgloss colors; hex comments beside 256-color codes are preferred
- `ytdlp`: yt-dlp wrapper (`Fetch` = run_yt_dlp with cookie-file fallback, `BuildOpts` = build_ytdlp_opts), `Entry`/`Playlist` JSON models and formatting
- `youtube`: URL builders, search `:filter` → `sp=` mapping, oEmbed
- `cache`: on-disk store (feed cache, metadata, recent/saved/subscriptions JSON, search history, custom playlists/commands, m3u); file names match the Bash version
- `thumb`: thumbnail download pool, protocol detection, renderers (kitty/sixel/iTerm2/half-block/chafa) and overlay placement
- `player`: mpv/vlc/Android command building, stderr classification, clipboard/open/notify helpers
- `download`: background yt-dlp jobs with progress parsed from `--progress-template`
- `plugin`: gopher-lua (Lua 5.1) runtime exposing the `yt` table; see `plugins/example.lua`
- `selfupdate`: GitHub release check and binary replacement (assets named `reelterm-<os>-<arch>`)
- `deps`: dependency report/installer, `--install-self`
- `tui`: screens on a stack in `Root` (`push`/`pop`/`replace` messages). `Menu` is the generic fzf-style list, `VideoList` the browsing screen with the preview pane

**Thumbnail overlays (important):** Kitty/Sixel/iTerm2 images are written straight to stdout by `thumb.Manager.HandleDraw`, outside Bubble Tea's renderer. The preview pane reserves the image cells with `ansi.CursorForward` instead of spaces, so the renderer's line rewrites skip them. That's why `Root.View` splits and pads lines by hand (never wrap such lines in lipgloss, which would pad them), and why list rows leave the last column free. Draws are debounced 40ms via `Manager.Send` so queued frames flush first. Half-block/chafa render as ordinary text.

**Windows (experimental):** platform differences live behind `runtime.GOOS` checks in `player` (open via rundll32, clipboard via `clip` fed UTF-16LE, `cmd.exe` shells, Notepad fallback, VLC in Program Files), `app` (`%APPDATA%`/`%LOCALAPPDATA%` unless `~/.config/reelterm` or `~/.config/yt-x` exists), `selfupdate` (rename running .exe to `.old`), `deps` (winget) and `thumb` (auto → half-blocks). Run the suite as Windows binaries with `GOOS=windows go test -exec wine ./...`; `*_windows_test.go` files cover these paths.

**Nerd Font icons:** write private-use codepoints as `\uXXXX`/`\UXXXXXXXX` escapes in Go (and `\ddd` byte escapes in Lua 5.1). Literal glyphs get lost by some editing tools.

**Releases:** bump `Version` in `internal/app/app.go`, `version` in `default.nix` and `CLI_VERSION` in the root `yt-x` script, then push a `v*` tag to the `fork` remote (jkpair/reelterm). `.github/workflows/release.yml` runs GoReleaser (`.goreleaser.yaml`): static binaries `reelterm-<os>-<arch>` (plus legacy `yt-x-<os>-<arch>` copies) (used by `--update`, `install.sh` and the migration script), tarballs, deb/rpm/apk/Arch packages, `install.sh` and checksums. Test locally with `goreleaser release --snapshot --clean`. Nix: `default.nix` (`vendorHash` must be updated after dependency changes).

## Search Filters

Append to search queries: `:hour`, `:today`, `:week`, `:month`, `:year` (time), `:video`, `:movie`, `:live`, `:short`, `:long` (type), `:4k`, `:hd` (quality), `:views`, `:rating`, `:newest` (sort)

## Commit Convention

Uses conventional commits: `feat:`, `fix:`, `chore:`, `docs:`, `refactor:`
Changelog generated via git-cliff (`cliff.toml`)
