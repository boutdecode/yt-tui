// Package player pilote un processus mpv via son socket IPC JSON.
package player

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// EventKind identifie le type d'un événement du lecteur.
type EventKind int

const (
	EvTime     EventKind = iota // Value = position en secondes
	EvDuration                  // Value = durée en secondes
	EvPause                     // Bool = en pause
	EvVolume                    // Value = volume 0..100+
	EvEnded                     // fin naturelle du morceau
	EvError                     // échec de lecture, Err = message
)

// Event est un changement d'état émis par mpv.
type Event struct {
	Kind  EventKind
	Value float64
	Bool  bool
	Err   string
}

// Player est un client mpv.
type Player struct {
	cmd    *exec.Cmd
	conn   net.Conn
	dir    string
	wmu    sync.Mutex
	events chan Event
	done   chan struct{}
	once   sync.Once

	smu     sync.Mutex // protège la diffusion en cours
	stream  context.CancelFunc
	streamN int
}

// Start lance mpv en mode idle sans fenêtre et se connecte à son socket.
// ytdlConf (optionnel) est un fichier de config yt-dlp utilisé pour le streaming ;
// mpvLog (optionnel) reçoit les logs détaillés de mpv.
func Start(ytdlConf, mpvLog string) (*Player, error) {
	bin, err := exec.LookPath("mpv")
	if err != nil {
		return nil, errors.New("mpv introuvable dans le PATH")
	}
	dir, err := os.MkdirTemp("", "yt-tui-")
	if err != nil {
		return nil, err
	}
	sock := filepath.Join(dir, "mpv.sock")
	args := []string{"--no-video", "--idle=yes", "--no-terminal", "--force-window=no",
		"--ytdl-format=bestaudio/best", "--volume=80", "--input-ipc-server=" + sock}
	if ytdlConf != "" {
		args = append(args, "--ytdl-raw-options=config-locations="+ytdlConf)
	}
	if mpvLog != "" {
		args = append(args, "--log-file="+mpvLog, "--msg-level=all=info,ytdl_hook=v,ffmpeg=v")
	}
	cmd := exec.Command(bin, args...)
	if err := cmd.Start(); err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	go cmd.Wait() // évite le zombie
	var conn net.Conn
	for i := 0; i < 100; i++ {
		if conn, err = net.Dial("unix", sock); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		cmd.Process.Kill()
		os.RemoveAll(dir)
		return nil, fmt.Errorf("connexion au socket mpv: %w", err)
	}
	p := Attach(conn)
	p.cmd, p.dir = cmd, dir
	return p, nil
}

// Attach construit un Player sur une connexion IPC existante (utile aux tests).
func Attach(conn net.Conn) *Player {
	p := &Player{conn: conn, events: make(chan Event, 64), done: make(chan struct{})}
	go p.readLoop()
	for i, name := range []string{"time-pos", "duration", "pause", "volume"} {
		p.send("observe_property", i+1, name)
	}
	return p
}

// Events renvoie le canal d'événements ; il est fermé quand mpv s'arrête.
func (p *Player) Events() <-chan Event { return p.events }

// Load charge un fichier ou une URL et démarre la lecture.
func (p *Player) Load(target string) error {
	if err := p.send("loadfile", target, "replace"); err != nil {
		return err
	}
	return p.send("set_property", "pause", false)
}

// LoadStream lance `name args...` dont la sortie standard (un flux audio) est
// envoyée à mpv par un tube nommé. Sert à lire YouTube via yt-dlp : l'URL directe
// que yt-dlp résout est souvent refusée (403) à tout autre client que lui.
// Le flux n'est pas seekable.
func (p *Player) LoadStream(name string, args ...string) error {
	if p.dir == "" {
		return errors.New("diffusion indisponible sans répertoire temporaire")
	}
	p.smu.Lock()
	p.cancelStreamLocked()
	p.streamN++
	fifo := filepath.Join(p.dir, fmt.Sprintf("stream%d.fifo", p.streamN))
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		p.smu.Unlock()
		return err
	}
	// O_RDWR n'attend pas de lecteur ; on ferme à la fin du processus, ce qui donne l'EOF à mpv.
	f, err := os.OpenFile(fifo, os.O_RDWR, 0)
	if err != nil {
		p.smu.Unlock()
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.stream = cancel
	p.smu.Unlock()

	cmd := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = f, &stderr
	if err := cmd.Start(); err != nil {
		f.Close()
		cancel()
		return err
	}
	go func() {
		err := cmd.Wait()
		f.Close()
		if err != nil && ctx.Err() == nil {
			log.Printf("flux %s: %v\n--- stderr ---\n%s", name, err, stderr.String())
		}
		os.Remove(fifo)
	}()
	return p.Load(fifo)
}

