package ui

import (
	"sort"
	"strings"
	"time"

	"github.com/kevin/yt-tui/internal/yt"
)

func (m *Model) trackRows(ts []yt.Track, numbered bool) []row {
	rows := make([]row, len(ts))
	favs := map[string]bool{}
	for _, f := range m.store.Tracks(favName) {
		favs[f.ID] = true
	}
	for i, t := range ts {
		_, local := m.lib.Path(t.ID)
		r := trackRow{t: t, local: local, fav: favs[t.ID], thumbs: m.thumbs}
		if numbered {
			r.num = i + 1
		}
		rows[i] = r
	}
	return rows
}

func (m *Model) syncQueue() {
	rows := m.trackRows(m.queue, true)
	if m.hasCur && m.qpos >= 0 && m.qpos < len(rows) {
		r := rows[m.qpos].(trackRow)
		r.playing = true
		rows[m.qpos] = r
	}
	m.queueL.SetRows(rows)
}

// ---------- recherche ----------

func (m *Model) searchCount() int {
	if m.drill != nil {
		return len(m.drill.tracks)
	}
	n := 0
	if m.kind == kindAll || m.kind == kindVideos {
		n += len(m.resVideos)
	}
	if m.kind == kindAll || m.kind == kindPlaylists {
		n += len(m.resPlays)
	}
	if m.kind == kindAll || m.kind == kindChannels {
		n += len(m.resChans)
	}
	return n
}

func (m *Model) syncSearch() {
	var rows []row
	var refs []sref
	add := func(r row, ref sref) { rows, refs = append(rows, r), append(refs, ref) }

	if m.drill != nil {
		for i, r := range m.trackRows(m.drill.tracks, true) {
			add(r, sref{yt.KindVideo, i})
		}
	} else {
		if (m.kind == kindAll || m.kind == kindChannels) && len(m.resChans) > 0 {
			add(headerRow{"Chaînes"}, sref{})
			for i, e := range m.resChans {
				add(textRow{label: e.Title}, sref{yt.KindChannel, i})
			}
		}
		if (m.kind == kindAll || m.kind == kindPlaylists) && len(m.resPlays) > 0 {
			add(headerRow{"Playlists"}, sref{})
			for i, e := range m.resPlays {
				add(textRow{label: e.Title, sub: "· " + e.Channel}, sref{yt.KindPlaylist, i})
			}
		}
		if (m.kind == kindAll || m.kind == kindVideos) && len(m.resVideos) > 0 {
			add(headerRow{"Vidéos"}, sref{})
			for i, r := range m.trackRows(m.resVideos, false) {
				add(r, sref{yt.KindVideo, i})
			}
		}
	}
	m.searchRefs = refs
	m.searchL.SetRows(rows)
}

func (m *Model) searchRef() (sref, bool) {
	i, ok := m.searchL.Cursor()
	if !ok || i >= len(m.searchRefs) {
		return sref{}, false
	}
	return m.searchRefs[i], true
}

// searchTrack renvoie la vidéo sélectionnée dans l'onglet Recherche.
func (m *Model) searchTrack() (yt.Track, bool) {
	ref, ok := m.searchRef()
	if !ok || ref.kind != yt.KindVideo {
		return yt.Track{}, false
	}
	if m.drill != nil {
		if ref.idx < len(m.drill.tracks) {
			return m.drill.tracks[ref.idx], true
		}
		return yt.Track{}, false
	}
	if ref.idx < len(m.resVideos) {
		return m.resVideos[ref.idx], true
	}
	return yt.Track{}, false
}

// searchEntry renvoie la playlist/chaîne sélectionnée.
func (m *Model) searchEntry() (yt.Entry, bool) {
	ref, ok := m.searchRef()
	if !ok || m.drill != nil {
		return yt.Entry{}, false
	}
	switch ref.kind {
	case yt.KindPlaylist:
		if ref.idx < len(m.resPlays) {
			return m.resPlays[ref.idx], true
		}
	case yt.KindChannel:
		if ref.idx < len(m.resChans) {
			return m.resChans[ref.idx], true
		}
	}
	return yt.Entry{}, false
}

// ---------- bibliothèque ----------

