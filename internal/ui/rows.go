package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/kevin/yt-tui/internal/thumb"
	"github.com/kevin/yt-tui/internal/yt"
)

// headerRow : titre de section non sélectionnable (« Chaînes », « Playlists », « Vidéos »).
type headerRow struct{ text string }

func (headerRow) Height() int      { return 1 }
func (headerRow) Selectable() bool { return false }
func (r headerRow) Render(w int, _ bool) []string {
	return []string{fit(" "+mutedStyle.Render(r.text), w, false)}
}

// textRow : une ligne de texte (artiste, album, playlist, chaîne…).
type textRow struct {
	label, sub string
	bold       bool
}

func (textRow) Height() int      { return 1 }
func (textRow) Selectable() bool { return true }
func (r textRow) Render(w int, sel bool) []string {
	fg := lipgloss.TerminalColor(cText)
	if sel {
		fg = cSelFg
	}
	s := spaces(1, sel) + seg(sel, fg, sel || r.bold, r.label)
	if r.sub != "" {
		sf := lipgloss.TerminalColor(cMuted)
		if sel {
			sf = cMutedSel
		}
		s += spaces(1, sel) + seg(sel, sf, false, r.sub)
	}
	return []string{fit(s, w, sel)}
}

// trackRow : une piste avec miniature (11×3), sur 3 lignes + 1 ligne d'espace.
type trackRow struct {
	t       yt.Track
	num     int // 0 = pas de numéro
	playing bool
	local   bool
	fav     bool
	thumbs  *thumb.Store
}

func (trackRow) Height() int      { return thumb.Rows + 1 }
func (trackRow) Selectable() bool { return true }

// rowCols donne la largeur des colonnes « # » et « Durée » selon la place disponible
// (étroit : ni numéro ni colonne de durée, la durée passe derrière la chaîne).
func rowCols(w int, numbered bool) (num, dur int) {
	if w < 56 {
		return 2, 0
	}
	if !numbered {
		return 2, durW
	}
	return numW, durW
}

func (r trackRow) Render(w int, sel bool) []string {
	art, ok := r.thumbs.Lines(r.t.ID)
	if !ok {
		art = thumb.Placeholder("♪")
	}
	numW, durW := rowCols(w, r.num > 0)
	textW := max(1, w-1-numW-thumb.Cols-2-durW)

	titleFg, mutedFg := lipgloss.TerminalColor(cText), lipgloss.TerminalColor(cMuted)
	if sel {
		titleFg, mutedFg = cSelFg, cMutedSel
	} else if r.playing {
		titleFg = cPink
	}
	num := spaces(numW, sel)
	if r.playing {
		label := "▶"
		if r.num > 0 {
			label = "▶ " + itoa(r.num)
		}
		num = fit(seg(sel, cPink, true, label), numW, sel)
	} else if r.num > 0 {
		num = fit(seg(sel, mutedFg, false, itoa(r.num)), numW, sel)
	}
	var meta []string
	if r.fav {
		meta = append(meta, seg(sel, cPink, true, "♥"))
	}
	if r.local {
		meta = append(meta, seg(sel, cGreen, true, txt("⬇ local")))
	}
	if r.t.Album != "" && r.t.Album != "Singles" {
		meta = append(meta, seg(sel, mutedFg, false, r.t.Album))
	}
	dur := ""
	if r.t.Duration > 0 {
		dur = fmtTime(r.t.Duration.Seconds())
	}
	chanText := seg(sel, mutedFg, false, r.t.Channel)
	if durW == 0 && dur != "" { // colonne de durée absente : durée derrière la chaîne
		chanText += seg(sel, mutedFg, false, " · "+dur)
		dur = ""
	}

	line := func(i int, number, text, right string) string {
		s := spaces(1, sel) + number + art[i] + spaces(2, sel) + fit(text, textW, sel)
		if durW > 0 {
			s += fit(strings.Repeat(" ", max(0, durW-lipgloss.Width(right)))+right, durW, sel)
		}
		return s
	}
	out := []string{
		line(0, spaces(numW, sel), seg(sel, titleFg, true, r.t.Title), ""),
		line(1, num, chanText, seg(sel, mutedFg, false, dur)),
		line(2, spaces(numW, sel), strings.Join(meta, spaces(2, sel)), ""),
		strings.Repeat(" ", w),
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for ; n > 0; n /= 10 {
		i--
		b[i] = byte('0' + n%10)
	}
	return string(b[i:])
}
