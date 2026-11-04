package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Palette de la maquette (variantes claires pour les terminaux à fond clair).
var (
	cPink      = lipgloss.AdaptiveColor{Light: "#d6336c", Dark: "#ff6fa5"}
	cSel       = lipgloss.AdaptiveColor{Light: "#d9d2ff", Dark: "#4b36a8"}
	cSelFg     = lipgloss.AdaptiveColor{Light: "#1b1340", Dark: "#ffffff"}
	cBorderOn  = lipgloss.AdaptiveColor{Light: "#6d5bd0", Dark: "#6d5bd0"}
	cBorderOff = lipgloss.AdaptiveColor{Light: "#c9c5dd", Dark: "#2c2748"}
	cText      = lipgloss.AdaptiveColor{Light: "#1b1340", Dark: "#e8e4f7"}
	cMuted     = lipgloss.AdaptiveColor{Light: "#6b6690", Dark: "#a39cc4"}
	cMutedSel  = lipgloss.AdaptiveColor{Light: "#3b2f8f", Dark: "#d4cff0"}
	cKey       = lipgloss.AdaptiveColor{Light: "#3b2f8f", Dark: "#d4cff0"}
	cBg        = lipgloss.AdaptiveColor{Light: "#f6f4ff", Dark: "#14121f"} // fond de l'application
	cGreen     = lipgloss.AdaptiveColor{Light: "#059669", Dark: "#34d399"}
	cRed       = lipgloss.AdaptiveColor{Light: "#dc2626", Dark: "#f87171"}
)

var (
	logoStyle, tabActive, tabInactive, pinkBold, pinkText        lipgloss.Style
	mutedStyle, textStyle, boldText, keyStyle, okStyle, errStyle lipgloss.Style
	chipActive                                                   lipgloss.Style
)

func init() { buildStyles() }

// buildStyles (re)construit les styles dérivés de la palette courante.
func buildStyles() {
	logoStyle = lipgloss.NewStyle().Bold(true).Foreground(cBg).Background(cPink).Padding(0, 1)
	tabActive = lipgloss.NewStyle().Bold(true).Foreground(cSelFg).Background(cSel).Padding(0, 1)
	tabInactive = lipgloss.NewStyle().Foreground(cMuted).Padding(0, 1)
	pinkBold = lipgloss.NewStyle().Bold(true).Foreground(cPink)
	pinkText = lipgloss.NewStyle().Foreground(cPink)
	mutedStyle = lipgloss.NewStyle().Foreground(cMuted)
	textStyle = lipgloss.NewStyle().Foreground(cText)
	boldText = lipgloss.NewStyle().Foreground(cText).Bold(true)
	keyStyle = lipgloss.NewStyle().Bold(true).Foreground(cKey)
	okStyle = lipgloss.NewStyle().Foreground(cGreen)
	errStyle = lipgloss.NewStyle().Foreground(cRed)
	chipActive = lipgloss.NewStyle().Foreground(cSelFg).Background(cSel).Padding(0, 1)
}

// trunc coupe s à w colonnes (séquences ANSI et caractères larges gérés).
func trunc(s string, w int) string {
	if w < 1 {
		return ""
	}
	return ansi.Truncate(s, w, "…")
}

// seg stylise un morceau de texte ; en ligne sélectionnée le fond violet est appliqué
// à chaque segment (un reset ANSI interrompt sinon le fond).
func seg(sel bool, fg lipgloss.TerminalColor, bold bool, s string) string {
	st := lipgloss.NewStyle().Foreground(fg).Bold(bold)
	if sel {
		st = st.Background(cSel)
	}
	return st.Render(s)
}

// pickFg choisit la couleur de texte selon que la ligne est sélectionnée ou non.
func pickFg(sel bool, normal, onSel lipgloss.TerminalColor) lipgloss.TerminalColor {
	if sel {
		return onSel
	}
	return normal
}

func spaces(n int, sel bool) string {
	if n <= 0 {
		return ""
	}
	if sel {
		return lipgloss.NewStyle().Background(cSel).Render(strings.Repeat(" ", n))
	}
	return strings.Repeat(" ", n)
}

// fit tronque ou complète s pour qu'il occupe exactement w colonnes.
func fit(s string, w int, sel bool) string {
	s = trunc(s, w)
	return s + spaces(w-lipgloss.Width(s), sel)
}

// fmtTime formate des secondes en mm:ss (ou h:mm:ss au-delà d'une heure).
func fmtTime(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	s := int(sec)
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, s%3600/60, s%60)
	}
	return fmt.Sprintf("%02d:%02d", s/60, s%60)
}

type hint struct{ key, desc string }

// renderHints affiche « touche libellé » séparés par deux espaces, tronqués à w.
func renderHints(hs []hint, w int) string {
	parts := make([]string, len(hs))
	for i, h := range hs {
		parts[i] = keyStyle.Render(h.key) + " " + mutedStyle.Render(h.desc)
	}
	return trunc(strings.Join(parts, "  "), w)
}

// panel encadre body (déjà à la bonne taille) dans un cadre arrondi de w×h, de couleur c.
func panel(body string, w, h int, c lipgloss.TerminalColor) string {
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(c).
		Width(w - 2).Height(h - 2).MaxWidth(w).MaxHeight(h).Render(body)
}

// titleLine : titre rose à gauche, complément atténué à droite, sur w colonnes.
func titleLine(title, extra string, w int) string {
	left := " " + pinkBold.Render(title)
	right := ""
	if extra != "" {
		right = mutedStyle.Render(extra) + " "
	}
	gap := max(1, w-lipgloss.Width(left)-lipgloss.Width(right))
	return trunc(left+strings.Repeat(" ", gap)+right, w)
}

// joinH colle deux blocs côte à côte avec un espace entre eux.
func joinH(a, b string) string { return lipgloss.JoinHorizontal(lipgloss.Top, a, " ", b) }

// bgCodes renvoie la séquence ANSI qui active le fond de l'application (selon le profil
// de couleurs du terminal ; vide si le terminal n'a pas de couleurs) et celle de reset.
func bgCodes() (on, reset string) {
	const probe = "\x00"
	s := lipgloss.NewStyle().Background(cBg).Render(probe)
	i := strings.Index(s, probe)
	if i <= 0 {
		return "", ""
	}
	return s[:i], s[i+len(probe):]
}

// paintBackground applique le fond sur chaque cellule de l'écran w×h : chaque ligne
// commence par le fond, le refait après chaque reset ANSI (sinon le premier segment
// stylisé l'effacerait jusqu'à la fin de la ligne) et est complétée jusqu'à w colonnes.
func paintBackground(screen string, w, h int, on string) string {
	if on == "" {
		return screen
	}
	lines := strings.Split(screen, "\n")
	for len(lines) < h {
		lines = append(lines, "")
	}
	for i, l := range lines {
		l = strings.ReplaceAll(l, "\x1b[0m", "\x1b[0m"+on)
		l = strings.ReplaceAll(l, "\x1b[m", "\x1b[m"+on)
		if pad := w - lipgloss.Width(l); pad > 0 {
			l += strings.Repeat(" ", pad)
		}
		lines[i] = on + l + "\x1b[0m"
	}
	return strings.Join(lines, "\n")
}

// borderFor renvoie la couleur de bordure d'un panneau focalisé ou non.
func borderFor(focus bool) lipgloss.TerminalColor {
	if focus {
		return cBorderOn
	}
	return cBorderOff
}
