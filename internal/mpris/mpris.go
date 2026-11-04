// Package mpris expose le lecteur sur le bus D-Bus de session via la spécification
// MPRIS2 : le widget son de Cinnamon/GNOME/KDE, les touches multimédia et
// `playerctl` voient alors yt-tui comme un lecteur et peuvent le piloter.
package mpris

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"
)

const (
	busName    = "org.mpris.MediaPlayer2.yt-tui"
	objectPath = dbus.ObjectPath("/org/mpris/MediaPlayer2")
	ifaceRoot  = "org.mpris.MediaPlayer2"
	ifacePlay  = "org.mpris.MediaPlayer2.Player"
	noTrack    = dbus.ObjectPath("/org/mpris/MediaPlayer2/TrackList/NoTrack")
)

// State est l'état du lecteur tel que publié sur D-Bus.
type State struct {
	Status   string // "Playing", "Paused" ou "Stopped"
	HasTrack bool
	ID       string
	Title    string
	Artist   string
	URL      string
	ArtURL   string
	Length   time.Duration
	Position time.Duration
	Volume   float64 // 0..1
	CanNext  bool
	CanPrev  bool
	CanSeek  bool
	Shuffle  bool
	Loop     string // "None", "Track" ou "Playlist"
}

// Command est une demande venue d'un client MPRIS.
// Name : play, pause, playpause, stop, next, previous, seek (Arg = secondes relatives),
// setpos (Arg = secondes absolues), volume (Arg = 0..1), shuffle (Arg = 1 ou 0),
// loop (Text = "None", "Track" ou "Playlist"), quit.
type Command struct {
	Name string
	Arg  float64
	Text string
}

// TrackPath convertit un identifiant YouTube en chemin d'objet D-Bus valide.
func TrackPath(id string) dbus.ObjectPath {
	var b strings.Builder
	for _, r := range id {
		if r == '-' {
			b.WriteString("_2d") // '-' est interdit dans un chemin d'objet
		} else if r == '_' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return noTrack
	}
	return dbus.ObjectPath("/org/yttui/track/" + b.String())
}

// Metadata construit le dictionnaire xesam/mpris du morceau courant.
func Metadata(st State) map[string]dbus.Variant {
	if !st.HasTrack {
		return map[string]dbus.Variant{"mpris:trackid": dbus.MakeVariant(noTrack)}
	}
	md := map[string]dbus.Variant{
		"mpris:trackid": dbus.MakeVariant(TrackPath(st.ID)),
		"mpris:length":  dbus.MakeVariant(st.Length.Microseconds()),
		"xesam:title":   dbus.MakeVariant(st.Title),
		"xesam:artist":  dbus.MakeVariant([]string{st.Artist}),
		"xesam:url":     dbus.MakeVariant(st.URL),
	}
	if st.ArtURL != "" {
		md["mpris:artUrl"] = dbus.MakeVariant(st.ArtURL)
	}
	return md
}

// Server est le service D-Bus.
type Server struct {
	conn  *dbus.Conn
	props *prop.Properties
	send  func(Command)
	name  string

	mu     sync.Mutex
	last   State
	lastAt time.Time
	init   bool
}

