package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/kevin/yt-tui/internal/thumb"
	"github.com/kevin/yt-tui/internal/yt"
)

const (
	minWidth  = 60
	minHeight = 16
	sideWidth = 36 // panneau latéral « En lecture » / « Aperçu », masqué sous 100 colonnes
	numW      = 5  // colonne « # » des lignes de piste
	durW      = 8  // colonne « Durée »
)

// margins renvoie la marge horizontale (colonnes) et verticale (lignes) autour de l'application,
// réduite sur les petits terminaux pour ne pas manger la place utile.
func margins(w, h int) (mx, my int) {
	mx, my = 2, 1
	if w < 80 {
		mx = 1
	}
	if h < 24 {
		my = 0
	}
	return mx, my
}

// withMargins entoure l'écran de marges (colonnes à gauche, lignes en haut et en bas).
func withMargins(screen string, mx, my int) string {
	left := strings.Repeat(" ", mx)
	lines := strings.Split(screen, "\n")
	for i, l := range lines {
		lines[i] = left + l
	}
	blank := make([]string, my)
	return strings.Join(append(append(blank, lines...), blank...), "\n")
}

type geom struct{ bodyH, playerH, sideW, leftW int }

func (m *Model) geom() geom {
	playerH := 3
	if m.tab == tabQueue {
		playerH = 4
	}
	g := geom{playerH: playerH, bodyH: max(6, m.height-1-playerH-2)}
	if m.width >= 100 {
		g.sideW = sideWidth
	}
	g.leftW = m.width
	if g.sideW > 0 {
		g.leftW = m.width - g.sideW - 1
	}
	return g
}

func (m *Model) libWidths() (a, b, t int) {
	a, b = max(16, m.width*22/100), max(20, m.width*26/100)
	return a, b, m.width - a - b - 2
}

func (m *Model) plWidths() (l, r int) {
	l = max(22, m.width*28/100)
	return l, m.width - l - 1
}

// layout recale la taille de chaque liste selon l'onglet et le terminal.
func (m *Model) layout() {
	if m.width == 0 {
		return
	}
	g := m.geom()
	b := g.bodyH
	m.queueL.SetSize(g.leftW-2, b-5)
	m.searchL.SetSize(g.leftW-2, b-6)
	aW, bW, tW := m.libWidths()
	m.libArtists.SetSize(aW-2, b-3)
	m.libAlbums.SetSize(bW-2, b-3)
	m.libTracks.SetSize(tW-2, b-6)
	lW, rW := m.plWidths()
	m.plL.SetSize(lW-2, b-4)
	m.plTracks.SetSize(rW-2, b-5)
	m.input.Width = max(8, m.width-2-3-lipgloss.Width(m.chipsView())-3)
	m.prompt.Width = max(10, min(50, m.width-50))
}

func (m Model) View() string {
	if m.termW < minWidth || m.termH < minHeight {
		return txt("Terminal trop petit (%d×%d) — minimum %d×%d", m.termW, m.termH, minWidth, minHeight)
	}
	g := m.geom()
	var body string
	switch m.tab {
	case tabQueue:
		body = m.queueBody(g)
	case tabSearch:
		body = m.searchBody(g)
	case tabLibrary:
		body = m.libraryBody(g)
	case tabPlaylists:
		body = m.playlistsBody(g)
	default:
		body = m.downloadsPanel(g)
	}
	bottom := m.playerBar()
	if m.prompting {
		bottom = m.promptBar(g.playerH)
	}
	screen := strings.Join([]string{m.headerView(), body, bottom, m.statusView(), m.hintsView()}, "\n")
	mx, my := margins(m.termW, m.termH)
	screen = withMargins(screen, mx, my)
	if m.cfg.Transparent {
		return screen
	}
	on, _ := bgCodes()
	return paintBackground(screen, m.termW, m.termH, on)
}

// ---------- en-tête, statut, aide ----------

