// Package icons provides NerdFont icons with ASCII fallback.
// Used by both lazarusd (kernel banner) and mcp (prompts).
package icons

import (
	"os"
	"strings"
)

// NerdFont Mono icons — Unicode private use area glyphs.
// Fallback ASCII characters provided for terminals without NerdFont support.
const (
	Check  = "\U000F012C" // nf-md-check
	Close  = "\U000F0156" // nf-md-close
	Warn   = "\U000F002A" // nf-md-alert
	Gear   = "\U000F061B" // nf-md-cog
	Info   = "\U000F02FD" // nf-md-help
	Heart  = "\U000F02D1" // nf-md-heart
	Shield = "\U000F0687" // nf-md-shield
)

// UseNerd reports whether NerdFont icons should be used.
// Controlled by LAZARUS_NERDFONT env (default: true).
// Falls back to false when terminal lacks UTF-8 or NerdFont support.
func UseNerd() bool {
	if v := os.Getenv("LAZARUS_NERDFONT"); v == "0" || v == "false" {
		return false
	}
	term := os.Getenv("TERM")
	// Linux console (Ctrl+Alt+F2) uses built-in VGA font — no NerdFont support.
	// TERM=linux with LANG=UTF-8 would otherwise pass the check but render garbage.
	if term == "linux" {
		return false
	}
	lang := os.Getenv("LANG")
	return strings.Contains(term, "256color") ||
		strings.Contains(term, "alacritty") ||
		strings.Contains(term, "kitty") ||
		strings.Contains(term, "ghostty") ||
		strings.Contains(lang, "UTF-8")
}

// Icon returns the NerdFont icon if available, otherwise the ASCII fallback.
func Icon(nerd, ascii string) string {
	if UseNerd() {
		return nerd
	}
	return ascii
}
