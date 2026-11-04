package ui

import (
	"fmt"
	"image/color"
	"math"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"

	"github.com/kevin/yt-tui/internal/thumb"
)

// Theme regroupe les couleurs qui suivent la pochette (le texte reste fixe).
type Theme struct {
	Accent, Accent2     string // rose de la maquette et seconde teinte du dégradé
	Sel                 string // fond de la ligne sélectionnée / onglet actif
	BorderOn, BorderOff string
	Bg                  string // fond de l'application
	Track               string // piste vide de la barre de progression
}

// defaultTheme est la palette de la maquette.
func defaultTheme() Theme {
	return Theme{Accent: "#ff6fa5", Accent2: "#7d56f4", Sel: "#4b36a8",
		BorderOn: "#6d5bd0", BorderOff: "#2c2748", Bg: "#14121f", Track: "#2c2748"}
}

func hexOf(c color.RGBA) string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

// themeFrom dérive un thème (sombre) des deux couleurs dominantes ; les luminosités sont
// bornées pour garder texte et sélection lisibles quelle que soit la pochette.
func themeFrom(c thumb.Colors) Theme {
	h1, s1, _ := thumb.RGBToHSL(c.Primary)
	h2, s2, _ := thumb.RGBToHSL(c.Secondary)
	at := func(h, s, l float64) string { return hexOf(thumb.HSLToRGBA(h, clamp(s, 0.35, 0.9), l)) }
	t := Theme{
		Accent:    at(h1, s1, 0.68),
		Accent2:   at(h2, s2, 0.58),
		Sel:       at(h1, math.Min(s1, 0.55), 0.30),
		BorderOn:  at(h1, math.Min(s1, 0.6), 0.55),
		BorderOff: at(h1, math.Min(s1, 0.35), 0.19),
		Bg:        hexOf(thumb.HSLToRGBA(h1, clamp(s1, 0.15, 0.3), 0.075)),
	}
	t.Track = t.BorderOff
	if contrast(t.Sel, "#ffffff") < 4.5 || contrast(t.Bg, "#e8e4f7") < 7 || contrast(t.Bg, t.Accent) < 4.5 {
		return defaultTheme()
	}
	return t
}

func lum(hex string) float64 {
	var r, g, b uint8
	fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	f := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= 0.03928 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*f(r) + 0.7152*f(g) + 0.0722*f(b)
}

// contrast : rapport de contraste WCAG entre deux couleurs « #rrggbb ».
func contrast(a, b string) float64 {
	la, lb := lum(a), lum(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// applyTheme installe t comme palette courante et reconstruit les styles qui en dépendent.
// À appeler depuis la boucle Bubble Tea uniquement (Update/View partagent la goroutine).
func applyTheme(t Theme) {
	cPink = lipgloss.AdaptiveColor{Light: cPink.Light, Dark: t.Accent}
	cSel = lipgloss.AdaptiveColor{Light: cSel.Light, Dark: t.Sel}
	cBorderOn = lipgloss.AdaptiveColor{Light: cBorderOn.Light, Dark: t.BorderOn}
	cBorderOff = lipgloss.AdaptiveColor{Light: cBorderOff.Light, Dark: t.BorderOff}
	cBg = lipgloss.AdaptiveColor{Light: cBg.Light, Dark: t.Bg}
	curTheme = t
	buildStyles()
}

var curTheme = defaultTheme()

// newProgress crée la barre de progression au dégradé du thème courant.
func newProgress() progress.Model {
	pg := progress.New(progress.WithGradient(curTheme.Accent2, curTheme.Accent), progress.WithoutPercentage())
	pg.Full, pg.Empty = '━', '━'
	pg.EmptyColor = curTheme.Track
	return pg
}
