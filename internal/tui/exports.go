package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jkpair/reelterm/internal/app"
	"github.com/jkpair/reelterm/internal/player"
)

// Push returns a command that opens s on top of the stack.
func Push(s Screen) tea.Cmd { return push(s) }

// Toast returns a command that shows a status message.
func Toast(s string) tea.Cmd { return toast(s) }

// SearchList opens search results for q (records history).
func SearchList(env *Env, q string) Screen { return searchList(env, q) }

// NewWelcome is the monthly support screen.
func NewWelcome(env *Env) *Menu {
	return NewMenu(env, "Welcome back!", func() []Item {
		return []Item{
			{Icon: "", IconColor: env.Theme.Red, Label: "Open support page", Value: "open", Desc: "reelterm is based on yt-x by Benexl. If you enjoy it, consider supporting the original author: " + app.SupportProjectURL + "\nDisable in config: WELCOME_SCREEN: false"},
			{Icon: env.Theme.Icon("play"), IconColor: env.Theme.Green, Label: "Continue", Value: "", Desc: "Disable in config: WELCOME_SCREEN: false"},
		}
	}, func(it Item) tea.Cmd {
		if it.Value == "open" {
			_ = player.OpenURL(app.SupportProjectURL)
		}
		return pop()
	})
}
