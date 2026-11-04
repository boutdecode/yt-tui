package yt

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

// ParseSearch décode la sortie `yt-dlp --flat-playlist -J ytsearchN:...`.
func ParseSearch(data []byte) ([]Track, error) {
	var r struct {
		Entries []struct {
			ID       string   `json:"id"`
			Title    string   `json:"title"`
			Channel  string   `json:"channel"`
			Uploader string   `json:"uploader"`
			Duration *float64 `json:"duration"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("réponse yt-dlp invalide: %w", err)
	}
	var out []Track
	for _, e := range r.Entries {
		if e.ID == "" || e.Title == "" {
			continue
		}
		ch := e.Channel
		if ch == "" {
			ch = e.Uploader
		}
		t := Track{ID: e.ID, Title: e.Title, Channel: ch}
		if e.Duration != nil {
			t.Duration = time.Duration(*e.Duration * float64(time.Second))
		}
		out = append(out, t)
	}
	return out, nil
}

// Search recherche n vidéos sur YouTube via yt-dlp (sans clé API).
func Search(ctx context.Context, query string, n int) ([]Track, error) {
	args := append(baseArgs(), "--flat-playlist", "--no-warnings", "-J",
		fmt.Sprintf("ytsearch%d:%s", n, query))
	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		log.Printf("search %q: yt-dlp a échoué: %v\n--- stderr ---\n%s", query, err, stderr.String())
		return nil, errors.New("yt-dlp: " + lastLine(stderr.String(), err))
	}
	return ParseSearch(out)
}

func lastLine(s string, fallback error) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return fallback.Error()
	}
	lines := strings.Split(s, "\n")
	return lines[len(lines)-1]
}
