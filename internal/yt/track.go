// Package yt enveloppe yt-dlp : recherche et téléchargement.
package yt

import "time"

// Track est un morceau YouTube (éventuellement déjà téléchargé).
type Track struct {
	ID       string        `json:"id"`
	Title    string        `json:"title"`
	Channel  string        `json:"channel"` // artiste / chaîne
	Duration time.Duration `json:"duration"`
	Path     string        `json:"-"` // renseigné pour les fichiers locaux
	Album    string        `json:"-"` // métadonnées lues dans le fichier local
	Year     int           `json:"-"`
	Format   string        `json:"-"` // ex. « MP3 · 44,1 kHz »
}

// URL renvoie l'URL de la vidéo.
func (t Track) URL() string { return "https://www.youtube.com/watch?v=" + t.ID }

// ConfigFile, s'il est renseigné, est passé à yt-dlp via --config-locations.
var ConfigFile string

// StreamArgs renvoie les arguments de yt-dlp pour écrire l'audio de t sur stdout.
func StreamArgs(t Track) []string {
	return append(baseArgs(), "--no-warnings", "-q", "-f", "bestaudio/best", "-o", "-", "--", t.URL())
}

func baseArgs() []string {
	if ConfigFile == "" {
		return nil
	}
	return []string{"--config-locations", ConfigFile}
}
