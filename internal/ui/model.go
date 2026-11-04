// Package ui contient le TUI Bubble Tea.
package ui

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kevin/yt-tui/internal/config"
	"github.com/kevin/yt-tui/internal/history"
	"github.com/kevin/yt-tui/internal/library"
	"github.com/kevin/yt-tui/internal/mpris"
	"github.com/kevin/yt-tui/internal/notify"
	"github.com/kevin/yt-tui/internal/player"
	"github.com/kevin/yt-tui/internal/playlist"
	"github.com/kevin/yt-tui/internal/session"
	"github.com/kevin/yt-tui/internal/thumb"
	"github.com/kevin/yt-tui/internal/yt"
)

type tab int

const (
	tabQueue tab = iota
	tabSearch
	tabLibrary
	tabPlaylists
	tabDownloads
	numTabs
)

var tabNames = [numTabs]string{"File", "Recherche", "Bibliothèque", "Playlists", "Téléchargements"}

// Types de recherche (puces tout | vidéos | playlists | chaînes).
type searchKind int

const (
	kindAll searchKind = iota
	kindVideos
	kindPlaylists
	kindChannels
	numKinds
)

var kindNames = [numKinds]string{"tout", "vidéos", "playlists", "chaînes"}

// Modes de répétition.
type repeatMode int

const (
	repeatOff repeatMode = iota
	repeatQueue
	repeatTrack
)

type promptKind int

const (
	promptAdd promptKind = iota
	promptNew
	promptRename
	promptFilter
	promptDelete
	promptQuit
	promptTagArtist
	promptTagAlbum
	promptDeleteFile
	promptAddAll // confirmation : ajouter toute la collection ouverte à la file
)

// maxAddNoConfirm : au-delà, « A » demande confirmation avant d'ajouter une collection.
const maxAddNoConfirm = 10

// isConfirm : invite de confirmation (sans champ de saisie).
func (k promptKind) isConfirm() bool {
	return k == promptDelete || k == promptAddAll || k == promptQuit || k == promptDeleteFile
}

// sref relie une ligne de la liste de recherche à ses données.
type sref struct {
	kind yt.Kind
	idx  int
}

type preview struct {
	tracks  []yt.Track
	loading bool
	err     error
}

type drill struct {
	title  string
	tracks []yt.Track
}

// Model est le modèle racine.
type Model struct {
	ctx   context.Context
	cfg   config.Config
	pl    *player.Player
	lib   *library.Library
	store *playlist.Store
	hist  *history.History
	nt    *notify.Notifier // peut être nil (pas de notifications)
	// focus du terminal (rapporté par le terminal ; inconnu tant qu'aucun évènement n'est reçu)
	focusSeen, focused bool
	thumbs             *thumb.Store
	mp                 *mpris.Server // peut être nil (pas de bus D-Bus)
	dl                 *dlManager
	width              int // zone utile (terminal moins les marges)
	height             int
	termW              int // taille réelle du terminal
	termH              int
	tab                tab

	spin      spinner.Model
	prog      progress.Model
	themes    map[string]Theme // thèmes déjà calculés, par id de vidéo
	status    string
	statusErr bool

	// recherche
	input      textinput.Model
	searching  bool
	loading    bool
	kind       searchKind
	resVideos  []yt.Track
	resPlays   []yt.Entry
	resChans   []yt.Entry
	drill      *drill
	searchL    vlist
	searchRefs []sref
	previews   map[string]*preview

	// file d'attente
	queue   []yt.Track
	qpos    int // index du morceau courant dans queue, -1 si aucun
	queueL  vlist
	shuffle bool
	repeat  repeatMode
	history []int
	played  map[int]bool

	// lecture
	cur           yt.Track
	hasCur        bool
	curLocal      bool   // le morceau courant est lu depuis un fichier
	curFormat     string // format du fichier local courant
	pendingID     string // morceau à jouer dès la fin de son téléchargement (repli du streaming)
	playing       bool
	paused        bool
	pos, dur, vol float64
	resumeAt      float64   // position à rejoindre au prochain chargement (session restaurée)
	restored      bool      // le morceau courant vient de la session : pas encore lancé
	radio         bool      // à la fin de la file, enchaîne sur des suggestions YouTube
	radioBusy     bool      // une recherche de suggestions est en cours
	radioPlay     bool      // lancer la première suggestion dès qu'elle arrive
	sleepAt       time.Time // arrêt programmé de la lecture (zéro = aucun)
	sleepEOT      bool      // arrêt à la fin de la piste en cours
	now           func() time.Time
	saveTick      int // compte les secondes de lecture entre deux sauvegardes de session

	// bibliothèque
	libFilter  string
	libCol     int // 0 artistes, 1 albums, 2 pistes
	libArtists vlist
	libAlbums  vlist
	libTracks  vlist
	libAll     []yt.Track
	libArtistN []string
	libAlbumN  []string
	libTrackD  []yt.Track
	libBusy    bool

	// playlists
	plL      vlist
	plTracks vlist
	plNames  []string
	plCol    int // 0 liste, 1 pistes
	plData   []yt.Track

	// téléchargements
	downloads []*dlRow
	dlCursor  int

	// saisie en bas d'écran
	prompt      textinput.Model
	prompting   bool
	promptKind  promptKind
	promptArg   string
	promptTrack []yt.Track
	promptLabel string
}