func (m Model) headerView() string {
	parts := []string{logoStyle.Render("♪ yt-tui"), "  "}
	for i := 0; i < int(numTabs); i++ {
		label := fmt.Sprintf("%d %s", i+1, tabName(tab(i)))
		if tab(i) == m.tab {
			parts = append(parts, tabActive.Render(label))
		} else {
			parts = append(parts, tabInactive.Render(label))
		}
	}
	left := strings.Join(parts, " ")
	right := mutedStyle.Render(m.summary())
	gap := max(1, m.width-lipgloss.Width(left)-lipgloss.Width(right)-1)
	return trunc(left+strings.Repeat(" ", gap)+right, m.width)
}

func (m Model) summary() string {
	switch m.tab {
	case tabQueue:
		return plural(len(m.queue), "piste", "pistes") + " · " + fmtTime(totalDuration(m.queue).Seconds())
	case tabSearch:
		if m.loading {
			return txt("recherche…")
		}
		return plural(m.searchCount(), "résultat", "résultats")
	case tabLibrary:
		all := m.lib.Tracks()
		albums := map[string]bool{}
		for _, t := range all {
			albums[t.Channel+"\x00"+t.Album] = true
		}
		return plural(len(all), "piste", "pistes") + " · " + plural(len(albums), "album", "albums") + " · " + shortPath(m.lib.Dir())
	case tabPlaylists:
		return plural(len(m.plNames), "playlist", "playlists")
	}
	return plural(len(m.downloads), "téléchargement", "téléchargements") + " · " + txt("%d en cours", m.activeDownloads())
}

func shortPath(p string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(p, home) {
		return "~" + strings.TrimPrefix(p, home)
	}
	return p
}

func (m Model) statusView() string {
	s := m.status
	if m.loading {
		s = m.spin.View() + " " + s
	}
	switch {
	case m.status == "":
		return ""
	case m.statusErr:
		return trunc(errStyle.Render("✘ "+s), m.width)
	}
	return trunc(mutedStyle.Render(" "+s), m.width)
}

func (m Model) hintsView() string {
	var h []hint
	switch m.tab {
	case tabQueue:
		h = []hint{{"↑/↓", "naviguer"}, {"entrée", "lire"}, {"espace", "pause"}, {"d", "retirer"}, {"J/K", "déplacer"},
			{"P", "→ playlist"}, {"p/n", "préc./suiv."}, {"s", "aléatoire"}, {"l", "répéter"}, {"R", "radio"}, {"z", "minuteur"}, {"f", "favori"}, {"tab", "vue"}, {"q", "quitter"}}
	case tabSearch:
		if m.searching {
			h = []hint{{"entrée", "rechercher"}, {"↑/↓", "résultats"}, {"ctrl+f", "filtre type"}, {"esc", "quitter la saisie"}}
		} else if m.drill != nil {
			h = []hint{{"↑/↓", "naviguer"}, {"entrée", "lire"}, {"a", "ajouter"}, {"A", "tout ajouter"}, {"P", "→ playlist"}, {"D", "télécharger"}, {"esc", "retour"}, {"/", "chercher"}}
		} else {
			h = []hint{{"↑/↓", "résultats"}, {"entrée", "lire / ouvrir"}, {"a", "ajouter"}, {"P", "→ playlist"}, {"D", "télécharger"}, {"ctrl+f", "filtre type"}, {"/", "chercher"}}
		}
	case tabLibrary:
		h = []hint{{"←/→", "colonne"}, {"↑/↓", "naviguer"}, {"entrée", "lire"}, {"a", "ajouter à la file"}, {"P", "→ playlist"},
			{"f", "favori"}, {"e", "tags"}, {"X", "supprimer"}, {"r", "réanalyser"}, {"/", "filtrer"}}
	case tabPlaylists:
		h = []hint{{"←/→", "liste / pistes"}, {"entrée", "lire la playlist"}, {"a", "ajouter à la file"}, {"J/K", "réordonner"},
			{"d", "retirer"}, {"R", "renommer"}, {"X", "supprimer"}, {"e", "export M3U"}, {"f", "favori"}, {"N", "nouvelle"}}
	default:
		h = []hint{{"↑/↓", "naviguer"}, {"d", "relancer un échec"}, {"tab", "vue"}, {"q", "quitter"}}
	}
	return renderHints(h, m.width)
}

