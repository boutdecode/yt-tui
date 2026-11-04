package ui

import "strings"

// row est une ligne (ou un bloc de lignes) d'une vlist.
type row interface {
	Height() int
	Selectable() bool
	// Render renvoie exactement Height() lignes de largeur w.
	Render(w int, sel bool) []string
}

// vlist est une liste verticale à lignes de hauteurs variables (en-têtes de section,
// lignes de texte, pistes avec miniature), avec curseur et défilement.
type vlist struct {
	rows []row
	cur  int
	off  int
	w, h int
}

func (v *vlist) SetSize(w, h int) { v.w, v.h = w, max(1, h) }

// SetRows remplace le contenu en gardant le curseur à sa place (borné, sur une ligne sélectionnable).
func (v *vlist) SetRows(rows []row) {
	v.rows = rows
	v.cur = min(v.cur, len(rows)-1)
	v.cur = max(v.cur, 0)
	v.snap(1)
}

// SetCursor place le curseur sur i (ou la ligne sélectionnable la plus proche).
func (v *vlist) SetCursor(i int) {
	v.cur = max(0, min(i, len(v.rows)-1))
	v.snap(1)
}

func (v *vlist) Len() int { return len(v.rows) }

// Cursor renvoie l'index de la ligne sélectionnée.
func (v *vlist) Cursor() (int, bool) {
	if v.cur < 0 || v.cur >= len(v.rows) || !v.rows[v.cur].Selectable() {
		return 0, false
	}
	return v.cur, true
}

// snap amène le curseur sur une ligne sélectionnable (dir : sens de recherche préféré).
func (v *vlist) snap(dir int) {
	if len(v.rows) == 0 {
		v.cur = 0
		return
	}
	for _, d := range []int{dir, -dir} {
		for i := v.cur; i >= 0 && i < len(v.rows); i += d {
			if v.rows[i].Selectable() {
				v.cur = i
				return
			}
		}
	}
}

// Move déplace le curseur de n lignes sélectionnables (négatif : vers le haut).
func (v *vlist) Move(n int) {
	dir := 1
	if n < 0 {
		dir, n = -1, -n
	}
	for ; n > 0; n-- {
		moved := false
		for i := v.cur + dir; i >= 0 && i < len(v.rows); i += dir {
			if v.rows[i].Selectable() {
				v.cur, moved = i, true
				break
			}
		}
		if !moved {
			return
		}
	}
}

// Page déplace d'environ une page.
func (v *vlist) Page(dir int) {
	n := 0
	for used := 0; used < v.h && n < len(v.rows); n++ {
		used += v.rows[min(len(v.rows)-1, max(0, v.cur+dir*n))].Height()
	}
	v.Move(dir * max(1, n-1))
}

func (v *vlist) Home() { v.cur = 0; v.snap(1) }
func (v *vlist) End()  { v.cur = len(v.rows) - 1; v.snap(-1) }

// Near renvoie les lignes proches du curseur (pour précharger les miniatures).
func (v *vlist) Near(before, after int) []row {
	lo, hi := max(0, v.cur-before), min(len(v.rows), v.cur+after)
	if lo >= hi {
		return nil
	}
	return v.rows[lo:hi]
}

// scroll ajuste off pour que la ligne du curseur (et son en-tête de section) soit visible.
func (v *vlist) scroll() {
	if len(v.rows) == 0 {
		v.off = 0
		return
	}
	top := v.cur
	if top > 0 && !v.rows[top-1].Selectable() { // montre l'en-tête de section au-dessus
		top--
	}
	if top < v.off {
		v.off = top
	}
	for v.off < v.cur {
		used := 0
		for i := v.off; i <= v.cur; i++ {
			used += v.rows[i].Height()
		}
		if used <= v.h {
			break
		}
		v.off++
	}
	v.off = max(0, min(v.off, len(v.rows)-1))
}

// View rend exactement h lignes de largeur w.
func (v *vlist) View() string {
	v.scroll()
	lines := make([]string, 0, v.h)
	for i := v.off; i < len(v.rows) && len(lines) < v.h; i++ {
		lines = append(lines, v.rows[i].Render(v.w, i == v.cur)...)
	}
	if len(lines) > v.h {
		lines = lines[:v.h]
	}
	for len(lines) < v.h {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}
