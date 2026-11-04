package yt

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os/exec"
	"regexp"
)

// Kind est le type de résultat d'une recherche YouTube.
type Kind int

const (
	KindVideo    Kind = iota // piste
	KindPlaylist             // « album »
	KindChannel              // « artiste »
)

// Entry est un résultat de recherche : vidéo, playlist ou chaîne.
type Entry struct {
	Kind    Kind
	ID      string
	Title   string
	Channel string
}

var safeIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// URL renvoie l'adresse à passer à yt-dlp (vide si l'identifiant est suspect).
func (e Entry) URL() string {
	if !safeIDRe.MatchString(e.ID) {
		return ""
	}
	switch e.Kind {
	case KindPlaylist:
		return "https://www.youtube.com/playlist?list=" + e.ID
	case KindChannel:
		return "https://www.youtube.com/channel/" + e.ID + "/videos"
	}
	return "https://www.youtube.com/watch?v=" + e.ID
}

// Filtres de type de la page de résultats YouTube (paramètre sp).
var spFilter = map[Kind]string{KindPlaylist: "EgIQAw%3D%3D", KindChannel: "EgIQAg%3D%3D"}

// ParseEntries décode la sortie de `yt-dlp --flat-playlist -J` en entrées d'un type donné.
func ParseEntries(data []byte, kind Kind) ([]Entry, error) {
	ts, err := ParseSearch(data) // même format : entries[{id,title,channel,uploader}]
	if err != nil {
		return nil, err
	}
	out := make([]Entry, len(ts))
	for i, t := range ts {
		out[i] = Entry{Kind: kind, ID: t.ID, Title: t.Title, Channel: t.Channel}
	}
	return out, nil
}

// SearchKind cherche des playlists ou des chaînes (les vidéos passent par Search).
func SearchKind(ctx context.Context, query string, kind Kind, n int) ([]Entry, error) {
	sp, ok := spFilter[kind]
	if !ok {
		return nil, errors.New("SearchKind : type non supporté")
	}
	u := "https://www.youtube.com/results?search_query=" + url.QueryEscape(query) + "&sp=" + sp
	args := append(baseArgs(), "--flat-playlist", "--no-warnings", "-J", "--playlist-end", fmt.Sprint(n), u)
	out, err := runYtDlp(ctx, args)
	if err != nil {
		return nil, err
	}
	return ParseEntries(out, kind)
}

// ListEntries liste les vidéos d'une playlist ou d'une chaîne (n premières).
func ListEntries(ctx context.Context, e Entry, n int) ([]Track, error) {
	u := e.URL()
	if u == "" || e.Kind == KindVideo {
		return nil, errors.New("entrée non listable")
	}
	args := append(baseArgs(), "--flat-playlist", "--no-warnings", "-J", "--playlist-end", fmt.Sprint(n), u)
	out, err := runYtDlp(ctx, args)
	if err != nil {
		return nil, err
	}
	return ParseSearch(out)
}

func runYtDlp(ctx context.Context, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		log.Printf("yt-dlp %v: %v\n--- stderr ---\n%s", args, err, stderr.String())
		return nil, errors.New("yt-dlp: " + lastLine(stderr.String(), err))
	}
	return out, nil
}