// Start se connecte au bus de session et publie le lecteur. send reçoit les
// commandes des clients ; il est appelé depuis des goroutines D-Bus.
func Start(send func(Command)) (*Server, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("bus de session D-Bus indisponible: %w", err)
	}
	s := &Server{conn: conn, send: send}

	root, player := &rootObj{s}, &playerObj{s}
	if err := conn.Export(root, objectPath, ifaceRoot); err != nil {
		conn.Close()
		return nil, err
	}
	if err := conn.ExportWithMap(player, map[string]string{"SeekBy": "Seek"}, objectPath, ifacePlay); err != nil {
		conn.Close()
		return nil, err
	}
	st := State{Status: "Stopped", Loop: "None"}
	spec := map[string]map[string]*prop.Prop{
		ifaceRoot: {
			"CanQuit":             {Value: true, Emit: prop.EmitConst},
			"CanRaise":            {Value: false, Emit: prop.EmitConst},
			"HasTrackList":        {Value: false, Emit: prop.EmitConst},
			"Identity":            {Value: "yt-tui", Emit: prop.EmitConst},
			"SupportedUriSchemes": {Value: []string{}, Emit: prop.EmitConst},
			"SupportedMimeTypes":  {Value: []string{}, Emit: prop.EmitConst},
		},
		ifacePlay: {
			"PlaybackStatus": {Value: st.Status, Emit: prop.EmitTrue},
			"LoopStatus": {Value: "None", Emit: prop.EmitTrue, Writable: true, Callback: func(c *prop.Change) *dbus.Error {
				if v, ok := c.Value.(string); ok && (v == "None" || v == "Track" || v == "Playlist") {
					send(Command{Name: "loop", Text: v})
					return nil
				}
				return dbus.MakeFailedError(errors.New("LoopStatus invalide"))
			}},
			"Rate":        {Value: 1.0, Emit: prop.EmitFalse},
			"MinimumRate": {Value: 1.0, Emit: prop.EmitConst},
			"MaximumRate": {Value: 1.0, Emit: prop.EmitConst},
			"Shuffle": {Value: false, Emit: prop.EmitTrue, Writable: true, Callback: func(c *prop.Change) *dbus.Error {
				if v, ok := c.Value.(bool); ok {
					a := 0.0
					if v {
						a = 1
					}
					send(Command{Name: "shuffle", Arg: a})
				}
				return nil
			}},
			"Metadata": {Value: Metadata(st), Emit: prop.EmitTrue},
			"Volume": {Value: 0.8, Emit: prop.EmitTrue, Writable: true, Callback: func(c *prop.Change) *dbus.Error {
				if v, ok := c.Value.(float64); ok {
					send(Command{Name: "volume", Arg: v})
				}
				return nil
			}},
			"Position":      {Value: int64(0), Emit: prop.EmitFalse}, // pas de signal : les clients l'interrogent
			"CanGoNext":     {Value: false, Emit: prop.EmitTrue},
			"CanGoPrevious": {Value: false, Emit: prop.EmitTrue},
			"CanPlay":       {Value: false, Emit: prop.EmitTrue},
			"CanPause":      {Value: false, Emit: prop.EmitTrue},
			"CanSeek":       {Value: false, Emit: prop.EmitTrue},
			"CanControl":    {Value: true, Emit: prop.EmitConst},
		},
	}
	props, err := prop.Export(conn, objectPath, spec)
	if err != nil {
		conn.Close()
		return nil, err
	}
	s.props = props
	playerMethods := introspect.Methods(player)
	for i := range playerMethods {
		if playerMethods[i].Name == "SeekBy" {
			playerMethods[i].Name = "Seek"
		}
	}
	node := &introspect.Node{
		Name: string(objectPath),
		Interfaces: []introspect.Interface{
			introspect.IntrospectData,
			prop.IntrospectData,
			{Name: ifaceRoot, Methods: introspect.Methods(root), Properties: props.Introspection(ifaceRoot)},
			{Name: ifacePlay, Methods: playerMethods, Properties: props.Introspection(ifacePlay),
				Signals: []introspect.Signal{{Name: "Seeked", Args: []introspect.Arg{{Name: "Position", Type: "x"}}}}},
		},
	}
	if err := conn.Export(introspect.NewIntrospectable(node), objectPath, "org.freedesktop.DBus.Introspectable"); err != nil {
		conn.Close()
		return nil, err
	}

	name := busName
	reply, err := conn.RequestName(name, dbus.NameFlagDoNotQueue)
	if err == nil && reply != dbus.RequestNameReplyPrimaryOwner {
		// une autre instance de yt-tui tourne déjà : suffixe d'instance prévu par la spec
		name = fmt.Sprintf("%s.instance%d", busName, os.Getpid())
		reply, err = conn.RequestName(name, dbus.NameFlagDoNotQueue)
	}
	if err != nil {
		conn.Close()
		return nil, err
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		conn.Close()
		return nil, errors.New("nom D-Bus " + name + " indisponible")
	}
	s.name = name
	return s, nil
}

// Name renvoie le nom D-Bus obtenu.
func (s *Server) Name() string { return s.name }

// Close libère le nom et ferme la connexion.
func (s *Server) Close() {
	if s == nil {
		return
	}
	s.conn.ReleaseName(s.name)
	s.conn.Close()
}

