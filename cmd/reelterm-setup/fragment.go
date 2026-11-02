package main

import "encoding/json"

// nerdFontFace is the font family DEVCOM.JetBrainsMonoNerdFont installs.
const nerdFontFace = "JetBrainsMono Nerd Font"

// fragment returns the Windows Terminal profile fragment for exe. Fragments
// add a profile without touching the user's settings.json.
func fragment(exe string, nerdFont bool) []byte {
	profile := map[string]any{
		"name":              "reelterm",
		"commandline":       `"` + exe + `"`,
		"icon":              exe,
		"startingDirectory": "%USERPROFILE%",
	}
	if nerdFont {
		profile["font"] = map[string]string{"face": nerdFontFace}
	}
	b, _ := json.MarshalIndent(map[string]any{"profiles": []any{profile}}, "", "  ")
	return b
}
