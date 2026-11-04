package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kevin/yt-tui/internal/config"
	"github.com/kevin/yt-tui/internal/library"
	"github.com/kevin/yt-tui/internal/playlist"
	"github.com/kevin/yt-tui/internal/session"
	"github.com/kevin/yt-tui/internal/thumb"
	"github.com/kevin/yt-tui/internal/yt"
)

func tr(i int) yt.Track {
	return yt.Track{ID: fmt.Sprintf("id%09d", i), Title: fmt.Sprintf("Piste numéro %d avec un titre assez long pour déborder ✨", i),
		Channel: fmt.Sprintf("Chaîne %d", i%3), Duration: time.Duration(120+i) * time.Second}
}

func testModel(t *testing.T, w, h int) Model {
	t.Helper()
	dir := t.TempDir()
	for i := 0; i < 4; i++ { // quelques fichiers locaux
		os.WriteFile(filepath.Join(dir, fmt.Sprintf("Local %d [id%09d].mp3", i, i)), []byte("x"), 0o644)
	}
	lib, err := library.New(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	store, _ := playlist.Open(filepath.Join(t.TempDir(), "p.json"))
	for i := 0; i < 5; i++ {
		store.Add("Concentration", tr(i))
	}
	store.Create("Vide")
	m := New(context.Background(), config.Config{MusicDir: dir, Results: 20}, nil, lib, store, thumb.New(t.TempDir()), nil)
	for i := 0; i < 12; i++ {
		m.queue = append(m.queue, tr(i))
	}
	m.qpos, m.hasCur, m.cur, m.playing, m.dur, m.pos = 2, true, tr(2), true, 200, 66
	m.syncAll()
	m.resVideos = []yt.Track{tr(20), tr(21), tr(22), tr(23)}
	m.resPlays = []yt.Entry{{Kind: yt.KindPlaylist, ID: "PLabc", Title: "Un album complet", Channel: "Artiste"}}
	m.resChans = []yt.Entry{{Kind: yt.KindChannel, ID: "UCabc", Title: "Une chaîne"}}
	m.syncSearch()
	m.downloads = []*dlRow{{id: 0, track: tr(1), state: dlRunning, pct: 40}, {id: 1, track: tr(2), state: dlFailed, err: fmt.Errorf("boom")}}
	nm, _ := m.update(tea.WindowSizeMsg{Width: w, Height: h})
	m = nm.(Model)
	m.layout()
	return m
}

func TestViewFitsTerminal(t *testing.T) { viewFits(t) }

func viewFits(t *testing.T) {
	for _, sz := range [][2]int{{140, 40}, {100, 30}, {80, 24}, {60, 16}} {
		for tb := tab(0); tb < numTabs; tb++ {
			for _, prompting := range []bool{false, true} {
				m := testModel(t, sz[0], sz[1])
				m.tab, m.searching = tb, false
				if prompting {
					m.openPrompt(promptNew, "Nouvelle playlist", "", "", nil)
				}
				m.layout()
				out := m.View()
				lines := strings.Split(out, "\n")
				if len(lines) != sz[1] {
					t.Errorf("%dx%d %s prompt=%v : %d lignes, attendu %d", sz[0], sz[1], tabName(tb), prompting, len(lines), sz[1])
				}
				for i, l := range lines {
					if w := lipgloss.Width(l); w > sz[0] {
						t.Errorf("%dx%d %s prompt=%v ligne %d : largeur %d > %d : %q", sz[0], sz[1], tabName(tb), prompting, i, w, sz[0], l)
						break
					}
				}
			}
		}
	}
}

func TestQueueOrderingAndModes(t *testing.T) {
	m := testModel(t, 120, 36) // qpos = 2, file de 12
	if i, ok := m.nextIndex(false); !ok || i != 3 {
		t.Errorf("suivant = %d,%v", i, ok)
	}
	m.qpos = 11
	if _, ok := m.nextIndex(true); ok {
		t.Error("fin de file sans répétition : doit s'arrêter")
	}
	m.repeat = repeatQueue
	if i, ok := m.nextIndex(true); !ok || i != 0 {
		t.Errorf("répéter file = %d,%v", i, ok)
	}
	m.repeat = repeatTrack
	if i, ok := m.nextIndex(true); !ok || i != 11 {
		t.Errorf("répéter piste (auto) = %d,%v", i, ok)
	}
	if i, ok := m.nextIndex(false); ok && i == 11 {
		t.Error("suivant manuel ne doit pas rejouer la piste")
	}
	// aléatoire : visite chaque piste une fois puis s'arrête
	m.repeat, m.shuffle, m.qpos = repeatOff, true, 0
	m.resetOrder()
	seen := map[int]bool{0: true}
	for k := 0; k < 11; k++ {
		i, ok := m.nextIndex(false)
		if !ok || seen[i] {
			t.Fatalf("tirage %d = %d,%v (déjà vu : %v)", k, i, ok, seen[i])
		}
		seen[i], m.played[i], m.qpos = true, true, i
	}
	if _, ok := m.nextIndex(false); ok {
		t.Error("aléatoire sans répétition : doit s'arrêter après 12 pistes")
	}
}

func TestMoveAndRemoveKeepCurrentTrack(t *testing.T) {
	m := testModel(t, 120, 36) // qpos = 2
	cur := m.queue[m.qpos].ID
	m.moveInQueue(2, 1)
	if m.qpos != 3 || m.queue[3].ID != cur {
		t.Errorf("après déplacement : qpos=%d", m.qpos)
	}
	m.moveInQueue(0, 1) // ne touche pas la piste en cours
	if m.queue[m.qpos].ID != cur {
		t.Error("le déplacement d'une autre piste a perdu le morceau courant")
	}
	m.removeFromQueue(0)
	if m.queue[m.qpos].ID != cur {
		t.Error("retirer une piste avant la courante a décalé qpos")
	}
	m.removeFromQueue(m.qpos)
	if m.qpos != -1 {
		t.Errorf("retirer la piste courante : qpos=%d, attendu -1", m.qpos)
	}
	if m.moveInQueue(0, -1) != 0 {
		t.Error("déplacement hors bornes accepté")
	}
}

func TestPaintBackground(t *testing.T) {
	on := "\x1b[48;2;20;18;31m"
	in := "\x1b[1mtitre\x1b[0m suite\n\x1b[38;5;1mrouge\x1b[m fin"
	out := paintBackground(in, 20, 3, on)
	lines := strings.Split(out, "\n")
	if len(lines) != 3 {
		t.Fatalf("%d lignes, attendu 3 (complétées jusqu'à la hauteur)", len(lines))
	}
	for i, l := range lines {
		if !strings.HasPrefix(l, on) {
			t.Errorf("ligne %d sans fond au début : %q", i, l)
		}
		if w := lipgloss.Width(l); w != 20 {
			t.Errorf("ligne %d : largeur %d, attendu 20", i, w)
		}
	}
	// après chaque reset le fond est réappliqué
	if !strings.Contains(lines[0], "\x1b[0m"+on+" suite") || !strings.Contains(lines[1], "\x1b[m"+on+" fin") {
		t.Errorf("fond non réappliqué après reset : %q / %q", lines[0], lines[1])
	}
	if paintBackground(in, 20, 3, "") != in {
		t.Error("sans couleur, l'écran doit rester inchangé")
	}
}

func TestMargins(t *testing.T) {
	m := testModel(t, 120, 36)
	lines := strings.Split(m.View(), "\n")
	if len(lines) != 36 {
		t.Fatalf("%d lignes, attendu 36", len(lines))
	}
	if strings.TrimSpace(lines[0]) != "" || strings.TrimSpace(lines[35]) != "" {
		t.Errorf("marges haut/bas non vides : %q / %q", lines[0], lines[35])
	}
	for i, l := range lines[1:35] {
		if !strings.HasPrefix(l, "  ") {
			t.Errorf("ligne %d sans marge gauche : %q", i+1, l)
			break
		}
		if w := lipgloss.Width(l); w > 120-2 {
			t.Errorf("ligne %d empiète sur la marge droite (largeur %d) : %q", i+1, w, l)
			break
		}
	}
	// petit terminal : marges réduites
	if mx, my := margins(70, 20); mx != 1 || my != 0 {
		t.Errorf("marges petit terminal = %d,%d", mx, my)
	}
	if mx, my := margins(140, 40); mx != 2 || my != 1 {
		t.Errorf("marges normales = %d,%d", mx, my)
	}
}

func TestAddAllFromOpenedPlaylist(t *testing.T) {
	m := testModel(t, 110, 30)
	m.queue, m.qpos = nil, -1
	m.tab, m.searching = tabSearch, false
	mk := func(n int) []yt.Track {
		var ts []yt.Track
		for i := 0; i < n; i++ {
			ts = append(ts, tr(30+i))
		}
		return ts
	}
	// ≤ 10 pistes : ajout direct.
	m.drill = &drill{title: "P", tracks: mk(10)}
	m.syncSearch()
	if ok, _ := m.keySearch("A"); !ok || len(m.queue) != 10 || m.prompting {
		t.Fatalf("10 pistes : ajout direct attendu (file=%d, prompt=%v)", len(m.queue), m.prompting)
	}
	// > 10 pistes : confirmation, puis ajout.
	m.queue = nil
	m.drill = &drill{title: "P", tracks: mk(11)}
	m.syncSearch()
	m.keySearch("A")
	if !m.prompting || len(m.queue) != 0 {
		t.Fatal("11 pistes : confirmation attendue, rien d'ajouté avant")
	}
	m.closePrompt()
	if len(m.queue) != 0 {
		t.Fatal("annuler ne doit rien ajouter")
	}
	m.keySearch("A")
	m.closePrompt()
	m.applyPrompt(m.promptKind, "", "", m.promptTrack)
	if len(m.queue) != 11 {
		t.Errorf("file = %d, attendu 11", len(m.queue))
	}
	// Hors playlist ouverte : sans effet.
	m.drill = nil
	if ok, _ := m.keySearch("A"); ok {
		t.Error("A hors playlist ouverte ne doit rien faire")
	}
}

func TestQuitAsksConfirmation(t *testing.T) {
	m := testModel(t, 110, 30)
	m.tab, m.searching = tabQueue, false
	r, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = r.(Model)
	if !m.prompting || m.promptKind != promptQuit || cmd != nil {
		t.Fatal("q doit ouvrir la confirmation sans quitter")
	}
	r, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = r.(Model)
	if m.prompting {
		t.Fatal("esc doit annuler")
	}
	r, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	_, cmd = r.(Model).handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("entrée doit quitter")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("la commande doit être tea.Quit")
	}
}

