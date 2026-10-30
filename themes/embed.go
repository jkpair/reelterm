// Package themes embeds the built-in *.theme files.
package themes

import "embed"

//go:embed *.theme
var FS embed.FS
