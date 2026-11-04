package yt

import (
	"context"
	"errors"
	"fmt"
	"regexp"
)

var videoIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// Related renvoie les n premières vidéos du « mix » YouTube (radio) construit à partir
// de la vidéo id. La vidéo de départ elle-même est exclue.
func Related(ctx context.Context, id string, n int) ([]Track, error) {
	if !videoIDRe.MatchString(id) {
		return nil, errors.New("identifiant vidéo invalide")
	}
	u := "https://www.youtube.com/watch?v=" + id + "&list=RD" + id
	args := append(baseArgs(), "--flat-playlist", "--no-warnings", "-J", "--playlist-end", fmt.Sprint(n+1), u)
	out, err := runYtDlp(ctx, args)
	if err != nil {
		return nil, err
	}
	ts, err := ParseSearch(out)
	if err != nil {
		return nil, err
	}
	return ExcludeIDs(ts, id), nil
}

// ExcludeIDs retire de ts les pistes dont l'id figure dans ids (et les doublons internes).
func ExcludeIDs(ts []Track, ids ...string) []Track {
	seen := make(map[string]bool, len(ids)+len(ts))
	for _, id := range ids {
		seen[id] = true
	}
	var out []Track
	for _, t := range ts {
		if !seen[t.ID] {
			seen[t.ID] = true
			out = append(out, t)
		}
	}
	return out
}