// ---------- barre de lecture / saisie ----------

func (m Model) stateLabel() string {
	switch {
	case !m.hasCur:
		return mutedStyle.Render(txt("♪ Rien en lecture"))
	case !m.playing:
		return mutedStyle.Render(txt("■ Arrêt"))
	case m.paused:
		return mutedStyle.Render(txt("⏸ Pause"))
	}
	return pinkBold.Render(txt("▶ Lecture"))
}

func (m Model) playerBar() string {
	iw := m.width - 2
	if m.tab != tabQueue {
		line := " " + m.stateLabel()
		if m.hasCur {
			line += "  " + textStyle.Render(m.cur.Title) + mutedStyle.Render(" — "+m.cur.Channel+" · "+fmtTime(m.pos)+" / "+fmtTime(m.dur))
		} else {
			line += mutedStyle.Render(txt("  — / pour rechercher de la musique"))
		}
		right := ""
		if m.tab == tabLibrary && !m.lib.ScannedAt().IsZero() {
			right = txt("Dernière analyse : %s", ago(m.lib.ScannedAt())) + " "
		}
		return panel(spread(line, mutedStyle.Render(right), iw), m.width, 3, cBorderOff)
	}
	optTxt := txt("⇄ aléatoire %s · ↻ répéter : %s", onOff(m.shuffle), m.repeatLabel())
	if m.radio {
		optTxt += txt(" · ◉ radio")
	}
	if s := m.sleepLabel(); s != "" {
		optTxt += " · " + s
	}
	opts := mutedStyle.Render(optTxt + txt(" · vol %.0f%% ", m.vol))
	line1 := " " + m.stateLabel()
	if m.hasCur {
		line1 += "  " + boldText.Render(m.cur.Title) + mutedStyle.Render(" — "+m.cur.Channel)
	}
	times := mutedStyle.Render(fmtTime(m.pos))
	total := mutedStyle.Render(fmtTime(m.dur))
	p := m.prog
	p.Width = max(8, iw-lipgloss.Width(times)-lipgloss.Width(total)-4)
	frac := 0.0
	if m.dur > 0 {
		frac = min(1, m.pos/m.dur)
	}
	line2 := " " + times + " " + p.ViewAs(frac) + " " + total
	return panel(spread(line1, opts, iw)+"\n"+trunc(line2, iw), m.width, 4, cBorderOn)
}

// spread place left à gauche et right à droite sur w colonnes.
func spread(left, right string, w int) string {
	gap := max(1, w-lipgloss.Width(left)-lipgloss.Width(right))
	return trunc(left+strings.Repeat(" ", gap)+right, w)
}

func (m Model) promptBar(h int) string {
	iw := m.width - 2
	hint := txt("entrée valider · esc annuler")
	if m.promptKind.isConfirm() {
		hint = txt("entrée confirmer · esc annuler")
	}
	left := " " + pinkBold.Render(m.promptLabel)
	if !m.promptKind.isConfirm() {
		left += " " + m.prompt.View()
	}
	return panel(spread(left, mutedStyle.Render(hint+" "), iw), m.width, h, cPink)
}

func ago(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return txt("à l'instant")
	case d < time.Hour:
		return fmt.Sprintf("%d min", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%d h", int(d.Hours()))
	}
	return fmt.Sprintf("%d j", int(d.Hours()/24))
}

// ---------- blocs ----------

// block force un texte à occuper exactement h lignes de largeur w.
func block(lines []string, w, h int) string {
	out := make([]string, h)
	for i := range out {
		if i < len(lines) {
			out[i] = trunc(lines[i], w)
		}
	}
	return strings.Join(out, "\n")
}