func TestSessionRestoreAndSnapshot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.json")
	st := session.State{Queue: []yt.Track{tr(1), tr(2), tr(3)}, Pos: 1, At: 77, Shuffle: true, Repeat: 1, Volume: 55}
	if err := session.Save(path, st); err != nil {
		t.Fatal(err)
	}
	lib, _ := library.New(t.TempDir(), "")
	ps, _ := playlist.Open(filepath.Join(dir, "p.json"))
	cfg := config.Config{MusicDir: dir, ConfigDir: dir, Results: 10}
	m := New(context.Background(), cfg, nil, lib, ps, thumb.New(t.TempDir()), nil)
	if len(m.queue) != 3 || m.qpos != 1 || !m.hasCur || m.playing || !m.restored || m.resumeAt != 77 ||
		!m.shuffle || m.repeat != repeatQueue || m.vol != 55 {
		t.Fatalf("restauration incorrecte : q=%d pos=%d cur=%v playing=%v restored=%v at=%v shuf=%v rep=%v vol=%v",
			len(m.queue), m.qpos, m.hasCur, m.playing, m.restored, m.resumeAt, m.shuffle, m.repeat, m.vol)
	}
	snap := m.snapshot()
	if snap.Pos != 1 || snap.At != 77 || len(snap.Queue) != 3 || snap.Volume != 55 {
		t.Errorf("snapshot = %+v", snap)
	}
	cfg.NoResume = true
	if m2 := New(context.Background(), cfg, nil, lib, ps, thumb.New(t.TempDir()), nil); len(m2.queue) != 0 {
		t.Error("--no-resume doit ignorer la session")
	}
}

