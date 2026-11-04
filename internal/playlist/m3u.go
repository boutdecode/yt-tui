package playlist

import (
	"fmt"
	"strings"

	"github.com/kevin/yt-tui/internal/yt"
)

// M3U produit une playlist M3U étendue. pathOf renvoie le fichier local d'un morceau ;
// à défaut c'est l'URL YouTube qui est écrite.
func M3U(name string, ts []yt.Track, pathOf func(id string) (string, bool)) string {
	var b strings.Builder
	b.WriteString("#EXTM3U\n#PLAYLIST:" + oneLine(name) + "\n")
	for _, t := range ts {
		target := t.URL()
		if pathOf != nil {
			if p, ok := pathOf(t.ID); ok {
				target = p
			}
		}
		title := oneLine(t.Title)
		if t.Channel != "" {
			title = oneLine(t.Channel) + " - " + title
		}
		fmt.Fprintf(&b, "#EXTINF:%d,%s\n%s\n", int(t.Duration.Seconds()), title, target)
	}
	return b.String()
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// FileName transforme un nom de playlist en nom de fichier sûr (sans séparateurs ni caractères
// problématiques), jamais vide.
func FileName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r < 32, strings.ContainsRune(`/\:*?"<>|`, r):
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	s := strings.Trim(strings.TrimSpace(b.String()), ".")
	if s == "" {
		return "playlist"
	}
	return s
}