// listOrEmpty affiche la liste, ou un message quand elle est vide.
func listOrEmpty(v *vlist, msg string) string {
	if v.Len() == 0 {
		return block([]string{"", " " + mutedStyle.Render(msg)}, v.w, v.h)
	}
	return v.View()
}

func tableHead(w int, left string) string {
	nw, dw := rowCols(w, true)
	line := " " + mutedStyle.Render(fit(pick3(nw >= 5, "#", ""), nw, false)) + strings.Repeat(" ", thumb.Cols+2) +
		mutedStyle.Render(fit(left, max(1, w-1-nw-thumb.Cols-2-dw), false))
	if dur := txt("Durée"); dw > 0 {
		line += mutedStyle.Render(strings.Repeat(" ", max(0, dw-lipgloss.Width(dur))) + dur)
	}
	return trunc(line, w)
}

func pick3(c bool, a, b string) string {
	if c {
		return a
	}
	return b
}

func rule(w int) string {
	return lipgloss.NewStyle().Foreground(cBorderOff).Render(strings.Repeat("─", w))
}

// ---------- File ----------

func (m Model) queueBody(g geom) string {
	iw := g.leftW - 2
	inner := titleLine(txt("File d'attente"), "", iw) + "\n" + tableHead(iw, txt("Titre · Chaîne")) + "\n" + rule(iw) + "\n" +
		listOrEmpty(&m.queueL, txt("La file est vide. Appuyez sur a pour ajouter un morceau."))
	left := panel(inner, g.leftW, g.bodyH, cBorderOn)
	if g.sideW == 0 {
		return left
	}
	return joinH(left, m.nowPlayingPanel(g.sideW, g.bodyH))
}

func (m Model) nowPlayingPanel(w, h int) string {
	iw := w - 2
	lines := []string{titleLine(txt("En lecture"), "", iw)}
	if !m.hasCur {
		lines = append(lines, "", " "+mutedStyle.Render(txt("Rien en lecture.")))
		return panel(block(lines, iw, h-2), w, h, cBorderOff)
	}
	lines = append(lines, m.artLines(m.cur.ID, iw, h)...)
	lines = append(lines, m.wrap(boldText, m.cur.Title, iw-2, 2)...)
	lines = append(lines, " "+textStyle.Render(m.cur.Channel))
	src := txt("YouTube · streaming")
	if m.curLocal {
		src = txt("local · %s", m.curFormat)
	}
	lines = append(lines, " "+mutedStyle.Render(src), " "+mutedStyle.Render(txt("Durée · %s", fmtTime(m.dur))), "", " "+mutedStyle.Render(txt("À suivre")))
	next := "—"
	switch {
	case m.shuffle:
		next = txt("aléatoire")
	case m.qpos >= 0 && m.qpos+1 < len(m.queue):
		next = m.queue[m.qpos+1].Title
	}
	lines = append(lines, " "+textStyle.Render(next))
	return panel(block(lines, iw, h-2), w, h, cBorderOff)
}

// artLines renvoie la grande image (ou un cadre gris) entourée d'une ligne vide, si la place le permet.
func (m Model) artLines(id string, iw, h int) []string {
	if h < 22 {
		return []string{""}
	}
	art, ok := m.thumbs.Large(id)
	if !ok {
		art = bigPlaceholder("♫")
	}
	out := []string{""}
	for _, l := range art {
		out = append(out, " "+l)
	}
	return append(out, "")
}

func bigPlaceholder(glyph string) []string {
	bg := lipgloss.NewStyle().Background(lipgloss.AdaptiveColor{Light: "#d1d5db", Dark: "#2c2748"}).Foreground(cMuted)
	out := make([]string, thumb.LargeRows)
	for i := range out {
		out[i] = bg.Render(strings.Repeat(" ", thumb.LargeCols))
	}
	pad := (thumb.LargeCols - lipgloss.Width(glyph)) / 2
	out[thumb.LargeRows/2] = bg.Render(strings.Repeat(" ", pad) + glyph + strings.Repeat(" ", thumb.LargeCols-pad-lipgloss.Width(glyph)))
	return out
}

