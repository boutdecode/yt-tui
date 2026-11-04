package playlist

import (
	"path/filepath"
	"testing"

	"github.com/kevin/yt-tui/internal/yt"
)

func TestStorePersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "p.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	a, b := yt.Track{ID: "aaaaaaaaaaa", Title: "A"}, yt.Track{ID: "bbbbbbbbbbb", Title: "B", Path: "/tmp/x"}
	if ok, _ := s.Add("mix", a); !ok {
		t.Fatal("ajout refusé")
	}
	if ok, _ := s.Add("mix", a); ok {
		t.Error("doublon accepté")
	}
	s.Add("mix", b)

	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ts := s2.Tracks("mix")
	if len(ts) != 2 || ts[1].Title != "B" || ts[1].Path != "" {
		t.Fatalf("rechargé = %+v", ts)
	}
	s2.Remove("mix", 0)
	if ts := s2.Tracks("mix"); len(ts) != 1 || ts[0].ID != b.ID {
		t.Errorf("après Remove = %+v", ts)
	}
	s2.Delete("mix")
	if len(s2.Names()) != 0 {
		t.Error("playlist non supprimée")
	}
}

func TestCreateRenameMove(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "p.json"))
	if ok, _ := s.Create("a"); !ok {
		t.Fatal("Create refusé")
	}
	if ok, _ := s.Create("a"); ok {
		t.Error("doublon de playlist accepté")
	}
	for _, id := range []string{"aaaaaaaaaaa", "bbbbbbbbbbb", "ccccccccccc"} {
		s.Add("a", yt.Track{ID: id, Title: id})
	}
	if j, _ := s.Move("a", 2, -1); j != 1 {
		t.Errorf("Move = %d, attendu 1", j)
	}
	if ts := s.Tracks("a"); ts[1].ID != "ccccccccccc" || ts[2].ID != "bbbbbbbbbbb" {
		t.Errorf("ordre = %v", ts)
	}
	if j, _ := s.Move("a", 0, -1); j != 0 {
		t.Error("déplacement hors bornes accepté")
	}
	s.Create("b")
	if ok, _ := s.Rename("a", "b"); ok {
		t.Error("renommage vers un nom existant accepté")
	}
	if ok, _ := s.Rename("a", "c"); !ok || len(s.Tracks("c")) != 3 || len(s.Tracks("a")) != 0 {
		t.Error("Rename n'a pas conservé les pistes")
	}
	s2, _ := Open(s.path)
	if len(s2.Tracks("c")) != 3 {
		t.Error("Rename non persisté")
	}
}
