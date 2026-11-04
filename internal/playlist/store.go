// Package playlist persiste les playlists en JSON.
package playlist

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/kevin/yt-tui/internal/yt"
)

// Store contient les playlists nommées.
type Store struct {
	path string
	mu   sync.Mutex
	data map[string][]yt.Track
}

// Open charge le fichier (absent = store vide).
func Open(path string) (*Store, error) {
	s := &Store{path: path, data: map[string][]yt.Track{}}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &s.data); err != nil {
		return nil, fmt.Errorf("%s illisible: %w", path, err)
	}
	if s.data == nil {
		s.data = map[string][]yt.Track{}
	}
	return s, nil
}

// Names renvoie les noms triés.
func (s *Store) Names() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	names := make([]string, 0, len(s.data))
	for n := range s.data {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Tracks renvoie une copie des morceaux d'une playlist.
func (s *Store) Tracks(name string) []yt.Track {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]yt.Track(nil), s.data[name]...)
}

// Add ajoute t (créant la playlist au besoin) ; sans doublon. Renvoie false si déjà présent.
func (s *Store) Add(name string, t yt.Track) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, x := range s.data[name] {
		if x.ID == t.ID {
			return false, nil
		}
	}
	t.Path = ""
	s.data[name] = append(s.data[name], t)
	return true, s.saveLocked()
}

// Create crée une playlist vide ; renvoie false si le nom existe déjà.
func (s *Store) Create(name string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[name]; ok || name == "" {
		return false, nil
	}
	s.data[name] = []yt.Track{}
	return true, s.saveLocked()
}

// Rename renomme une playlist ; renvoie false si l'ancien nom n'existe pas ou si le nouveau est pris.
func (s *Store) Rename(oldName, newName string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ts, ok := s.data[oldName]
	if !ok || newName == "" {
		return false, nil
	}
	if _, taken := s.data[newName]; taken && newName != oldName {
		return false, nil
	}
	delete(s.data, oldName)
	s.data[newName] = ts
	return true, s.saveLocked()
}

// Move déplace le morceau i de delta positions (-1 = vers le haut) ; renvoie le nouvel index.
func (s *Store) Move(name string, i, delta int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ts := s.data[name]
	j := i + delta
	if i < 0 || i >= len(ts) || j < 0 || j >= len(ts) {
		return i, nil
	}
	ts[i], ts[j] = ts[j], ts[i]
	return j, s.saveLocked()
}

// Remove retire le morceau d'index i.
func (s *Store) Remove(name string, i int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ts := s.data[name]
	if i < 0 || i >= len(ts) {
		return nil
	}
	s.data[name] = append(ts[:i:i], ts[i+1:]...)
	return s.saveLocked()
}

// Delete supprime une playlist.
func (s *Store) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, name)
	return s.saveLocked()
}

// saveLocked écrit de façon atomique (fichier temporaire + rename).
func (s *Store) saveLocked() error {
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
