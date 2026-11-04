package ui

import (
	"log"
	"math/rand/v2"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kevin/yt-tui/internal/player"
	"github.com/kevin/yt-tui/internal/yt"
)

// startTrack joue queue[i] ; pushHistory mémorise le morceau quitté (pour « précédent » en aléatoire).
func (m *Model) startTrack(i int, pushHistory bool) tea.Cmd {
	if i < 0 || i >= len(m.queue) {
		return nil
	}
	t := m.queue[i]
	target, local := t.URL(), false
	if p, ok := m.lib.Path(t.ID); ok {
		target, local = p, true
	}
	var err error
	if local {
		err = m.pl.Load(target)
	} else {
		err = m.pl.LoadStream("yt-dlp", yt.StreamArgs(t)...)
	}
	if err != nil {
		m.setError("mpv : " + err.Error())
		return nil
	}
	if pushHistory && m.qpos >= 0 && m.qpos != i {
		m.history = append(m.history, m.qpos)
		if len(m.history) > 200 {
			m.history = m.history[1:]
		}
	}
	log.Printf("lecture: piste=%q local=%v", t.Title, local)
	m.qpos, m.cur, m.hasCur, m.curLocal, m.pendingID = i, t, true, local, ""
	m.restored, m.resumeAt = false, 0
	if err := m.hist.Add(t); err != nil {
		log.Printf("historique: %v", err)
	}
	m.syncPlaylists()
	if m.nt != nil && !m.focused { // fenêtre au premier plan : inutile de notifier
		icon, ok := m.thumbs.Path(t.ID)
		if !ok {
			icon = ""
		}
		m.nt.Show(t.Title, t.Channel, icon)
	}
	m.played[i] = true
	m.curFormat = ""
	if local {
		for _, lt := range m.lib.Tracks() {
			if lt.ID == t.ID {
				m.curFormat = lt.Format
				break
			}
		}
	}
	m.playing, m.paused, m.pos, m.dur = true, false, 0, t.Duration.Seconds()
	m.syncQueue()
	theme := tea.Batch(m.themeCmd(t.ID), m.prefetchRadio(i))
	if local {
		m.setStatus("Lecture (local) : " + t.Title)
		return theme
	}
	m.setStatus("Lecture (streaming) : " + t.Title)
	if m.cfg.AutoCache {
		m.enqueueDownload(t)
	}
	return theme
}

// prefetchRadio charge des suggestions quand le dernier morceau de la file démarre.
func (m *Model) prefetchRadio(i int) tea.Cmd {
	if i != len(m.queue)-1 {
		return nil
	}
	return m.radioCmd()
}

func (m *Model) playAt(i int) tea.Cmd { return m.startTrack(i, true) }

// resetOrder oublie l'historique d'aléatoire après une modification de la file.
func (m *Model) resetOrder() {
	m.history = nil
	m.played = map[int]bool{}
	if m.qpos >= 0 {
		m.played[m.qpos] = true
	}
}

// nextIndex calcule le morceau suivant selon aléatoire / répéter.
// auto : fin naturelle du morceau (la répétition d'une piste ne s'applique qu'alors).
func (m *Model) nextIndex(auto bool) (int, bool) {
	n := len(m.queue)
	if n == 0 {
		return 0, false
	}
	if auto && m.repeat == repeatTrack && m.qpos >= 0 {
		return m.qpos, true
	}
	if m.shuffle {
		var cands []int
		for i := 0; i < n; i++ {
			if !m.played[i] {
				cands = append(cands, i)
			}
		}
		if len(cands) == 0 {
			if m.repeat == repeatOff {
				return 0, false
			}
			m.played = map[int]bool{}
			if m.qpos >= 0 {
				m.played[m.qpos] = true
			}
			for i := 0; i < n; i++ {
				if i != m.qpos {
					cands = append(cands, i)
				}
			}
			if len(cands) == 0 {
				return m.qpos, true
			}
		}
		return cands[rand.IntN(len(cands))], true
	}
	if m.qpos+1 < n {
		return m.qpos + 1, true
	}
	if m.repeat != repeatOff {
		return 0, true
	}
	return 0, false
}

