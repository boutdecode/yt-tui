package thumb

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func testJPEG(t *testing.T) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 320, 180))
	for y := 0; y < 180; y++ {
		for x := 0; x < 320; x++ {
			img.Set(x, y, color.RGBA{200, uint8(x % 256), 30, 255})
		}
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestRenderDimensions(t *testing.T) {
	lines := Render(image.NewRGBA(image.Rect(0, 0, 320, 180)), Cols, Rows)
	if len(lines) != Rows {
		t.Fatalf("%d lignes, attendu %d", len(lines), Rows)
	}
	for i, l := range lines {
		if w := lipgloss.Width(l); w != Cols {
			t.Errorf("ligne %d : largeur %d, attendu %d", i, w, Cols)
		}
	}
	for i, l := range Placeholder("♪") {
		if w := lipgloss.Width(l); w != Cols {
			t.Errorf("placeholder ligne %d : largeur %d", i, w)
		}
	}
}

func TestLoadCachesOnDisk(t *testing.T) {
	var hits atomic.Int32
	data := testJPEG(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/abcdefghijk/mqdefault.jpg" {
			http.NotFound(w, r)
			return
		}
		w.Write(data)
	}))
	defer srv.Close()

	dir := t.TempDir()
	s := New(dir)
	s.baseURL = srv.URL
	if err := s.Load(context.Background(), "abcdefghijk"); err != nil {
		t.Fatal(err)
	}
	if l, ok := s.Lines("abcdefghijk"); !ok || len(l) != Rows {
		t.Fatalf("Lines = %v,%v", l, ok)
	}
	// Nouveau Store, même dossier : doit servir depuis le disque sans réseau.
	s2 := New(dir)
	s2.baseURL = "http://127.0.0.1:1"
	if err := s2.Load(context.Background(), "abcdefghijk"); err != nil {
		t.Fatalf("cache disque non utilisé: %v", err)
	}
	if hits.Load() != 1 {
		t.Errorf("%d requêtes réseau, attendu 1", hits.Load())
	}
	if err := s.Load(context.Background(), "zzzzzzzzzzz"); err == nil {
		t.Error("404 accepté")
	}
}

func TestInvalidIDAndClaim(t *testing.T) {
	s := New(t.TempDir())
	for _, id := range []string{"../../etc/passwd", "court", "avec espace1", ""} {
		if err := s.Load(context.Background(), id); err == nil {
			t.Errorf("id %q accepté", id)
		}
	}
	if !s.Claim("abcdefghijk") || s.Claim("abcdefghijk") {
		t.Error("Claim doit renvoyer true une seule fois")
	}
}

func TestLoadLarge(t *testing.T) {
	data := testJPEG(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(data) }))
	defer srv.Close()
	s := New(t.TempDir())
	s.baseURL = srv.URL
	if err := s.LoadLarge(context.Background(), "abcdefghijk"); err != nil {
		t.Fatal(err)
	}
	l, ok := s.Large("abcdefghijk")
	if !ok || len(l) != LargeRows || lipgloss.Width(l[0]) != LargeCols {
		t.Fatalf("grande image = %d lignes, largeur %d", len(l), lipgloss.Width(l[0]))
	}
	if _, ok := s.Lines("abcdefghijk"); ok {
		t.Error("LoadLarge ne doit pas remplir la petite miniature")
	}
	if !s.ClaimLarge("abcdefghijk") || s.ClaimLarge("abcdefghijk") {
		t.Error("ClaimLarge doit renvoyer true une seule fois")
	}
	if !s.Claim("abcdefghijk") {
		t.Error("Claim et ClaimLarge doivent être indépendants")
	}
}