// wrap renvoie au plus n lignes de s, retournées à w colonnes, avec une marge d'une colonne.
func (m Model) wrap(st lipgloss.Style, s string, w, n int) []string {
	var out []string
	for _, l := range strings.Split(lipgloss.NewStyle().Width(w).Render(s), "\n") {
		out = append(out, " "+st.Render(strings.TrimRight(l, " ")))
		if len(out) == n {
			break
		}
	}
	return out
}

// ---------- Recherche ----------

func (m Model) chipsView() string {
	parts := make([]string, numKinds)
	for i := range parts {
		n := kindName(searchKind(i))
		if searchKind(i) == m.kind {
			parts[i] = chipActive.Render(n)
		} else {
			parts[i] = mutedStyle.Padding(0, 1).Render(n)
		}
	}
	return strings.Join(parts, " ")
}

func (m Model) searchBody(g geom) string {
	prefix := pinkBold.Render("/")
	if m.loading {
		prefix = m.spin.View()
	}
	box := " " + prefix + " " + m.input.View()
	box = spread(box, m.chipsView()+" ", m.width-2)
	color := borderFor(false)
	if m.searching {
		color = cPink
	}
	input := panel(box, m.width, 3, color)

	h := g.bodyH - 3
	iw := g.leftW - 2
	title, extra := txt("Résultats"), plural(m.searchCount(), "résultat", "résultats")
	if m.drill != nil {
		title, extra = txt("Playlist · %s", m.drill.title), txt("esc : retour")
	}
	inner := titleLine(title, extra, iw) + "\n" + listOrEmpty(&m.searchL, m.searchEmptyMsg())
	left := panel(inner, g.leftW, h, cBorderOn)
	if g.sideW == 0 {
		return input + "\n" + left
	}
	return input + "\n" + joinH(left, m.previewPanel(g.sideW, h))
}

func (m Model) searchEmptyMsg() string {
	if m.loading {
		return txt("Recherche en cours…")
	}
	return txt("Tapez une recherche puis Entrée.")
}

func (m Model) previewPanel(w, h int) string {
	iw := w - 2
	lines := []string{titleLine(txt("Aperçu"), "", iw)}
	switch {
	case func() bool { _, ok := m.searchTrack(); return ok }():
		t, _ := m.searchTrack()
		lines = append(lines, m.artLines(t.ID, iw, h)...)
		lines = append(lines, m.wrap(boldText, t.Title, iw-2, 2)...)
		lines = append(lines, " "+textStyle.Render(t.Channel))
		if t.Duration > 0 {
			lines = append(lines, " "+mutedStyle.Render(txt("Durée · %s", fmtTime(t.Duration.Seconds()))))
		}
		lines = append(lines, "", " "+keyStyle.Render(txt("entrée"))+mutedStyle.Render(txt(" : lire")),
			" "+keyStyle.Render("a")+mutedStyle.Render(txt(" : ajouter à la file")),
			" "+keyStyle.Render("P")+mutedStyle.Render(txt(" : ajouter à une playlist")),
			" "+keyStyle.Render("D")+mutedStyle.Render(txt(" : télécharger")))
	default:
		e, ok := m.searchEntry()
		if !ok {
			lines = append(lines, "", " "+mutedStyle.Render(txt("Sélectionnez un résultat.")))
			break
		}
		kind := txt(map[yt.Kind]string{yt.KindPlaylist: "Playlist", yt.KindChannel: "Chaîne"}[e.Kind])
		lines = append(lines, "")
		lines = append(lines, m.wrap(boldText, e.Title, iw-2, 3)...)
		lines = append(lines, " "+mutedStyle.Render(kind+" · "+e.Channel), "")
		switch p := m.previews[entryKey(e)]; {
		case p == nil || p.loading:
			lines = append(lines, " "+mutedStyle.Render(txt("Chargement…")))
		case p.err != nil:
			lines = append(lines, " "+errStyle.Render(txt("Aperçu indisponible")))
		default:
			for i, t := range p.tracks {
				lines = append(lines, " "+mutedStyle.Render(itoa(i+1)+" ")+textStyle.Render(t.Title))
			}
			lines = append(lines, "", " "+keyStyle.Render(txt("entrée"))+mutedStyle.Render(txt(" : ouvrir")))
		}
	}
	return panel(block(lines, iw, h-2), w, h, cBorderOff)
}