func TestSleepTimerCycleAndExpiry(t *testing.T) {
	m := testModel(t, 110, 30)
	clock := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return clock }
	want := []string{"⏾ 15 min", "⏾ 30 min", "⏾ 60 min", "⏾ fin de piste", ""}
	for i, w := range want {
		m.cycleSleep()
		if got := m.sleepLabel(); got != w {
			t.Fatalf("étape %d : %q, attendu %q", i, got, w)
		}
	}
	// Échéance : le minuteur se réinitialise (m.pl est nil dans ce test → pas en lecture).
	m.playing = false
	m.cycleSleep()
	clock = clock.Add(16 * time.Minute)
	m.checkSleep()
	if !m.sleepAt.IsZero() {
		t.Error("le minuteur doit être effacé à l'échéance")
	}
}

func TestRadioQueueDedup(t *testing.T) {
	m := testModel(t, 110, 30)
	m.radio = true
	before := len(m.queue)
	existing := m.queue[0]
	r, _ := m.Update(radioMsg{seed: "x", tracks: []yt.Track{existing, tr(90), tr(91), tr(90)}})
	m = r.(Model)
	if len(m.queue) != before+2 {
		t.Errorf("file = %d, attendu %d (doublons exclus)", len(m.queue), before+2)
	}
}