func matchFilter(t yt.Track, f string) bool {
	if f == "" {
		return true
	}
	h := strings.ToLower(t.Title + " " + t.Channel + " " + t.Album)
	return strings.Contains(h, strings.ToLower(f))
}

func (m *Model) syncLibrary() {
	m.libAll = m.libAll[:0]
	for _, t := range m.lib.Tracks() {
		if matchFilter(t, m.libFilter) {
			m.libAll = append(m.libAll, t)
		}
	}
	seen := map[string]bool{}
	m.libArtistN = m.libArtistN[:0]
	for _, t := range m.libAll {
		if !seen[t.Channel] {
			seen[t.Channel] = true
			m.libArtistN = append(m.libArtistN, t.Channel)
		}
	}
	sort.Slice(m.libArtistN, func(i, j int) bool {
		return strings.ToLower(m.libArtistN[i]) < strings.ToLower(m.libArtistN[j])
	})
	rows := make([]row, len(m.libArtistN))
	for i, a := range m.libArtistN {
		rows[i] = textRow{label: a}
	}
	m.libArtists.SetRows(rows)
	m.syncLibAlbums(false)
}

func (m *Model) libArtist() string {
	if i, ok := m.libArtists.Cursor(); ok && i < len(m.libArtistN) {
		return m.libArtistN[i]
	}
	return ""
}

func (m *Model) libAlbum() string {
	if i, ok := m.libAlbums.Cursor(); ok && i < len(m.libAlbumN) {
		return m.libAlbumN[i]
	}
	return ""
}

// syncLibAlbums reconstruit la colonne Albums de l'artiste sélectionné.
func (m *Model) syncLibAlbums(resetCursor bool) {
	artist := m.libArtist()
	years := map[string]int{}
	m.libAlbumN = m.libAlbumN[:0]
	for _, t := range m.libAll {
		if t.Channel != artist {
			continue
		}
		if _, ok := years[t.Album]; !ok {
			m.libAlbumN = append(m.libAlbumN, t.Album)
		}
		years[t.Album] = max(years[t.Album], t.Year)
	}
	sort.Slice(m.libAlbumN, func(i, j int) bool {
		a, b := m.libAlbumN[i], m.libAlbumN[j]
		if (a == "Singles") != (b == "Singles") { // « Singles » en dernier
			return b == "Singles"
		}
		return strings.ToLower(a) < strings.ToLower(b)
	})
	rows := make([]row, len(m.libAlbumN))
	for i, a := range m.libAlbumN {
		sub := ""
		if y := years[a]; y > 0 {
			sub = itoa(y)
		}
		rows[i] = textRow{label: a, sub: sub}
	}
	m.libAlbums.SetRows(rows)
	if resetCursor {
		m.libAlbums.SetCursor(0)
	}
	m.syncLibTracks(resetCursor)
}

func (m *Model) syncLibTracks(resetCursor bool) {
	artist, album := m.libArtist(), m.libAlbum()
	m.libTrackD = m.libTrackD[:0]
	for _, t := range m.libAll {
		if t.Channel == artist && t.Album == album {
			m.libTrackD = append(m.libTrackD, t)
		}
	}
	m.libTracks.SetRows(m.trackRows(m.libTrackD, true))
	if resetCursor {
		m.libTracks.SetCursor(0)
	}
}

// libSelection renvoie les pistes désignées par la colonne focalisée de la bibliothèque.
func (m *Model) libSelection() []yt.Track {
	switch m.libCol {
	case 0:
		var ts []yt.Track
		for _, t := range m.libAll {
			if t.Channel == m.libArtist() {
				ts = append(ts, t)
			}
		}
		return ts
	case 1:
		return append([]yt.Track(nil), m.libTrackD...)
	}
	if i, ok := m.libTracks.Cursor(); ok && i < len(m.libTrackD) {
		return []yt.Track{m.libTrackD[i]}
	}
	return nil
}

// ---------- playlists ----------

func (m *Model) syncPlaylists() {
	m.plNames = append([]string{recentName}, m.store.Names()...)
	rows := make([]row, len(m.plNames))
	for i, n := range m.plNames {
		rows[i] = textRow{label: n, sub: "· " + itoa(len(m.playlistTracks(n)))}
	}
	m.plL.SetRows(rows)
	m.syncPlTracks(false)
}