// ---------- Bibliothèque ----------

func (m Model) libraryBody(g geom) string {
	aW, bW, tW := m.libWidths()
	b := g.bodyH
	artists := panel(titleLine(txt("Artistes"), "", aW-2)+"\n"+listOrEmpty(&m.libArtists, txt("Aucune piste.")), aW, b, borderFor(m.libCol == 0))
	albums := panel(titleLine(txt("Albums"), "", bW-2)+"\n"+listOrEmpty(&m.libAlbums, ""), bW, b, borderFor(m.libCol == 1))

	title := strings.Trim(m.libAlbum()+" · "+artistDisplay(m.libArtist()), " ·")
	stats := " " + mutedStyle.Render(m.libStats())
	tracks := panel(titleLine(title, "", tW-2)+"\n"+tableHead(tW-2, txt("Titre"))+"\n"+rule(tW-2)+"\n"+
		listOrEmpty(&m.libTracks, "")+"\n"+stats, tW, b, borderFor(m.libCol == 2))
	return lipgloss.JoinHorizontal(lipgloss.Top, artists, " ", albums, " ", tracks)
}

func (m Model) libStats() string {
	if len(m.libTrackD) == 0 {
		return ""
	}
	fmts := map[string]bool{}
	var list []string
	for _, t := range m.libTrackD {
		f := strings.TrimSpace(strings.SplitN(t.Format, "·", 2)[0])
		if f != "" && !fmts[f] {
			fmts[f] = true
			list = append(list, f)
		}
	}
	return plural(len(m.libTrackD), "piste", "pistes") + " · " + fmtTime(totalDuration(m.libTrackD).Seconds()) + " · " + strings.Join(list, ", ")
}

// ---------- Playlists ----------

func (m Model) playlistsBody(g geom) string {
	lW, rW := m.plWidths()
	b := g.bodyH
	newHint := " " + keyStyle.Render("N") + mutedStyle.Render(txt(" : nouvelle playlist"))
	left := panel(titleLine(txt("Mes playlists"), "", lW-2)+"\n"+listOrEmpty(&m.plL, txt("Aucune playlist."))+"\n"+newHint, lW, b, borderFor(m.plCol == 0))

	title := plDisplay(m.plName())
	extra := ""
	if title == "" {
		title = txt("Playlist")
	} else {
		extra = plural(len(m.plData), "piste", "pistes") + " · " + fmtTime(totalDuration(m.plData).Seconds())
	}
	right := panel(titleLine(title, extra, rW-2)+"\n"+tableHead(rW-2, txt("Titre · Chaîne"))+"\n"+rule(rW-2)+"\n"+
		listOrEmpty(&m.plTracks, txt("Cette playlist est vide. Ajoutez des pistes avec P.")), rW, b, borderFor(m.plCol == 1))
	return joinH(left, right)
}

// ---------- Téléchargements ----------

func (m Model) downloadsPanel(g geom) string {
	iw := m.width - 2
	inner := titleLine("Téléchargements", m.cfg.MusicDir, iw) + "\n\n" + m.downloadsBody(iw, g.bodyH-4)
	return panel(inner, m.width, g.bodyH, cBorderOn)
}