func (m *Model) goNext(auto bool) tea.Cmd {
	if i, ok := m.nextIndex(auto); ok {
		return m.startTrack(i, !(auto && i == m.qpos))
	}
	if auto {
		m.playing = false
		m.pos = 0
		if m.radio && m.hasCur { // file épuisée : la radio prend le relais
			m.radioPlay = true
			return m.radioCmd()
		}
	}
	return nil
}

func (m *Model) goPrev() tea.Cmd {
	if n := len(m.history); n > 0 {
		i := m.history[n-1]
		m.history = m.history[:n-1]
		if i < len(m.queue) {
			return m.startTrack(i, false)
		}
	}
	if m.qpos > 0 {
		return m.startTrack(m.qpos-1, false)
	}
	return nil
}

func (m *Model) enqueue(ts ...yt.Track) {
	m.queue = append(m.queue, ts...)
	m.resetOrder()
	m.syncQueue()
}

// removeFromQueue retire le morceau i ; le morceau en cours continue de jouer s'il est retiré.
func (m *Model) removeFromQueue(i int) {
	if i < 0 || i >= len(m.queue) {
		return
	}
	m.queue = append(m.queue[:i:i], m.queue[i+1:]...)
	switch {
	case i < m.qpos:
		m.qpos--
	case i == m.qpos:
		m.qpos = -1
	}
	m.resetOrder()
	m.syncQueue()
	m.queueL.SetCursor(min(i, len(m.queue)-1))
}

// moveInQueue échange le morceau i avec son voisin (delta = ±1) et renvoie le nouvel index.
func (m *Model) moveInQueue(i, delta int) int {
	j := i + delta
	if i < 0 || i >= len(m.queue) || j < 0 || j >= len(m.queue) {
		return i
	}
	m.queue[i], m.queue[j] = m.queue[j], m.queue[i]
	switch m.qpos {
	case i:
		m.qpos = j
	case j:
		m.qpos = i
	}
	m.resetOrder()
	m.syncQueue()
	m.queueL.SetCursor(j)
	return j
}

func (m *Model) enqueueDownload(t yt.Track) {
	if _, ok := m.lib.Path(t.ID); ok {
		m.setStatus("Déjà téléchargé : " + t.Title)
		return
	}
	for _, r := range m.downloads {
		if r.track.ID == t.ID && (r.state == dlQueued || r.state == dlRunning) {
			m.setStatus("Téléchargement déjà en cours : " + t.Title)
			return
		}
	}
	row := &dlRow{id: len(m.downloads), track: t}
	m.downloads = append(m.downloads, row)
	m.dl.start(row.id, t)
	m.setStatus("Téléchargement ajouté : " + t.Title)
}

func (m *Model) onPlayer(ev player.Event) tea.Cmd {
	if ev.Kind == player.EvEnded || ev.Kind == player.EvError {
		log.Printf("mpv: événement %d err=%q piste=%q (%s) local=%v", ev.Kind, ev.Err, m.cur.Title, m.cur.ID, m.curLocal)
	}
	switch ev.Kind {
	case player.EvTime:
		m.pos = ev.Value
		m.checkSleep()
		if m.saveTick++; m.saveTick >= 30 {
			m.saveTick = 0
			m.saveSession()
		}
	case player.EvDuration:
		m.dur = ev.Value
		if m.resumeAt > 1 { // reprise d'une session : rejoint la position sauvegardée
			m.pl.SeekTo(m.resumeAt)
		}
		m.resumeAt = 0
	case player.EvPause:
		m.paused = ev.Bool
	case player.EvVolume:
		m.vol = ev.Value
	case player.EvEnded:
		if m.sleepEOT {
			m.sleepEOT = false
			m.playing = false
			m.setStatus("Minuteur : lecture arrêtée à la fin de la piste")
			return nil
		}
		return m.goNext(true)
	case player.EvError:
		m.playing = false
		if m.hasCur && !m.curLocal {
			// YouTube refuse souvent le flux direct à mpv (403) alors que yt-dlp,
			// qui télécharge par morceaux, réussit : on télécharge puis on joue.
			m.pendingID = m.cur.ID
			m.enqueueDownload(m.cur)
			m.setStatus("Streaming refusé (" + ev.Err + ") — téléchargement avant lecture : " + m.cur.Title)
			return nil
		}
		m.setError("Lecture impossible : " + ev.Err)
	}
	return nil
}

func (m Model) repeatLabel() string {
	return [...]string{"off", "file", "piste"}[m.repeat]
}
