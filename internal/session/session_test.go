package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kevin/yt-tui/internal/yt"
)

func TestRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s", "session.json")
	in := State{
		Queue: []yt.Track{{ID: "abcdefghijk", Title: "A", Channel: "C", Duration: 90 * time.Second}, {ID: "bcdefghijkl", Title: "B"}},
		Pos:   1, At: 42.5, Shuffle: true, Repeat: 2, Volume: 65, Radio: true,
	}
	if err := Save(p, in); err != nil {
		t.Fatal(err)
	}
	out := Load(p)
	if len(out.Queue) != 2 || out.Queue[0].Duration != 90*time.Second || out.Pos != 1 || out.At != 42.5 ||
		!out.Shuffle || out.Repeat != 2 || out.Volume != 65 || !out.Radio {
		t.Errorf("aller-retour incorrect : %+v", out)
	}
}

func TestLoadBadFiles(t *testing.T) {
	dir := t.TempDir()
	if s := Load(filepath.Join(dir, "absent.json")); len(s.Queue) != 0 || s.Pos != -1 {
		t.Errorf("absent : %+v", s)
	}
	bad := filepath.Join(dir, "bad.json")
	os.WriteFile(bad, []byte("{pas du json"), 0o644)
	if s := Load(bad); len(s.Queue) != 0 || s.Pos != -1 {
		t.Errorf("corrompu : %+v", s)
	}
	out := filepath.Join(dir, "oob.json")
	os.WriteFile(out, []byte(`{"queue":[],"pos":3}`), 0o644)
	if s := Load(out); s.Pos != -1 {
		t.Errorf("index hors file : %+v", s)
	}
}