func TestFavoritesToggle(t *testing.T) {
	m := testModel(t, 110, 30)
	m.tab = tabQueue
	m.queueL.SetCursor(0)
	id := m.queue[0].ID
	m.toggleFav()
	if got := m.store.Tracks(favName); len(got) != 1 || got[0].ID != id {
		t.Fatalf("favoris après ajout = %+v", got)
	}
	m.toggleFav()
	if got := m.store.Tracks(favName); len(got) != 0 {
		t.Errorf("favoris après retrait = %+v", got)
	}
}

func TestRecentIsReadOnlyAndReserved(t *testing.T) {
	m := testModel(t, 110, 30)
	m.hist.Add(tr(5))
	m.syncPlaylists()
	if m.plNames[0] != recentName {
		t.Fatalf("première entrée = %q", m.plNames[0])
	}
	m.plL.SetCursor(0)
	m.syncPlTracks(true)
	if len(m.plData) != 1 || m.plData[0].ID != tr(5).ID {
		t.Fatalf("contenu de Récents = %+v", m.plData)
	}
	m.tab = tabPlaylists
	for _, k := range []string{"X", "R", "d", "J", "K"} {
		m.prompting = false
		if _, cmd := m.keyPlaylists(k); cmd != nil || m.prompting {
			t.Errorf("touche %q ne doit rien faire sur Récents", k)
		}
	}
	m.applyPrompt(promptNew, recentName, "", nil)
	for _, n := range m.store.Names() {
		if n == recentName {
			t.Error("le nom Récents doit être réservé")
		}
	}
}

func TestExportM3U(t *testing.T) {
	m := testModel(t, 110, 30)
	m.tab = tabPlaylists
	for i, n := range m.plNames {
		if n == "Concentration" {
			m.plL.SetCursor(i)
		}
	}
	m.syncPlTracks(true)
	m.exportM3U()
	b, err := os.ReadFile(filepath.Join(m.cfg.MusicDir, "Concentration.m3u"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(b), "#EXTM3U") || strings.Count(string(b), "#EXTINF") != 5 {
		t.Errorf("M3U inattendu :\n%s", b)
	}
}

func TestDeleteFileAsksThenRemoves(t *testing.T) {
	m := testModel(t, 110, 30)
	m.tab, m.libCol = tabLibrary, 2
	m.syncLibrary()
	if len(m.libTrackD) == 0 {
		t.Skip("aucune piste locale analysée")
	}
	m.libTracks.SetCursor(0)
	id := m.libTrackD[0].ID
	if _, cmd := m.keyLibrary("X"); cmd != nil || !m.prompting || m.promptKind != promptDeleteFile {
		t.Fatalf("X doit demander confirmation (prompting=%v kind=%v)", m.prompting, m.promptKind)
	}
	m.closePrompt()
	if _, ok := m.lib.Path(id); !ok {
		t.Fatal("annuler ne doit rien supprimer")
	}
	m.applyPrompt(promptDeleteFile, "", id, nil)
	if _, ok := m.lib.Path(id); ok {
		t.Error("le fichier devait être supprimé")
	}
}