func (m *Model) plName() string {
	if i, ok := m.plL.Cursor(); ok && i < len(m.plNames) {
		return m.plNames[i]
	}
	return ""
}

// Playlists réservées : « Récents » est calculée (historique, lecture seule) ;
// « Favoris » est une playlist ordinaire alimentée par la touche f.
const (
	recentName = "Récents"
	favName    = "Favoris"
)

func (m *Model) playlistTracks(name string) []yt.Track {
	if name == recentName {
		return m.hist.Tracks()
	}
	return m.store.Tracks(name)
}

func (m *Model) syncPlTracks(resetCursor bool) {
	m.plData = m.playlistTracks(m.plName())
	m.plTracks.SetRows(m.trackRows(m.plData, true))
	if resetCursor {
		m.plTracks.SetCursor(0)
	}
}

// focusTrack renvoie l'unique piste désignée par le curseur (f, e, X) ; false si la
// sélection porte sur plusieurs pistes ou sur aucune.
func (m *Model) focusTrack() (yt.Track, bool) {
	switch m.tab {
	case tabQueue:
		if i, ok := m.queueL.Cursor(); ok && i < len(m.queue) {
			return m.queue[i], true
		}
	case tabSearch:
		return m.searchTrack()
	case tabLibrary:
		if m.libCol == 2 {
			if i, ok := m.libTracks.Cursor(); ok && i < len(m.libTrackD) {
				return m.libTrackD[i], true
			}
		}
	case tabPlaylists:
		if m.plCol == 1 {
			if i, ok := m.plTracks.Cursor(); ok && i < len(m.plData) {
				return m.plData[i], true
			}
		}
	}
	return yt.Track{}, false
}

// toggleFav ajoute la piste ciblée aux favoris, ou l'en retire.
func (m *Model) toggleFav() {
	t, ok := m.focusTrack()
	if !ok {
		m.setStatus("Favoris : placez le curseur sur une piste")
		return
	}
	for i, f := range m.store.Tracks(favName) {
		if f.ID == t.ID {
			if err := m.store.Remove(favName, i); err != nil {
				m.setError(err.Error())
				return
			}
			m.setStatus("♥ retiré des favoris : " + t.Title)
			m.keepCursors(m.syncAll)
			return
		}
	}
	if _, err := m.store.Add(favName, t); err != nil {
		m.setError(err.Error())
		return
	}
	m.setStatus("♥ ajouté aux favoris : " + t.Title)
	m.keepCursors(m.syncAll)
}

// keepCursors exécute sync en conservant la position des curseurs des listes principales.
func (m *Model) keepCursors(sync func()) {
	q, _ := m.queueL.Cursor()
	s, _ := m.searchL.Cursor()
	a, _ := m.libArtists.Cursor()
	al, _ := m.libAlbums.Cursor()
	lt, _ := m.libTracks.Cursor()
	pl, _ := m.plL.Cursor()
	pt, _ := m.plTracks.Cursor()
	sync()
	m.queueL.SetCursor(q)
	m.searchL.SetCursor(s)
	m.libArtists.SetCursor(a)
	m.libAlbums.SetCursor(al)
	m.libTracks.SetCursor(lt)
	m.plL.SetCursor(pl)
	m.plTracks.SetCursor(pt)
}

// ---------- sélection générique (touches a / D / p) ----------

// selectedTracks renvoie les pistes désignées par la sélection de l'onglet actif.
func (m *Model) selectedTracks() []yt.Track {
	switch m.tab {
	case tabQueue:
		if i, ok := m.queueL.Cursor(); ok && i < len(m.queue) {
			return []yt.Track{m.queue[i]}
		}
	case tabSearch:
		if t, ok := m.searchTrack(); ok {
			return []yt.Track{t}
		}
	case tabLibrary:
		return m.libSelection()
	case tabPlaylists:
		if m.plCol == 0 {
			return m.plData
		}
		if i, ok := m.plTracks.Cursor(); ok && i < len(m.plData) {
			return []yt.Track{m.plData[i]}
		}
	}
	return nil
}

func totalDuration(ts []yt.Track) time.Duration {
	var d time.Duration
	for _, t := range ts {
		d += t.Duration
	}
	return d
}