// Update publie le nouvel état ; seuls les champs modifiés émettent PropertiesChanged.
func (s *Server) Update(st State) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.last
	first := !s.init
	now := time.Now()
	s.last, s.init = st, true

	set := func(name string, v any) { s.props.SetMust(ifacePlay, name, v) }
	if first || st.Status != old.Status {
		set("PlaybackStatus", st.Status)
	}
	if first || st.ID != old.ID || st.HasTrack != old.HasTrack || st.Title != old.Title ||
		st.Artist != old.Artist || st.ArtURL != old.ArtURL || st.Length != old.Length || st.URL != old.URL {
		set("Metadata", Metadata(st))
	}
	if first || st.Volume != old.Volume {
		set("Volume", st.Volume)
	}
	if first || st.CanNext != old.CanNext {
		set("CanGoNext", st.CanNext)
	}
	if first || st.CanPrev != old.CanPrev {
		set("CanGoPrevious", st.CanPrev)
	}
	if first || st.HasTrack != old.HasTrack {
		set("CanPlay", st.HasTrack)
		set("CanPause", st.HasTrack)
	}
	if first || st.Shuffle != old.Shuffle {
		set("Shuffle", st.Shuffle)
	}
	if loop := st.Loop; (first || loop != old.Loop) && loop != "" {
		set("LoopStatus", loop)
	}
	if first || st.CanSeek != old.CanSeek {
		set("CanSeek", st.CanSeek)
	}
	if first || st.Position != old.Position {
		set("Position", st.Position.Microseconds())
		// saut par rapport à la position attendue sur le même morceau (seek) : signal Seeked
		// (lastAt = instant de la dernière position publiée, pas de la dernière mise à jour)
		expected := old.Position
		if old.Status == "Playing" {
			expected += now.Sub(s.lastAt)
		}
		if !first && st.ID == old.ID && st.Status == old.Status && abs(st.Position-expected) > 2*time.Second {
			s.conn.Emit(objectPath, ifacePlay+".Seeked", st.Position.Microseconds())
		}
		s.lastAt = now
	} else if st.Status != old.Status {
		s.lastAt = now // nouvelle base de temps à chaque changement d'état
	}
}

func abs(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

type rootObj struct{ s *Server }

func (r *rootObj) Raise() *dbus.Error { return nil }
func (r *rootObj) Quit() *dbus.Error {
	r.s.send(Command{Name: "quit"})
	return nil
}

type playerObj struct{ s *Server }

func (p *playerObj) Play() *dbus.Error      { p.s.send(Command{Name: "play"}); return nil }
func (p *playerObj) Pause() *dbus.Error     { p.s.send(Command{Name: "pause"}); return nil }
func (p *playerObj) PlayPause() *dbus.Error { p.s.send(Command{Name: "playpause"}); return nil }
func (p *playerObj) Stop() *dbus.Error      { p.s.send(Command{Name: "stop"}); return nil }
func (p *playerObj) Next() *dbus.Error      { p.s.send(Command{Name: "next"}); return nil }
func (p *playerObj) Previous() *dbus.Error  { p.s.send(Command{Name: "previous"}); return nil }

// SeekBy (exporté sous le nom D-Bus « Seek » ; go vet réserve Seek aux io.Seeker) reçoit un décalage relatif en microsecondes.
func (p *playerObj) SeekBy(offset int64) *dbus.Error {
	p.s.send(Command{Name: "seek", Arg: float64(offset) / 1e6})
	return nil
}

// SetPosition reçoit une position absolue en microsecondes pour le morceau courant.
func (p *playerObj) SetPosition(track dbus.ObjectPath, pos int64) *dbus.Error {
	p.s.mu.Lock()
	cur := TrackPath(p.s.last.ID)
	p.s.mu.Unlock()
	if track != cur { // morceau périmé : la spec demande d'ignorer
		return nil
	}
	p.s.send(Command{Name: "setpos", Arg: float64(pos) / 1e6})
	return nil
}

func (p *playerObj) OpenUri(uri string) *dbus.Error {
	return dbus.MakeFailedError(errors.New("OpenUri n'est pas supporté"))
}
