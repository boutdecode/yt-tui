// Package history garde les dernières pistes lues (la plus récente en premier).
package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/kevin/yt-tui/internal/yt"
)

// Max est le nombre de pistes conservées.
const Max = 200

// History est l'historique de lecture, persisté en JSON (chemin vide = en mémoire seulement).
type History struct {
	path string
	mu   sync.Mutex
	list []yt.Track
}

// Open charge l'historique ; un fichier absent ou illisible donne un historique vide.
func Open(path string) *History {
	h := &History{path: path}
	if path == "" {
		return h
	}
	if b, err := os.ReadFile(path); err == nil {
		json.Unmarshal(b, &h.list)
	}
	if len(h.list) > Max {
		h.list = h.list[:Max]
	}
	return h
}

// Add place t en tête. Une même piste ne figure qu'une fois (l'ancienne occurrence disparaît).
func (h *History) Add(t yt.Track) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	t.Path = ""
	out := make([]yt.Track, 0, min(len(h.list)+1, Max))
	out = append(out, t)
	for _, x := range h.list {
		if x.ID != t.ID && len(out) < Max {
			out = append(out, x)
		}
	}
	h.list = out
	return h.saveLocked()
}

// Tracks renvoie une copie de l'historique, la plus récente d'abord.
func (h *History) Tracks() []yt.Track {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]yt.Track(nil), h.list...)
}

func (h *History) saveLocked() error {
	if h.path == "" {
		return nil
	}
	b, err := json.Marshal(h.list)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(h.path), 0o755); err != nil {
		return err
	}
	tmp := h.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, h.path)
}
