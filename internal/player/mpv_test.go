package player

import (
	"bufio"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseEvent(t *testing.T) {
	cases := []struct {
		in   string
		want Event
		ok   bool
	}{
		{`{"event":"property-change","id":1,"name":"time-pos","data":12.5}`, Event{Kind: EvTime, Value: 12.5}, true},
		{`{"event":"property-change","id":3,"name":"pause","data":true}`, Event{Kind: EvPause, Bool: true}, true},
		{`{"event":"property-change","id":2,"name":"duration","data":null}`, Event{}, false},
		{`{"event":"end-file","reason":"eof"}`, Event{Kind: EvEnded}, true},
		{`{"event":"end-file","reason":"stop"}`, Event{}, false},
		{`{"event":"end-file","reason":"error","file_error":"loading failed"}`, Event{Kind: EvError, Err: "loading failed"}, true},
		{`{"error":"success","request_id":0}`, Event{}, false},
		{`not json`, Event{}, false},
	}
	for _, c := range cases {
		got, ok := parseEvent([]byte(c.in))
		if ok != c.ok || got != c.want {
			t.Errorf("parseEvent(%s) = %+v,%v ; attendu %+v,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

// Faux serveur mpv sur un vrai socket unix : vérifie commandes envoyées et événements reçus.
func TestPlayerAgainstFakeServer(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "s.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	cmds := make(chan []any, 16)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		go func() {
			sc := bufio.NewScanner(c)
			for sc.Scan() {
				var m struct {
					Command []any `json:"command"`
				}
				json.Unmarshal(sc.Bytes(), &m)
				cmds <- m.Command
			}
		}()
		c.Write([]byte(`{"event":"property-change","name":"time-pos","data":1.2}` + "\n"))
		c.Write([]byte(`{"event":"property-change","name":"time-pos","data":1.7}` + "\n")) // même seconde : filtré
		c.Write([]byte(`{"event":"end-file","reason":"eof"}` + "\n"))
	}()
	conn, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	p := Attach(conn)
	defer p.Close()

	if ev := <-p.Events(); ev.Kind != EvTime || ev.Value != 1.2 {
		t.Fatalf("premier événement = %+v", ev)
	}
	if ev := <-p.Events(); ev.Kind != EvEnded {
		t.Fatalf("deuxième événement = %+v (le time-pos de la même seconde aurait dû être filtré)", ev)
	}
	if err := p.Load("/tmp/x.mp3"); err != nil {
		t.Fatal(err)
	}
	var seen []string
	deadline := time.After(2 * time.Second)
	for len(seen) < 6 {
		select {
		case c := <-cmds:
			seen = append(seen, c[0].(string))
			if c[0] == "loadfile" && c[1] != "/tmp/x.mp3" {
				t.Errorf("loadfile cible = %v", c[1])
			}
		case <-deadline:
			t.Fatalf("commandes reçues: %v", seen)
		}
	}
}

func TestLoadStreamPipesCommandOutput(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "s.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	cmds := make(chan []any, 16)
	go func() {
		c, _ := ln.Accept()
		sc := bufio.NewScanner(c)
		for sc.Scan() {
			var m struct {
				Command []any `json:"command"`
			}
			json.Unmarshal(sc.Bytes(), &m)
			cmds <- m.Command
		}
	}()
	conn, _ := net.Dial("unix", sock)
	p := Attach(conn)
	p.dir = t.TempDir()
	defer p.Close()

	if err := p.LoadStream("printf", "bonjour"); err != nil {
		t.Fatal(err)
	}
	var fifo string
	deadline := time.After(2 * time.Second)
	for fifo == "" {
		select {
		case c := <-cmds:
			if c[0] == "loadfile" {
				fifo = c[1].(string)
			}
		case <-deadline:
			t.Fatal("pas de loadfile")
		}
	}
	// le lecteur (mpv) ouvre le tube : il doit recevoir la sortie de la commande puis EOF
	done := make(chan []byte, 1)
	go func() {
		f, err := os.Open(fifo)
		if err != nil {
			done <- nil
			return
		}
		defer f.Close()
		b, _ := io.ReadAll(f)
		done <- b
	}()
	select {
	case b := <-done:
		if string(b) != "bonjour" {
			t.Errorf("lu %q, attendu %q", b, "bonjour")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("le tube n'a pas atteint EOF")
	}
}
