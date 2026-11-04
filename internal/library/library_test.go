package library

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func fakeProbe(calls *atomic.Int32) func(string) (Meta, error) {
	return func(p string) (Meta, error) {
		calls.Add(1)
		switch filepath.Base(p) {
		case "Zeta song [abcdefghijk].mp3":
			return Meta{Title: "Zeta", Artist: "Nord Quartet", Album: "Aube Boréale", Year: 2019, Duration: 125, Codec: "mp3", SampleRate: 44100}, nil
		}
		return Meta{}, nil // sans tags
	}
}

func TestRefreshMetadataAndCache(t *testing.T) {
	dir, idx := t.TempDir(), filepath.Join(t.TempDir(), "library.json")
	for _, n := range []string{"Zeta song [abcdefghijk].mp3", "alpha [A-b_1234567].opus", "random.mp3", "bad [short].mp3", "notes.txt"} {
		os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644)
	}
	var calls atomic.Int32
	l := &Library{dir: dir, indexPath: idx, probe: fakeProbe(&calls), byID: map[string]string{}}
	if err := l.Refresh(); err != nil {
		t.Fatal(err)
	}
	ts := l.Tracks()
	if len(ts) != 2 || ts[0].Title != "alpha" || ts[1].Title != "Zeta" {
		t.Fatalf("tracks = %+v", ts)
	}
	z := ts[1]
	if z.Channel != "Nord Quartet" || z.Album != "Aube Boréale" || z.Year != 2019 || z.Format != "MP3 · 44,1 kHz" || z.Duration.Seconds() != 125 {
		t.Errorf("métadonnées = %+v", z)
	}
	if a := ts[0]; a.Channel != "Inconnu" || a.Album != "Singles" {
		t.Errorf("valeurs par défaut = %+v", a)
	}
	if p, ok := l.Path("abcdefghijk"); !ok || filepath.Base(p) != "Zeta song [abcdefghijk].mp3" {
		t.Errorf("Path = %q,%v", p, ok)
	}
	if calls.Load() != 2 {
		t.Fatalf("%d analyses, attendu 2", calls.Load())
	}
	// 2e passage : tout vient du cache d'index
	if err := l.Refresh(); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Errorf("fichiers inchangés ré-analysés (%d appels)", calls.Load())
	}
	// un fichier modifié est ré-analysé
	os.WriteFile(filepath.Join(dir, "alpha [A-b_1234567].opus"), []byte("changé !"), 0o644)
	l.Refresh()
	if calls.Load() != 3 {
		t.Errorf("fichier modifié non ré-analysé (%d appels)", calls.Load())
	}
	if l.ScannedAt().IsZero() {
		t.Error("ScannedAt non renseigné")
	}
}

func TestParseProbe(t *testing.T) {
	m, err := ParseProbe([]byte(`{"streams":[{"codec_name":"flac","sample_rate":"48000"}],
	 "format":{"duration":"251.5","tags":{"TITLE":"Sillage","ARTIST":"Nord Quartet","ALBUM":"Aube","DATE":"2021-05-01"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if m.Title != "Sillage" || m.Artist != "Nord Quartet" || m.Album != "Aube" || m.Year != 2021 || m.Codec != "flac" || m.SampleRate != 48000 || m.Duration != 251.5 {
		t.Errorf("meta = %+v", m)
	}
	m2, _ := ParseProbe([]byte(`{"format":{"tags":{"uploader":"Une chaîne"}}}`))
	if m2.Artist != "Une chaîne" {
		t.Errorf("repli artiste = %q", m2.Artist)
	}
	if _, err := ParseProbe([]byte("nope")); err == nil {
		t.Error("JSON invalide accepté")
	}
}

func TestRemoveStaysInsideDir(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "Song [abcdefghijk].mp3")
	os.WriteFile(f, []byte("x"), 0o644)
	var calls atomic.Int32
	l := &Library{dir: dir, probe: fakeProbe(&calls), byID: map[string]string{}}
	l.Refresh()
	// un chemin hors dossier (index corrompu) est refusé
	outside := filepath.Join(t.TempDir(), "evil.mp3")
	os.WriteFile(outside, []byte("x"), 0o644)
	l.byID["zzzzzzzzzzz"] = outside
	if err := l.Remove("zzzzzzzzzzz"); err != ErrOutside {
		t.Errorf("err = %v, attendu ErrOutside", err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Error("le fichier hors dossier ne doit pas être supprimé")
	}
	if err := l.Remove("abcdefghijk"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(f); !os.IsNotExist(err) {
		t.Error("le fichier devait être supprimé")
	}
	if len(l.Tracks()) != 0 {
		t.Error("la bibliothèque doit être mise à jour")
	}
	if err := l.Remove("abcdefghijk"); err == nil {
		t.Error("supprimer une piste absente doit échouer")
	}
}

func TestWriteTagsWithFfmpeg(t *testing.T) {
	for _, ext := range []string{"mp3", "opus"} {
		t.Run(ext, func(t *testing.T) {
			if _, err := exec.LookPath("ffmpeg"); err != nil {
				t.Skip("ffmpeg absent")
			}
			if _, err := exec.LookPath("ffprobe"); err != nil {
				t.Skip("ffprobe absent")
			}
			dir := t.TempDir()
			f := filepath.Join(dir, "Tone [abcdefghijk]."+ext)
			if out, err := exec.Command("ffmpeg", "-loglevel", "error", "-f", "lavfi", "-i", "sine=duration=1", "-metadata", "artist=Ancien", "-metadata", "album=Vieux", f).CombinedOutput(); err != nil {
				t.Skipf("génération impossible : %s", out)
			}
			l, err := New(dir, "")
			if err != nil {
				t.Fatal(err)
			}
			if ts := l.Tracks(); len(ts) != 1 || ts[0].Channel != "Ancien" {
				t.Fatalf("tags initiaux = %+v", ts)
			}
			if err := l.WriteTags("abcdefghijk", "Nouvel Artiste", "Nouvel Album"); err != nil {
				t.Fatal(err)
			}
			ts := l.Tracks()
			if len(ts) != 1 || ts[0].Channel != "Nouvel Artiste" || ts[0].Album != "Nouvel Album" {
				t.Errorf("tags après écriture = %+v", ts)
			}
			if ents, _ := os.ReadDir(dir); len(ents) != 1 {
				t.Errorf("fichier temporaire laissé : %v", ents)
			}
		})
	}
}

func TestParseProbeStreamTags(t *testing.T) {
	js := `{"format":{"duration":"19.0"},"streams":[{"codec_name":"opus","sample_rate":"48000","tags":{"ARTIST":"jawed","title":"Me at the zoo","ALBUM":"Zoo"}}]}`
	m, err := ParseProbe([]byte(js))
	if err != nil || m.Artist != "jawed" || m.Album != "Zoo" || m.Title != "Me at the zoo" {
		t.Errorf("tags du flux ignorés : %+v (%v)", m, err)
	}
	js = `{"format":{"tags":{"artist":"Conteneur"}},"streams":[{"codec_name":"mp3","tags":{"artist":"Flux"}}]}`
	if m, _ := ParseProbe([]byte(js)); m.Artist != "Conteneur" {
		t.Errorf("le conteneur doit être prioritaire : %+v", m)
	}
}