// New construit le modèle racine.
func New(ctx context.Context, cfg config.Config, pl *player.Player, lib *library.Library, store *playlist.Store, thumbs *thumb.Store, mp *mpris.Server) Model {
	in := textinput.New()
	in.Placeholder = "Rechercher sur YouTube…"
	in.Prompt = ""
	in.PlaceholderStyle = mutedStyle
	in.TextStyle = lipgloss.NewStyle().Foreground(cSelFg)
	in.Cursor.Style = lipgloss.NewStyle().Background(cPink).Foreground(cBg)
	in.Focus()
	pr := textinput.New()
	pr.Prompt = ""
	pr.PlaceholderStyle = mutedStyle
	pr.TextStyle = lipgloss.NewStyle().Foreground(cSelFg)
	pr.Cursor.Style = in.Cursor.Style
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = pinkBold
	pg := newProgress()
	m := Model{
		ctx: ctx, cfg: cfg, pl: pl, lib: lib, store: store, hist: history.Open(cfg.HistoryPath()), thumbs: thumbs, mp: mp,
		dl:  newDLManager(ctx, cfg.MusicDir, cfg.Format),
		tab: tabSearch, input: in, searching: true, spin: sp, prog: pg, prompt: pr,
		now: time.Now, qpos: -1, vol: 80, previews: map[string]*preview{}, themes: map[string]Theme{}, played: map[int]bool{},
	}
	if !cfg.NoResume && cfg.SessionPath() != "" {
		m.restoreSession(session.Load(cfg.SessionPath()))
	}
	m.syncQueue()
	m.syncSearch()
	m.syncLibrary()
	m.syncPlaylists()
	return m
}

// SetNotifier active les notifications de bureau au changement de piste.
func (m *Model) SetNotifier(n *notify.Notifier) { m.nt = n }

// restoreSession recharge file, options et morceau courant, sans lancer la lecture.
func (m *Model) restoreSession(s session.State) {
	if len(s.Queue) == 0 {
		return
	}
	m.queue = s.Queue
	m.radio = s.Radio
	m.shuffle, m.repeat = s.Shuffle, repeatMode(max(0, min(2, s.Repeat)))
	if s.Volume > 0 {
		m.vol = s.Volume
	}
	if s.Pos >= 0 && s.Pos < len(m.queue) {
		m.qpos, m.cur, m.hasCur, m.restored = s.Pos, m.queue[s.Pos], true, true
		m.dur, m.pos, m.resumeAt = m.cur.Duration.Seconds(), s.At, s.At
		m.played[s.Pos] = true
		m.queueL.SetCursor(s.Pos)
	}
	m.setStatus("Session restaurée : " + plural(len(m.queue), "piste", "pistes") + " — espace pour reprendre")
}

// snapshot est l'état à persister.
func (m Model) snapshot() session.State {
	pos := -1
	if m.hasCur {
		pos = m.qpos
	}
	return session.State{Queue: m.queue, Pos: pos, At: m.pos, Shuffle: m.shuffle,
		Repeat: int(m.repeat), Volume: m.vol, Radio: m.radio}
}

// saveSession écrit la session ; sans effet avec --no-resume.
func (m Model) saveSession() {
	if m.cfg.NoResume || m.cfg.SessionPath() == "" {
		return
	}
	if err := session.Save(m.cfg.SessionPath(), m.snapshot()); err != nil {
		log.Printf("session: %v", err)
	}
}

// SaveSession permet à main d'écrire la session à l'arrêt du programme.
func (m Model) SaveSession() { m.saveSession() }

func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.dl.wait(), m.waitPlayer())
}

