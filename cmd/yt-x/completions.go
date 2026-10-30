package main

import (
	"fmt"
	"strings"
)

var longFlags = []string{"--help", "--version", "--edit-config", "--update", "--search", "--player", "--extension",
	"--preview", "--no-preview", "--theme", "--list-themes", "--check-deps", "--install", "--install-self",
	"--disown-streaming-process", "--no-disown-streaming-process", "--generate-desktop-entry"}

func completions(name, shell string) (string, error) {
	switch shell {
	case "-f", "--fish":
		return strings.ReplaceAll(fishCompletions, "yt-x", name), nil
	case "-b", "--bash":
		return strings.ReplaceAll(strings.ReplaceAll(bashCompletions, "@FLAGS@", "-h -v -e -U -S -p -x -E "+strings.Join(longFlags, " ")), "yt-x", name), nil
	case "-z", "--zsh":
		return strings.ReplaceAll(zshCompletions, "yt-x", name), nil
	}
	return "", fmt.Errorf("usage: %s completions --fish|--bash|--zsh", name)
}

const fishCompletions = `# yt-x completions
complete -c yt-x --no-files --arguments completions --condition 'not __fish_seen_subcommand_from completions'
complete -c yt-x --no-files -s h -l help -d 'Print a short help text and exit'
complete -c yt-x --no-files -s v -l version -d 'Print the version and exit'
complete -c yt-x --no-files -s e -l edit-config -d 'Edit the config file'
complete -c yt-x --no-files -s U -l update -d 'Update to the latest release'
complete -c yt-x --no-files -s S -l search -d 'Search for a video' -r
complete -c yt-x --no-files -s p -l player -d 'The video player to use' -x -a 'mpv vlc'
complete -c yt-x --no-files -s x -l extension -d 'Load a Lua plugin' -x -a "(command ls ${XDG_CONFIG_HOME:-$HOME/.config}/yt-x/plugins 2>/dev/null)"
complete -c yt-x --no-files -l preview -d 'Enable the preview pane'
complete -c yt-x --no-files -l no-preview -d 'Disable the preview pane'
complete -c yt-x --no-files -l theme -d 'Set the color theme' -x -a "(yt-x --list-themes 2>/dev/null | string match -r '\S+\$' | string trim -c '* ')"
complete -c yt-x --no-files -l list-themes -d 'List available themes'
complete -c yt-x --no-files -l check-deps -d 'Check dependencies'
complete -c yt-x --no-files -l install -d 'Install missing dependencies'
complete -c yt-x --force-files -l install-self -d 'Install yt-x to a directory'
complete -c yt-x --no-files -l disown-streaming-process -d 'Keep the player running after exit'
complete -c yt-x --no-files -l no-disown-streaming-process -d 'Stop the player on exit'
complete -c yt-x --no-files -s E -l generate-desktop-entry -d 'Print a desktop entry'
complete -c yt-x --no-files -s f -l fish -d 'Print fish completions' --condition '__fish_seen_subcommand_from completions'
complete -c yt-x --no-files -s b -l bash -d 'Print bash completions' --condition '__fish_seen_subcommand_from completions'
complete -c yt-x --no-files -s z -l zsh -d 'Print zsh completions' --condition '__fish_seen_subcommand_from completions'
`

const bashCompletions = `_yt_x() {
  local cur prev words cword
  _init_completion || return
  if [[ "${words[1]}" == "completions" ]]; then
    COMPREPLY=($(compgen -W "--fish --bash --zsh" -- "$cur"))
    return
  fi
  case "$prev" in
    -p|--player) COMPREPLY=($(compgen -W "mpv vlc" -- "$cur")); return ;;
    --theme) COMPREPLY=($(compgen -W "$(yt-x --list-themes 2>/dev/null | tail -n +2 | sed 's/^[ *]*//;s/ (current)//')" -- "$cur")); return ;;
    --install-self) _filedir -d; return ;;
    -x|--extension)
      local d="${XDG_CONFIG_HOME:-$HOME/.config}/yt-x/plugins"
      [[ -d "$d" ]] && COMPREPLY=($(compgen -W "$(command ls "$d" 2>/dev/null)" -- "$cur"))
      _filedir; return ;;
    -S|--search) return ;;
  esac
  if [[ "$cur" == -* ]]; then
    COMPREPLY=($(compgen -W "@FLAGS@" -- "$cur"))
  else
    COMPREPLY=($(compgen -W "completions" -- "$cur"))
  fi
}
complete -F _yt_x yt-x
`

const zshCompletions = `#compdef yt-x

_yt_x_plugins() {
  local d="${XDG_CONFIG_HOME:-$HOME/.config}/yt-x/plugins"
  [[ -d "$d" ]] && compadd -- "$d"/*(:t) 2>/dev/null
}

_yt_x_themes() {
  compadd -- ${(f)"$(yt-x --list-themes 2>/dev/null | tail -n +2 | sed 's/^[ *]*//;s/ (current)//')"}
}

_yt_x() {
  _arguments -s \
    '(-h --help)'{-h,--help}'[show help message and exit]' \
    '(-v --version)'{-v,--version}'[print version and exit]' \
    '(-e --edit-config)'{-e,--edit-config}'[edit config file]' \
    '(-U --update)'{-U,--update}'[update to the latest release]' \
    '(-S --search)'{-S,--search}'[search for a query]:query: ' \
    '(-p --player)'{-p,--player}'[set video player]:player:(mpv vlc)' \
    '(-x --extension)'{-x,--extension}'[load a Lua plugin]:plugin:_yt_x_plugins' \
    '--preview[enable preview pane]' \
    '--no-preview[disable preview pane]' \
    '--theme[set color theme]:theme:_yt_x_themes' \
    '--list-themes[list available themes]' \
    '--check-deps[check dependencies]' \
    '--install[install missing dependencies]' \
    '--install-self[install yt-x to a directory]:path:_directories' \
    '--disown-streaming-process[keep the player running after exit]' \
    '--no-disown-streaming-process[stop the player on exit]' \
    '(-E --generate-desktop-entry)'{-E,--generate-desktop-entry}'[print a desktop entry]' \
    '1:command:(completions)' \
    '*::arg:->args'
  case "$words[1]" in
    completions) _arguments '--fish[fish completions]' '--bash[bash completions]' '--zsh[zsh completions]' ;;
  esac
}

_yt_x "$@"
`
