// Package thumb télécharge les miniatures YouTube et les rend dans le terminal
// avec des demi-blocs (▀) colorés : un caractère = 2 pixels verticaux.
package thumb

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg" // décodeur des miniatures YouTube
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Dimensions d'une miniature en cellules de terminal (11×6 px ≈ 16:9).
const (
	Cols = 11
	Rows = 3
)

// Dimensions de la grande image des panneaux « En lecture » / « Aperçu ».
const (
	LargeCols = 32
	LargeRows = 9
)

var idRe = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// Store charge, met en cache (disque + mémoire) et fournit les miniatures rendues.
type Store struct {
	dir     string
	baseURL string
	client  *http.Client
	sem     chan struct{}

	mu      sync.Mutex
	lines   map[string][]string
	large   map[string][]string
	pending map[string]bool
	pal     map[string]palEntry
}

// New crée un Store dont le cache disque est dir.
func New(dir string) *Store {
	return &Store{
		dir:     dir,
		baseURL: "https://i.ytimg.com/vi",
		client:  &http.Client{Timeout: 10 * time.Second},
		sem:     make(chan struct{}, 4),
		lines:   map[string][]string{},
		large:   map[string][]string{},
		pending: map[string]bool{},
		pal:     map[string]palEntry{},
	}
}

// Path renvoie le fichier image en cache disque, s'il existe (utile aux pochettes MPRIS).
func (s *Store) Path(id string) (string, bool) {
	if !idRe.MatchString(id) {
		return "", false
	}
	p := filepath.Join(s.dir, id+".jpg")
	_, err := os.Stat(p)
	return p, err == nil
}

// Lines renvoie les Rows lignes rendues de la miniature, si elle est prête.
func (s *Store) Lines(id string) ([]string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.lines[id]
	return l, ok
}

// Claim réserve le chargement de id : renvoie true une seule fois par id
// (un échec n'est donc pas retenté avant le prochain lancement).
func (s *Store) Claim(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending[id] {
		return false
	}
	s.pending[id] = true
	return true
}

// Large renvoie la grande image rendue (LargeRows lignes), si elle est prête.
func (s *Store) Large(id string) ([]string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.large[id]
	return l, ok
}

// ClaimLarge réserve le chargement de la grande image de id (une seule fois).
func (s *Store) ClaimLarge(id string) bool { return s.Claim("L:" + id) }

// Load obtient la miniature (cache disque puis réseau), la rend et la mémorise.
func (s *Store) Load(ctx context.Context, id string) error {
	img, err := s.image(ctx, id)
	if err != nil {
		return err
	}
	lines := Render(img, Cols, Rows)
	s.mu.Lock()
	s.lines[id] = lines
	s.mu.Unlock()
	return nil
}

// LoadLarge rend l'image en grand format (même cache disque que Load).
func (s *Store) LoadLarge(ctx context.Context, id string) error {
	img, err := s.image(ctx, id)
	if err != nil {
		return err
	}
	lines := Render(img, LargeCols, LargeRows)
	s.mu.Lock()
	s.large[id] = lines
	s.mu.Unlock()
	return nil
}

// image charge l'image depuis le cache disque ou le réseau.
func (s *Store) image(ctx context.Context, id string) (image.Image, error) {
	if !idRe.MatchString(id) {
		return nil, fmt.Errorf("identifiant vidéo invalide %q", id)
	}
	path := filepath.Join(s.dir, id+".jpg")
	data, err := os.ReadFile(path)
	if err != nil {
		if data, err = s.fetch(ctx, id); err != nil {
			return nil, err
		}
		if os.MkdirAll(s.dir, 0o755) == nil {
			tmp := path + ".tmp"
			if os.WriteFile(tmp, data, 0o644) == nil {
				os.Rename(tmp, path)
			}
		}
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		os.Remove(path) // cache corrompu
		return nil, fmt.Errorf("miniature illisible: %w", err)
	}
	return img, nil
}

func (s *Store) fetch(ctx context.Context, id string) ([]byte, error) {
	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"/"+id+"/mqdefault.jpg", nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("miniature %s: HTTP %d", id, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 4<<20))
}

// Render réduit img à cols×(rows*2) pixels (moyenne par zone) et renvoie rows
// lignes de demi-blocs : couleur de texte = pixel haut, fond = pixel bas.
func Render(img image.Image, cols, rows int) []string {
	w, h := cols, rows*2
	px := make([]color.RGBA, w*h)
	b := img.Bounds()
	sw, sh := b.Dx(), b.Dy()
	for ty := 0; ty < h; ty++ {
		y0, y1 := b.Min.Y+ty*sh/h, b.Min.Y+(ty+1)*sh/h
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for tx := 0; tx < w; tx++ {
			x0, x1 := b.Min.X+tx*sw/w, b.Min.X+(tx+1)*sw/w
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var r, g, bl, n uint64
			for y := y0; y < y1 && y < b.Max.Y; y++ {
				for x := x0; x < x1 && x < b.Max.X; x++ {
					c := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
					r, g, bl, n = r+uint64(c.R), g+uint64(c.G), bl+uint64(c.B), n+1
				}
			}
			if n > 0 {
				px[ty*w+tx] = color.RGBA{uint8(r / n), uint8(g / n), uint8(bl / n), 255}
			}
		}
	}
	out := make([]string, rows)
	for r := 0; r < rows; r++ {
		var sb strings.Builder
		for c := 0; c < cols; c++ {
			top, bot := px[(2*r)*w+c], px[(2*r+1)*w+c]
			sb.WriteString(lipgloss.NewStyle().Foreground(hex(top)).Background(hex(bot)).Render("▀"))
		}
		out[r] = sb.String()
	}
	return out
}

func hex(c color.RGBA) lipgloss.Color {
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B))
}

// Placeholder renvoie un cadre gris avec glyph au centre, affiché avant l'arrivée de l'image.
func Placeholder(glyph string) []string {
	bg := lipgloss.NewStyle().Background(lipgloss.AdaptiveColor{Light: "#D1D5DB", Dark: "#374151"}).
		Foreground(lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"})
	out := make([]string, Rows)
	for r := range out {
		out[r] = bg.Render(strings.Repeat(" ", Cols))
	}
	pad := (Cols - lipgloss.Width(glyph)) / 2
	out[Rows/2] = bg.Render(strings.Repeat(" ", pad) + glyph + strings.Repeat(" ", Cols-pad-lipgloss.Width(glyph)))
	return out
}
