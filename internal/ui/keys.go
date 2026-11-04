package ui

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kevin/yt-tui/internal/playlist"
	"github.com/kevin/yt-tui/internal/yt"
)

// activeVlist renvoie la liste qui reçoit les flèches dans l'onglet courant.
func (m *Model) activeVlist() *vlist {
	switch m.tab {
	case tabQueue:
		return &m.queueL
	case tabSearch:
		return &m.searchL
	case tabLibrary:
		return [...]*vlist{&m.libArtists, &m.libAlbums, &m.libTracks}[m.libCol]
	case tabPlaylists:
		if m.plCol == 0 {
			return &m.plL
		}
		return &m.plTracks
	}
	return nil
}

// navigate applique les touches de déplacement communes ; renvoie true si la touche a été consommée.
func (m *Model) navigate(s string) bool {
	v := m.activeVlist()
	if v == nil {
		return false
	}
	switch s {
	case "up", "k":
		v.Move(-1)
	case "down", "j":
		v.Move(1)
	case "pgup":
		v.Page(-1)
	case "pgdown":
		v.Page(1)
	case "home", "g":
		v.Home()
	case "end", "G":
		v.End()
	default:
		return false
	}
	// les colonnes dépendantes suivent la sélection
	switch {
	case m.tab == tabLibrary && m.libCol == 0:
		m.syncLibAlbums(true)
	case m.tab == tabLibrary && m.libCol == 1:
		m.syncLibTracks(true)
	case m.tab == tabPlaylists && m.plCol == 0:
		m.syncPlTracks(true)
	}
	return true
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	if s == "ctrl+c" {
		return m, tea.Quit
	}
	if m.prompting {
		return m.keyPrompt(msg)
	}
	if m.tab == tabSearch && m.searching {
		return m.keySearchInput(msg)
	}
	if m.tab == tabDownloads {
		switch s {
		case "up", "k":
			m.dlCursor = max(0, m.dlCursor-1)
			return m, nil
		case "down", "j":
			m.dlCursor = max(0, min(len(m.downloads)-1, m.dlCursor+1))
			return m, nil
		case "d", "D":
			if m.dlCursor < len(m.downloads) && m.downloads[m.dlCursor].state == dlFailed {
				m.enqueueDownload(m.downloads[m.dlCursor].track)
			}
			return m, nil
		}
	}
	if m.navigate(s) {
		return m, nil
	}
	if handled, cmd := m.tabKey(s); handled {
		return m, cmd
	}
	switch s { // touches globales
	case "q":
		return m, m.openPrompt(promptQuit, "Quitter yt-tui ?", "", "", nil)
	case "R":
		m.radio = !m.radio
		m.setStatus("Radio automatique : " + onOff(m.radio))
		if m.radio && m.hasCur && m.qpos == len(m.queue)-1 {
			return m, m.radioCmd()
		}
	case "z":
		m.cycleSleep()
	case "f":
		m.toggleFav()
	case "tab":
		m.tab = (m.tab + 1) % numTabs
	case "shift+tab":
		m.tab = (m.tab + numTabs - 1) % numTabs
	case "1", "2", "3", "4", "5":
		m.tab = tab(s[0] - '1')
		if m.tab == tabSearch {
			m.searching = true
			return m, m.input.Focus()
		}
	case "/":
		m.tab, m.searching = tabSearch, true
		return m, m.input.Focus()
	case " ":
		if m.restored && m.hasCur { // morceau de la session précédente : on le lance à sa position
			at := m.resumeAt
			cmd := m.startTrack(m.qpos, false)
			m.resumeAt = at
			return m, cmd
		}
		m.pl.TogglePause()
	case ",", "left":
		m.pl.Seek(-5)
	case ".", "right":
		m.pl.Seek(5)
	case "+", "=":
		m.pl.SetVolume(m.vol + 5)
	case "-":
		m.pl.SetVolume(m.vol - 5)
	case ">", "n":
		return m, m.goNext(false)
	case "<", "p":
		return m, m.goPrev()
	case "s":
		m.shuffle = !m.shuffle
		m.resetOrder()
		m.setStatus("Lecture aléatoire : " + onOff(m.shuffle))
	case "l":
		m.repeat = (m.repeat + 1) % 3
		m.setStatus("Répéter : " + m.repeatLabel())
	case "a":
		if ts := m.selectedTracks(); len(ts) > 0 {
			m.enqueue(ts...)
			m.setStatus("Ajouté à la file : " + plural(len(ts), "piste", "pistes"))
		}
	case "D":
		for _, t := range m.selectedTracks() {
			m.enqueueDownload(t)
		}
	case "P":
		if ts := m.selectedTracks(); len(ts) > 0 {
			return m, m.openPrompt(promptAdd, "→ playlist", "", "", ts)
		}
	}
	return m, nil
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// tabKey traite les touches propres à l'onglet courant.
func (m *Model) tabKey(s string) (bool, tea.Cmd) {
	switch m.tab {
	case tabQueue:
		i, ok := m.queueL.Cursor()
		switch s {
		case "enter":
			if ok {
				return true, m.playAt(i)
			}
			return true, nil
		case "d":
			if ok {
				m.removeFromQueue(i)
			}
			return true, nil
		case "J":
			if ok {
				m.moveInQueue(i, 1)
			}
			return true, nil
		case "K":
			if ok {
				m.moveInQueue(i, -1)
			}
			return true, nil
		}
	case tabSearch:
		return m.keySearch(s)
	case tabLibrary:
		return m.keyLibrary(s)
	case tabPlaylists:
		return m.keyPlaylists(s)
	}
	return false, nil
}

func (m *Model) keySearch(s string) (bool, tea.Cmd) {
	switch s {
	case "enter":
		ref, ok := m.searchRef()
		if !ok {
			return true, nil
		}
		if e, isEntry := m.searchEntry(); isEntry {
			m.loading = true
			m.setStatus("Ouverture de « " + e.Title + " »…")
			return true, tea.Batch(m.spin.Tick, m.openCmd(e))
		}
		if m.drill != nil { // la collection ouverte devient la file
			m.queue = append([]yt.Track(nil), m.drill.tracks...)
			m.resetOrder()
			m.qpos = -1
			return true, m.startTrack(ref.idx, false)
		}
		if t, ok := m.searchTrack(); ok { // une vidéo : insérée après le morceau en cours
			at := min(m.qpos+1, len(m.queue))
			m.queue = append(m.queue[:at:at], append([]yt.Track{t}, m.queue[at:]...)...)
			m.resetOrder()
			return true, m.startTrack(at, true)
		}
		return true, nil
	case "A":
		if m.drill == nil || len(m.drill.tracks) == 0 {
			return false, nil
		}
		ts := m.drill.tracks
		if len(ts) > maxAddNoConfirm {
			label := "Ajouter les " + plural(len(ts), "piste", "pistes") + " à la file ?"
			return true, m.openPrompt(promptAddAll, label, "", "", ts)
		}
		m.enqueue(ts...)
		m.setStatus("Ajouté à la file : " + plural(len(ts), "piste", "pistes"))
		return true, nil
	case "esc", "backspace":
		if m.drill != nil {
			m.drill = nil
			m.syncSearch()
			return true, nil
		}
	case "ctrl+f":
		m.kind = (m.kind + 1) % numKinds
		m.syncSearch()
		m.searchL.SetCursor(0)
		m.setStatus("Type de recherche : " + kindNames[m.kind])
		return true, nil
	}
	return false, nil
}

func (m *Model) keyLibrary(s string) (bool, tea.Cmd) {
	switch s {
	case "left", "h":
		m.libCol = max(0, m.libCol-1)
		return true, nil
	case "right", "l":
		m.libCol = min(2, m.libCol+1)
		return true, nil
	case "enter":
		ts := m.libSelection()
		if len(ts) == 0 {
			return true, nil
		}
		start := 0
		if m.libCol == 2 {
			start, _ = m.libTracks.Cursor()
			ts = m.libTrackD
		}
		m.queue = append([]yt.Track(nil), ts...)
		m.resetOrder()
		m.qpos = -1
		return true, m.startTrack(start, false)
	case "r":
		return true, m.refreshLibrary()
	case "e":
		if t, ok := m.focusTrack(); ok {
			if _, local := m.lib.Path(t.ID); local {
				return true, m.openPrompt(promptTagArtist, "Artiste", t.Channel, t.ID, nil)
			}
		}
		m.setStatus("Tags : sélectionnez une piste dans la colonne Pistes")
		return true, nil
	case "X":
		if t, ok := m.focusTrack(); ok {
			return true, m.openPrompt(promptDeleteFile, "Supprimer le fichier « "+trunc(t.Title, 40)+" » ?", "", t.ID, nil)
		}
		m.setStatus("Suppression : sélectionnez une piste dans la colonne Pistes")
		return true, nil
	case "/":
		return true, m.openPrompt(promptFilter, "Filtrer", m.libFilter, "", nil)
	case "esc":
		if m.libFilter != "" {
			m.libFilter = ""
			m.syncLibrary()
		}
		return true, nil
	}
	return false, nil
}

func (m *Model) keyPlaylists(s string) (bool, tea.Cmd) {
	i, hasTrack := m.plTracks.Cursor()
	switch s {
	case "left", "h":
		m.plCol = 0
		return true, nil
	case "right", "l":
		if len(m.plData) > 0 {
			m.plCol = 1
		}
		return true, nil
	case "enter":
		if len(m.plData) == 0 {
			return true, nil
		}
		start := 0
		if m.plCol == 1 && hasTrack {
			start = i
		}
		m.queue = append([]yt.Track(nil), m.plData...)
		m.resetOrder()
		m.qpos = -1
		return true, m.startTrack(start, false)
	case "N":
		return true, m.openPrompt(promptNew, "Nouvelle playlist", "", "", nil)
	case "e":
		return true, m.exportM3U()
	case "R", "X", "d", "J", "K":
		if m.plName() == recentName {
			m.setStatus("« Récents » est en lecture seule")
			return true, nil
		}
		return m.keyPlaylistsEdit(s, i, hasTrack)
	}
	return false, nil
}

// keyPlaylistsEdit : touches qui modifient une playlist (jamais « Récents »).
func (m *Model) keyPlaylistsEdit(s string, i int, hasTrack bool) (bool, tea.Cmd) {
	switch s {
	case "R":
		if name := m.plName(); name != "" {
			return true, m.openPrompt(promptRename, "Renommer", name, name, nil)
		}
		return true, nil
	case "X":
		if name := m.plName(); name != "" {
			return true, m.openPrompt(promptDelete, "Supprimer « "+name+" » ?", "", name, nil)
		}
		return true, nil
	case "d":
		if m.plCol == 1 && hasTrack {
			if err := m.store.Remove(m.plName(), i); err != nil {
				m.setError(err.Error())
			}
			m.syncPlaylists()
			m.plTracks.SetCursor(min(i, len(m.plData)-1))
			if len(m.plData) == 0 {
				m.plCol = 0
			}
		}
		return true, nil
	case "J", "K":
		if m.plCol == 1 && hasTrack {
			d := 1
			if s == "K" {
				d = -1
			}
			j, err := m.store.Move(m.plName(), i, d)
			if err != nil {
				m.setError(err.Error())
			}
			m.syncPlTracks(false)
			m.plTracks.SetCursor(j)
		}
		return true, nil
	}
	return false, nil
}

func (m Model) keySearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		q := strings.TrimSpace(m.input.Value())
		if q == "" {
			return m, nil
		}
		m.searching, m.loading = false, true
		m.input.Blur()
		m.setStatus("Recherche de « " + q + " »…")
		return m, tea.Batch(m.spin.Tick, m.searchCmd(q))
	case "esc":
		m.searching = false
		m.input.Blur()
		return m, nil
	case "up", "down", "pgup", "pgdown":
		m.navigate(msg.String())
		return m, nil
	case "ctrl+f":
		_, cmd := m.keySearch("ctrl+f")
		return m, cmd
	case "tab":
		m.searching = false
		m.input.Blur()
		m.tab = (m.tab + 1) % numTabs
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// openPrompt affiche la saisie en bas d'écran (à la place de la barre de lecture).
func (m *Model) openPrompt(k promptKind, label, value, arg string, ts []yt.Track) tea.Cmd {
	m.prompting, m.promptKind, m.promptArg, m.promptTrack = true, k, arg, ts
	m.prompt.SetValue(value)
	m.prompt.CursorEnd()
	m.prompt.Placeholder = map[promptKind]string{
		promptAdd: "nom de la playlist (créée si absente)", promptNew: "nom de la nouvelle playlist",
		promptRename: "nouveau nom", promptFilter: "texte à chercher (titre, artiste, album)",
		promptTagArtist: "nom de l'artiste", promptTagAlbum: "nom de l'album",
	}[k]
	m.promptLabel = label
	if k.isConfirm() {
		return nil
	}
	return m.prompt.Focus()
}

func (m Model) keyPrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.closePrompt()
		return m, nil
	case "enter":
		name := strings.TrimSpace(m.prompt.Value())
		kind, arg, ts := m.promptKind, m.promptArg, m.promptTrack
		m.closePrompt()
		return m, m.applyPrompt(kind, name, arg, ts)
	}
	if m.promptKind.isConfirm() {
		return m, nil
	}
	var cmd tea.Cmd
	m.prompt, cmd = m.prompt.Update(msg)
	if m.promptKind == promptFilter { // filtre en direct
		m.libFilter = strings.TrimSpace(m.prompt.Value())
		m.syncLibrary()
	}
	return m, cmd
}

