// Package session sauvegarde et restaure l'état de lecture (file, position, options).
package session

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/kevin/yt-tui/internal/yt"
)

// State est l'état persisté entre deux lancements.
type State struct {
	Queue   []yt.Track `json:"queue"`
	Pos     int        `json:"pos"` // index du morceau courant, -1 si aucun
	At      float64    `json:"at"`  // position dans le morceau, en secondes
	Shuffle bool       `json:"shuffle"`
	Repeat  int        `json:"repeat"`
	Volume  float64    `json:"volume"`
	Radio   bool       `json:"radio"`
}

// Load lit l'état. Fichier absent ou illisible : état vide, sans erreur (la session
// n'est qu'un confort, elle ne doit jamais empêcher de démarrer).
func Load(path string) State {
	b, err := os.ReadFile(path)
	if err != nil {
		return State{Pos: -1}
	}
	var s State
	if json.Unmarshal(b, &s) != nil || s.Pos >= len(s.Queue) {
		return State{Pos: -1}
	}
	if s.Pos < -1 {
		s.Pos = -1
	}
	return s
}

// Save écrit l'état de façon atomique.
func Save(path string, s State) error {
	if path == "" {
		return errors.New("chemin de session vide")
	}
	b, err := json.MarshalIndent(s, "", " ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
