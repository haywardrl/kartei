// Package ui embeds the room so the binary is self-contained.
package ui

import "embed"

// FS holds index.html and its scripts.
//
//go:embed index.html style.css *.js
var FS embed.FS