func (m *Model) closePrompt() {
	m.prompting = false
	m.prompt.Blur()
}

func (m *Model) applyPrompt(kind promptKind, name, arg string, ts []yt.Track) tea.Cmd {
	switch kind {
	case promptFilter:
		m.libFilter = name
		m.syncLibrary()
		return nil
	case promptQuit:
		return tea.Quit
	case promptDeleteFile:
		title := arg
		for _, t := range m.lib.Tracks() {
			if t.ID == arg {
				title = t.Title
			}
		}
		if err := m.lib.Remove(arg); err != nil {
			m.setError("Suppression impossible : " + err.Error())
		} else {
			m.setStatus("Fichier supprimé : " + title)
		}
		m.keepCursors(m.syncAll)
		return nil
	case promptAddAll:
		m.enqueue(ts...)
		m.setStatus("Ajouté à la file : " + plural(len(ts), "piste", "pistes"))
		return nil
	case promptDelete:
		if err := m.store.Delete(arg); err != nil {
			m.setError(err.Error())
		} else {
			m.setStatus("Playlist supprimée : " + arg)
		}
		m.plCol = 0
		m.syncPlaylists()
		return nil
	}
	if name == "" {
		return nil
	}
	if name == recentName && (kind == promptAdd || kind == promptNew || kind == promptRename) {
		m.setError("« Récents » est un nom réservé")
		return nil
	}
	switch kind {
	case promptTagArtist:
		album := ""
		for _, t := range m.lib.Tracks() {
			if t.ID == arg {
				album = t.Album
			}
		}
		return m.openPrompt(promptTagAlbum, "Album", album, arg+"\n"+name, nil)
	case promptTagAlbum:
		id, artist, _ := strings.Cut(arg, "\n")
		return m.tagsCmd(id, artist, name)
	case promptAdd:
		added := 0
		for _, t := range ts {
			ok, err := m.store.Add(name, t)
			if err != nil {
				m.setError(err.Error())
				break
			}
			if ok {
				added++
			}
		}
		if added > 0 {
			m.setStatus("Ajouté à « " + name + " » : " + plural(added, "piste", "pistes"))
		} else {
			m.setStatus("Déjà dans la playlist « " + name + " »")
		}
	case promptNew:
		if ok, err := m.store.Create(name); err != nil {
			m.setError(err.Error())
		} else if !ok {
			m.setError("La playlist « " + name + " » existe déjà")
		} else {
			m.setStatus("Playlist créée : " + name)
		}
	case promptRename:
		if ok, err := m.store.Rename(arg, name); err != nil {
			m.setError(err.Error())
		} else if !ok {
			m.setError("Impossible de renommer en « " + name + " »")
		} else {
			m.setStatus("Playlist renommée : " + name)
		}
	}
	m.syncPlaylists()
	for i, n := range m.plNames { // la playlist créée / renommée devient la sélection
		if n == name {
			m.plL.SetCursor(i)
			m.syncPlTracks(true)
		}
	}
	return nil
}

// tagsCmd réécrit artiste et album du fichier de la piste id, hors boucle UI.
func (m *Model) tagsCmd(id, artist, album string) tea.Cmd {
	lib := m.lib
	title := id
	for _, t := range lib.Tracks() {
		if t.ID == id {
			title = t.Title
		}
	}
	m.setStatus("Écriture des tags…")
	return func() tea.Msg { return tagsDoneMsg{title: title, err: lib.WriteTags(id, artist, album)} }
}

// exportM3U écrit la playlist sélectionnée dans le dossier de musique.
func (m *Model) exportM3U() tea.Cmd {
	name := m.plName()
	if name == "" || len(m.plData) == 0 {
		m.setStatus("Export : playlist vide")
		return nil
	}
	path := filepath.Join(m.cfg.MusicDir, playlist.FileName(name)+".m3u")
	if err := os.WriteFile(path, []byte(playlist.M3U(name, m.plData, m.lib.Path)), 0o644); err != nil {
		m.setError("Export impossible : " + err.Error())
		return nil
	}
	m.setStatus("Playlist exportée : " + path)
	return nil
}
