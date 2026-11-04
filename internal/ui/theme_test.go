package ui

import (
	"image/color"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/kevin/yt-tui/internal/thumb"
)

func TestThemeFromContrast(t *testing.T) {
	for h := 0.0; h < 360; h += 15 {
		for _, s := range []float64{0.3, 0.6, 1} {
			for _, l := range []float64{0.2, 0.5, 0.8} {
				p := thumb.HSLToRGBA(h, s, l)
				th := themeFrom(thumb.Colors{Primary: p, Secondary: thumb.HSLToRGBA(h+90, s, l)})
				if contrast(th.Sel, "#ffffff") < 4.5 {
					t.Errorf("h=%.0f s=%.1f l=%.1f : texte sur sélection illisible (%s)", h, s, l, th.Sel)
				}
				if contrast(th.Bg, "#e8e4f7") < 7 {
					t.Errorf("h=%.0f : texte sur fond illisible (%s)", h, th.Bg)
				}
				if contrast(th.Bg, th.Accent) < 4.5 {
					t.Errorf("h=%.0f : accent sur fond illisible (%s/%s)", h, th.Accent, th.Bg)
				}
			}
		}
	}
}

func TestThemeFollowsAndResets(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev); applyTheme(defaultTheme()) })
	m := testModel(t, 110, 30)
	red := themeFrom(thumb.Colors{Primary: color.RGBA{200, 30, 30, 255}, Secondary: color.RGBA{230, 200, 20, 255}})
	if red == defaultTheme() {
		t.Fatal("le thème rouge ne doit pas retomber sur le défaut")
	}
	m.setTheme(red)
	on, _ := bgCodes()
	if on == "" {
		t.Skip("terminal sans couleurs")
	}
	out := m.View()
	bad := 0
	for _, l := range strings.Split(out, "\n") {
		if !strings.HasPrefix(l, on) {
			bad++
		}
	}
	if bad != 0 {
		t.Errorf("%d lignes sans le fond du thème", bad)
	}
	m.setTheme(defaultTheme())
	if curTheme != defaultTheme() {
		t.Error("retour au thème par défaut impossible")
	}
}