func (p *Player) cancelStreamLocked() {
	if p.stream != nil {
		p.stream()
		p.stream = nil
	}
}

func (p *Player) TogglePause() error      { return p.send("cycle", "pause") }
func (p *Player) Seek(secs float64) error { return p.send("seek", secs, "relative") }

// SeekTo se place à une position absolue en secondes.
func (p *Player) SeekTo(secs float64) error { return p.send("seek", secs, "absolute") }
func (p *Player) SetVolume(v float64) error {
	return p.send("set_property", "volume", math.Max(0, math.Min(130, v)))
}
func (p *Player) Stop() error { return p.send("stop") }

// Close arrête mpv et nettoie le socket.
func (p *Player) Close() {
	p.once.Do(func() {
		close(p.done)
		p.smu.Lock()
		p.cancelStreamLocked()
		p.smu.Unlock()
		p.send("quit")
		p.conn.Close()
		if p.cmd != nil && p.cmd.Process != nil {
			p.cmd.Process.Kill()
		}
		if p.dir != "" {
			os.RemoveAll(p.dir)
		}
	})
}

func (p *Player) send(args ...any) error {
	b, err := json.Marshal(map[string]any{"command": args})
	if err != nil {
		return err
	}
	p.wmu.Lock()
	defer p.wmu.Unlock()
	p.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	_, err = p.conn.Write(append(b, '\n'))
	return err
}

func (p *Player) readLoop() {
	defer close(p.events)
	sc := bufio.NewScanner(p.conn)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	lastSec := -1
	for sc.Scan() {
		ev, ok := parseEvent(sc.Bytes())
		if !ok {
			continue
		}
		if ev.Kind == EvTime { // on ne remonte qu'un événement par seconde
			s := int(ev.Value)
			if s == lastSec {
				continue
			}
			lastSec = s
		}
		select {
		case p.events <- ev:
		case <-p.done:
			return
		}
	}
}

// parseEvent décode une ligne JSON de mpv ; ok=false si elle est sans intérêt.
func parseEvent(line []byte) (Event, bool) {
	var m struct {
		Event     string          `json:"event"`
		Name      string          `json:"name"`
		Data      json.RawMessage `json:"data"`
		Reason    string          `json:"reason"`
		FileError string          `json:"file_error"`
	}
	if json.Unmarshal(line, &m) != nil {
		return Event{}, false
	}
	switch m.Event {
	case "property-change":
		if len(m.Data) == 0 || string(m.Data) == "null" {
			return Event{}, false
		}
		switch m.Name {
		case "pause":
			var b bool
			if json.Unmarshal(m.Data, &b) != nil {
				return Event{}, false
			}
			return Event{Kind: EvPause, Bool: b}, true
		case "time-pos", "duration", "volume":
			var f float64
			if json.Unmarshal(m.Data, &f) != nil {
				return Event{}, false
			}
			kind := map[string]EventKind{"time-pos": EvTime, "duration": EvDuration, "volume": EvVolume}[m.Name]
			return Event{Kind: kind, Value: f}, true
		}
	case "end-file":
		switch m.Reason {
		case "eof":
			return Event{Kind: EvEnded}, true
		case "error":
			msg := m.FileError
			if msg == "" {
				msg = "lecture impossible"
			}
			return Event{Kind: EvError, Err: msg}, true
		}
	}
	return Event{}, false
}
