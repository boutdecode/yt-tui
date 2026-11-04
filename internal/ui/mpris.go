package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kevin/yt-tui/internal/mpris"
)

func secs(s float64) time.Duration { return time.Duration(s * float64(time.Second)) }

// mprisState traduit le modèle en état publié sur D-Bus.
func (m *Model) mprisState() mpris.State {
	st := mpris.State{Status: "Stopped", Volume: max(0, min(1, m.vol/100)), Shuffle: m.shuffle,
		Loop:    [...]string{"None", "Playlist", "Track"}[m.repeat],
		CanNext: m.qpos >= 0 && m.qpos+1 < len(m.queue), CanPrev: m.qpos > 0}
	if !m.hasCur {
		return st
	}
	st.HasTrack = true
	st.ID, st.Title, st.Artist, st.URL = m.cur.ID, m.cur.Title, m.cur.Channel, m.cur.URL()
	st.Length = secs(m.dur)
	if st.Length == 0 {
		st.Length = m.cur.Duration
	}
	st.Position = secs(m.pos)
	st.CanSeek = m.curLocal // un flux (tube) n'est pas seekable
	switch {
	case m.playing && m.paused:
		st.Status = "Paused"
	case m.playing:
		st.Status = "Playing"
	}
	if p, ok := m.thumbs.Path(m.cur.ID); ok {
		st.ArtURL = "file://" + p
	} else {
		st.ArtURL = "https://i.ytimg.com/vi/" + m.cur.ID + "/mqdefault.jpg"
	}
	return st
}

// handleRemote exécute une commande reçue d'un client MPRIS (widget, touches multimédia, playerctl).
func (m Model) handleRemote(c mpris.Command) (tea.Model, tea.Cmd) {
	switch c.Name {
	case "quit":
		return m, tea.Quit
	case "playpause":
		if m.playing {
			m.pl.TogglePause()
		} else {
			return m, m.resume()
		}
	case "play":
		if m.playing && m.paused {
			m.pl.TogglePause()
		} else if !m.playing {
			return m, m.resume()
		}
	case "pause":
		if m.playing && !m.paused {
			m.pl.TogglePause()
		}
	case "stop":
		m.pl.Stop()
		m.playing = false
	case "next":
		return m, m.goNext(false)
	case "previous":
		return m, m.goPrev()
	case "seek":
		m.pl.Seek(c.Arg)
	case "setpos":
		m.pl.SeekTo(c.Arg)
	case "volume":
		m.pl.SetVolume(c.Arg * 100)
	case "shuffle":
		m.shuffle = c.Arg > 0
		m.resetOrder()
	case "loop":
		m.repeat = map[string]repeatMode{"None": repeatOff, "Playlist": repeatQueue, "Track": repeatTrack}[c.Text]
	}
	return m, nil
}

// resume relance le morceau courant, ou le premier de la file, quand rien ne joue.
func (m *Model) resume() tea.Cmd {
	switch {
	case m.qpos >= 0 && m.qpos < len(m.queue):
		return m.startTrack(m.qpos, false)
	case len(m.queue) > 0:
		return m.startTrack(0, false)
	}
	return nil
}
