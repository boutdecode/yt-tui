package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func mkRows() []row {
	return []row{headerRow{"Chaînes"}, textRow{label: "a"}, textRow{label: "b"},
		headerRow{"Vidéos"}, textRow{label: "c"}, textRow{label: "d"}}
}

func TestVlistSkipsHeaders(t *testing.T) {
	v := &vlist{}
	v.SetSize(30, 10)
	v.SetRows(mkRows())
	if i, ok := v.Cursor(); !ok || i != 1 {
		t.Fatalf("curseur initial = %d,%v ; attendu 1", i, ok)
	}
	v.Move(1)
	v.Move(1) // saute l'en-tête « Vidéos »
	if i, _ := v.Cursor(); i != 4 {
		t.Errorf("après 2 descentes = %d, attendu 4", i)
	}
	v.Move(-1)
	if i, _ := v.Cursor(); i != 2 {
		t.Errorf("remontée = %d, attendu 2", i)
	}
	v.End()
	if i, _ := v.Cursor(); i != 5 {
		t.Errorf("End = %d, attendu 5", i)
	}
	v.Home()
	if i, _ := v.Cursor(); i != 1 {
		t.Errorf("Home = %d, attendu 1", i)
	}
	v.Move(-5) // borné
	if i, _ := v.Cursor(); i != 1 {
		t.Errorf("borne haute = %d", i)
	}
}

func TestVlistViewHeightAndScroll(t *testing.T) {
	v := &vlist{}
	v.SetSize(20, 3)
	v.SetRows(mkRows())
	if n := strings.Count(v.View(), "\n") + 1; n != 3 {
		t.Errorf("View = %d lignes, attendu 3", n)
	}
	v.End() // le curseur doit rester visible malgré h=3
	view := v.View()
	if !strings.Contains(view, "d") || !strings.Contains(view, "Vidéos") {
		t.Errorf("fin de liste invisible ou sans en-tête :\n%s", view)
	}
	for _, l := range strings.Split(view, "\n") {
		if lipgloss.Width(l) > 20 {
			t.Errorf("ligne trop large (%d) : %q", lipgloss.Width(l), l)
		}
	}
	empty := &vlist{}
	empty.SetSize(10, 2)
	empty.SetRows(nil)
	if empty.View() != "\n" {
		t.Errorf("liste vide = %q", empty.View())
	}
	if _, ok := empty.Cursor(); ok {
		t.Error("curseur valide sur liste vide")
	}
}

func TestRowsRenderExactSize(t *testing.T) {
	st := newTestThumbs(t)
	for _, sel := range []bool{false, true} {
		for _, w := range []int{30, 60, 100} {
			for _, r := range []row{
				headerRow{"Section"}, textRow{label: "Un libellé très long qui déborde de la colonne", sub: "· sous-titre"},
				trackRow{t: ytTrack("Un titre très long qui déborde de la colonne disponible ✨", "Chaîne"), num: 12, playing: true, local: true, thumbs: st},
				trackRow{t: ytTrack("Court", "C"), thumbs: st},
			} {
				lines := r.Render(w, sel)
				if len(lines) != r.Height() {
					t.Fatalf("%T : %d lignes, Height()=%d", r, len(lines), r.Height())
				}
				for _, l := range lines {
					if got := lipgloss.Width(l); got != w {
						t.Errorf("%T w=%d sel=%v : largeur %d", r, w, sel, got)
					}
				}
			}
		}
	}
}
