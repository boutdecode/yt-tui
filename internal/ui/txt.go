package ui

import "github.com/kevin/yt-tui/internal/i18n"

// txt traduit un texte (source : français) ; voir internal/i18n.
func txt(s string, args ...any) string { return i18n.T(s, args...) }