func (m Model) waitPlayer() tea.Cmd {
	ch := m.pl.Events()
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return playerClosedMsg{}
		}
		return ev
	}
}

func (m *Model) setStatus(s string) { m.status, m.statusErr = s, false }
func (m *Model) setError(s string)  { m.status, m.statusErr = s, true }

// Update traite le message puis recale la mise en page, précharge les images
// manquantes et publie l'état du lecteur sur MPRIS.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	nm, cmd := m.update(msg)
	mm, ok := nm.(Model)
	if !ok {
		return nm, cmd
	}
	mm.layout()
	cmd = tea.Batch(cmd, mm.wantThumbs(), mm.wantPreview())
	mm.mp.Update(mm.mprisState())
	return mm, cmd
}

func (m Model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(tea.KeyMsg); ok && m.focusSeen {
		m.focused = true // une touche prouve que la fenêtre a le focus
	}
	switch msg := msg.(type) {
	case tea.FocusMsg:
		m.focusSeen, m.focused = true, true
	case tea.BlurMsg:
		m.focusSeen, m.focused = true, false
	case mpris.Command:
		return m.handleRemote(msg)
	case tea.WindowSizeMsg:
		m.termW, m.termH = msg.Width, msg.Height
		mx, my := margins(msg.Width, msg.Height)
		m.width, m.height = msg.Width-2*mx, msg.Height-2*my
	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
	case searchResultMsg:
		m.loading = false
		if msg.err != nil {
			m.setError("Recherche : " + msg.err.Error())
			break
		}
		m.resVideos, m.resPlays, m.resChans, m.drill = msg.videos, msg.playlists, msg.channels, nil
		m.syncSearch()
		m.searchL.SetCursor(0)
		m.setStatus(plural(m.searchCount(), "résultat", "résultats"))
	case previewMsg:
		if p := m.previews[msg.key]; p != nil {
			p.loading, p.tracks, p.err = false, msg.tracks, msg.err
		}
	case collectionMsg:
		m.loading = false
		if msg.err != nil {
			m.setError("Ouverture : " + msg.err.Error())
			break
		}
		m.drill = &drill{title: msg.entry.Title, tracks: msg.tracks}
		m.syncSearch()
		m.searchL.SetCursor(0)
		m.setStatus(plural(len(msg.tracks), "vidéo", "vidéos") + " · esc pour revenir aux résultats")
	case libRefreshedMsg:
		m.libBusy = false
		if msg.err != nil {
			m.setError("Analyse : " + msg.err.Error())
			break
		}
		m.syncLibrary()
		m.syncQueue()
		m.setStatus("Bibliothèque analysée : " + plural(len(m.lib.Tracks()), "piste", "pistes"))
	case radioMsg:
		m.radioBusy = false
		play := m.radioPlay
		m.radioPlay = false
		if msg.err != nil {
			m.setError("Radio : " + msg.err.Error())
			break
		}
		ids := make([]string, len(m.queue))
		for i, t := range m.queue {
			ids[i] = t.ID
		}
		fresh := yt.ExcludeIDs(msg.tracks, ids...)
		if len(fresh) == 0 {
			m.setStatus("Radio : aucune suggestion de plus")
			break
		}
		first := len(m.queue)
		m.enqueue(fresh...)
		m.setStatus("Radio : " + plural(len(fresh), "piste ajoutée", "pistes ajoutées"))
		if play {
			return m, m.startTrack(first, true)
		}
	case tagsDoneMsg:
		if msg.err != nil {
			m.setError("Tags : " + msg.err.Error())
		} else {
			m.setStatus("Tags mis à jour : " + msg.title)
		}
		m.syncAll()
	case themeMsg:
		if m.hasCur && msg.id == m.cur.ID {
			m.themes[msg.id] = msg.theme
			m.setTheme(msg.theme)
		}
	case thumbMsg:
		if msg.err != nil {
			log.Printf("image %s: %v", msg.id, msg.err)
		}
	case player.Event:
		cmd := m.onPlayer(msg)
		return m, tea.Batch(cmd, m.waitPlayer())
	case playerClosedMsg:
		m.playing = false
		m.setError("mpv s'est arrêté ; relancez yt-tui")
	case dlUpdateMsg:
		if msg.id < len(m.downloads) {
			r := m.downloads[msg.id]
			r.state, r.pct = dlRunning, msg.pct
		}
		return m, m.dl.wait()
	case dlDoneMsg:
		m.onDownloadDone(msg)
		return m, m.dl.wait()
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) onDownloadDone(msg dlDoneMsg) {
	if msg.id >= len(m.downloads) {
		return
	}
	r := m.downloads[msg.id]
	if msg.err != nil {
		r.state, r.err = dlFailed, msg.err
		m.setError(r.track.Title + " : " + msg.err.Error())
		if m.pendingID == r.track.ID {
			m.pendingID = ""
		}
		return
	}
	r.state, r.pct = dlDone, 100
	m.lib.Refresh()
	m.syncAll()
	m.setStatus("Téléchargé : " + r.track.Title)
	if m.pendingID == r.track.ID {
		m.pendingID = ""
		if m.qpos >= 0 && m.qpos < len(m.queue) && m.queue[m.qpos].ID == r.track.ID {
			m.startTrack(m.qpos, false)
		}
	}
}

// syncAll reconstruit toutes les listes (après un changement de la bibliothèque).
func (m *Model) syncAll() {
	m.syncQueue()
	m.syncSearch()
	m.syncLibrary()
	m.syncPlaylists()
}

func (m *Model) refreshLibrary() tea.Cmd {
	if m.libBusy {
		return nil
	}
	m.libBusy = true
	m.setStatus("Analyse du dossier…")
	lib := m.lib
	return func() tea.Msg { return libRefreshedMsg{err: lib.Refresh()} }
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return itoa(n) + " " + many
}

// wantThumbs lance le chargement des images manquantes autour des curseurs.
func (m *Model) wantThumbs() tea.Cmd {
	if m.thumbs == nil {
		return nil
	}
	store, ctx := m.thumbs, m.ctx
	var cmds []tea.Cmd
	small := func(v *vlist) {
		for _, r := range v.Near(6, 18) {
			if tr, ok := r.(trackRow); ok && store.Claim(tr.t.ID) {
				id := tr.t.ID
				cmds = append(cmds, func() tea.Msg { return thumbMsg{id: id, err: store.Load(ctx, id)} })
			}
		}
	}
	large := func(id string) {
		if id != "" && store.ClaimLarge(id) {
			cmds = append(cmds, func() tea.Msg { return thumbMsg{id: id, err: store.LoadLarge(ctx, id)} })
		}
	}
	switch m.tab {
	case tabQueue:
		small(&m.queueL)
		if m.hasCur {
			large(m.cur.ID)
		}
	case tabSearch:
		small(&m.searchL)
		if t, ok := m.searchTrack(); ok {
			large(t.ID)
		}
	case tabLibrary:
		small(&m.libTracks)
	case tabPlaylists:
		small(&m.plTracks)
	}
	return tea.Batch(cmds...)
}

// wantPreview charge les premières vidéos d'une playlist/chaîne sélectionnée.
func (m *Model) wantPreview() tea.Cmd {
	if m.tab != tabSearch || m.drill != nil {
		return nil
	}
	e, ok := m.searchEntry()
	if !ok {
		return nil
	}
	key := entryKey(e)
	if _, seen := m.previews[key]; seen || e.URL() == "" {
		return nil
	}
	m.previews[key] = &preview{loading: true}
	ctx := m.ctx
	return func() tea.Msg {
		c, cancel := context.WithTimeout(ctx, 40*time.Second)
		defer cancel()
		ts, err := yt.ListEntries(c, e, 8)
		return previewMsg{key: key, tracks: ts, err: err}
	}
}

func entryKey(e yt.Entry) string { return itoa(int(e.Kind)) + ":" + e.ID }

// searchCmd lance la recherche selon le type choisi (« tout » = 3 requêtes en parallèle).
func (m Model) searchCmd(q string) tea.Cmd {
	ctx, n, kind := m.ctx, m.cfg.Results, m.kind
	return func() tea.Msg {
		c, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		var res searchResultMsg
		var wg sync.WaitGroup
		var errV, errP, errC error
		if kind == kindAll || kind == kindVideos {
			wg.Add(1)
			go func() { defer wg.Done(); res.videos, errV = yt.Search(c, q, n) }()
		}
		if kind == kindAll || kind == kindPlaylists {
			wg.Add(1)
			go func() {
				defer wg.Done()
				res.playlists, errP = yt.SearchKind(c, q, yt.KindPlaylist, pick(kind == kindAll, 5, n))
			}()
		}
		if kind == kindAll || kind == kindChannels {
			wg.Add(1)
			go func() {
				defer wg.Done()
				res.channels, errC = yt.SearchKind(c, q, yt.KindChannel, pick(kind == kindAll, 4, n))
			}()
		}
		wg.Wait()
		// en mode « tout », seule l'erreur des vidéos est bloquante
		switch {
		case errV != nil:
			res.err = errV
		case kind != kindAll && (errP != nil || errC != nil):
			res.err = firstErr(errP, errC)
		case errP != nil || errC != nil:
			log.Printf("recherche partielle: playlists=%v chaînes=%v", errP, errC)
		}
		return res
	}
}

func pick(cond bool, a, b int) int {
	if cond {
		return a
	}
	return b
}

func firstErr(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}

func (m Model) openCmd(e yt.Entry) tea.Cmd {
	ctx := m.ctx
	return func() tea.Msg {
		c, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		ts, err := yt.ListEntries(c, e, 50)
		return collectionMsg{entry: e, tracks: ts, err: err}
	}
}

// radioCmd cherche des suggestions à partir du dernier morceau de la file.
func (m *Model) radioCmd() tea.Cmd {
	if !m.radio || m.radioBusy || len(m.queue) == 0 {
		return nil
	}
	last := m.queue[len(m.queue)-1]
	seed := last.ID
	m.radioBusy = true
	ctx := m.ctx
	return func() tea.Msg {
		ts, err := yt.Related(ctx, seed, 15)
		if err == nil && len(ts) == 0 && last.Channel != "" {
			// Pas de mix YouTube pour cette vidéo : on retombe sur d'autres titres de la chaîne.
			ts, err = yt.Search(ctx, last.Channel, 15)
		}
		return radioMsg{seed: seed, tracks: ts, err: err}
	}
}

// Minuteur de sommeil : off → 15 → 30 → 60 min → fin de piste → off.
var sleepSteps = []time.Duration{15 * time.Minute, 30 * time.Minute, 60 * time.Minute}

func (m *Model) cycleSleep() {
	switch {
	case m.sleepEOT:
		m.sleepEOT, m.sleepAt = false, time.Time{}
		m.setStatus("Minuteur : off")
	case m.sleepAt.IsZero():
		m.sleepAt = m.now().Add(sleepSteps[0])
		m.setStatus("Minuteur : arrêt dans 15 min")
	default:
		left := m.sleepAt.Sub(m.now())
		for _, d := range sleepSteps {
			if left < d-time.Minute { // palier suivant : on ne revient jamais en arrière
				m.sleepAt = m.now().Add(d)
				m.setStatus("Minuteur : arrêt dans " + itoa(int(d.Minutes())) + " min")
				return
			}
		}
		m.sleepAt, m.sleepEOT = time.Time{}, true
		m.setStatus("Minuteur : arrêt à la fin de la piste")
	}
}

// sleepLabel est le texte du minuteur pour la barre de lecture (vide si inactif).
func (m Model) sleepLabel() string {
	switch {
	case m.sleepEOT:
		return "⏾ fin de piste"
	case !m.sleepAt.IsZero():
		left := max(0, int((m.sleepAt.Sub(m.now())+59*time.Second)/time.Minute))
		return "⏾ " + itoa(left) + " min"
	}
	return ""
}

// checkSleep met la lecture en pause quand le minuteur arrive à échéance.
func (m *Model) checkSleep() {
	if m.sleepAt.IsZero() || m.now().Before(m.sleepAt) {
		return
	}
	m.sleepAt = time.Time{}
	if m.playing && !m.paused {
		m.pl.TogglePause()
	}
	m.setStatus("Minuteur écoulé : lecture mise en pause")
}

// setTheme applique t à toute l'interface (styles, barre de progression, curseurs de saisie).
func (m *Model) setTheme(t Theme) {
	if t == curTheme {
		return
	}
	applyTheme(t)
	m.prog = newProgress()
	m.spin.Style = pinkBold
	cur := lipgloss.NewStyle().Background(cPink).Foreground(cBg)
	m.input.Cursor.Style, m.prompt.Cursor.Style = cur, cur
}

// themeCmd calcule (hors boucle UI) le thème de la pochette de id.
func (m *Model) themeCmd(id string) tea.Cmd {
	if m.cfg.StaticTheme {
		return nil
	}
	if t, ok := m.themes[id]; ok {
		return func() tea.Msg { return themeMsg{id: id, theme: t} }
	}
	store, ctx := m.thumbs, m.ctx
	return func() tea.Msg {
		c, ok, err := store.Palette(ctx, id)
		if err != nil || !ok {
			return themeMsg{id: id, theme: defaultTheme()}
		}
		return themeMsg{id: id, theme: themeFrom(c)}
	}
}
