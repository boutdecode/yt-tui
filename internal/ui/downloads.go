package ui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kevin/yt-tui/internal/yt"
)

type dlState int

const (
	dlQueued dlState = iota
	dlRunning
	dlDone
	dlFailed
)

type dlRow struct {
	id    int
	track yt.Track
	state dlState
	pct   float64
	err   error
}

func (d dlState) String() string {
	return txt([...]string{"en attente", "téléchargement", "terminé", "échec"}[d])
}

// dlManager lance les téléchargements (2 en parallèle max) et relaie leur
// progression dans la boucle Bubble Tea via un canal.
type dlManager struct {
	ctx    context.Context
	dir    string
	format string
	ch     chan tea.Msg
	sem    chan struct{}
}

func newDLManager(ctx context.Context, dir, format string) *dlManager {
	return &dlManager{ctx: ctx, dir: dir, format: format,
		ch: make(chan tea.Msg, 128), sem: make(chan struct{}, 2)}
}

// wait attend le prochain message ; à ré-armer après chaque réception.
func (d *dlManager) wait() tea.Cmd {
	return func() tea.Msg {
		select {
		case m := <-d.ch:
			return m
		case <-d.ctx.Done():
			return nil
		}
	}
}

func (d *dlManager) post(m tea.Msg) {
	select {
	case d.ch <- m:
	case <-d.ctx.Done():
	}
}

func (d *dlManager) start(id int, t yt.Track) {
	go func() {
		select {
		case d.sem <- struct{}{}:
			defer func() { <-d.sem }()
		case <-d.ctx.Done():
			return
		}
		d.post(dlUpdateMsg{id: id, pct: 0})
		path, err := yt.Download(d.ctx, d.dir, d.format, t, func(p float64) {
			select { // la progression peut être abandonnée si l'UI est en retard
			case d.ch <- dlUpdateMsg{id: id, pct: p}:
			default:
			}
		})
		d.post(dlDoneMsg{id: id, path: path, err: err})
	}()
}

func (m Model) activeDownloads() int {
	n := 0
	for _, r := range m.downloads {
		if r.state == dlQueued || r.state == dlRunning {
			n++
		}
	}
	return n
}

// downloadsBody rend les lignes (2 lignes + 1 d'espace par téléchargement) dans w×h.
func (m Model) downloadsBody(w, h int) string {
	if len(m.downloads) == 0 {
		return mutedStyle.Render(txt("  Aucun téléchargement. Appuyez sur D sur un morceau."))
	}
	const rowH = 3
	vis := max(1, h/rowH)
	start := max(0, min(m.dlCursor-vis/2, len(m.downloads)-vis))
	p := m.prog
	p.Width = max(10, min(40, w-20))
	var b strings.Builder
	for i := start; i < min(len(m.downloads), start+vis); i++ {
		r := m.downloads[i]
		sel := i == m.dlCursor
		var icon string
		switch r.state {
		case dlQueued:
			icon = mutedStyle.Render("◌")
		case dlRunning:
			icon = pinkText.Render("⬇")
		case dlDone:
			icon = okStyle.Render("✔")
		case dlFailed:
			icon = errStyle.Render("✘")
		}
		line1 := spaces(1, sel) + icon + spaces(1, sel) + seg(sel, pickFg(sel, cText, cSelFg), sel, r.track.Title)
		var line2 string
		if r.state == dlFailed && r.err != nil {
			line2 = spaces(3, sel) + errStyle.Render(r.err.Error())
		} else {
			line2 = spaces(3, sel) + p.ViewAs(r.pct/100) + spaces(1, sel) +
				seg(sel, pickFg(sel, cMuted, cMutedSel), false, fmt.Sprintf("%3.0f%%  %s", r.pct, r.state))
		}
		b.WriteString(fit(line1, w, sel) + "\n" + fit(line2, w, sel) + "\n\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
