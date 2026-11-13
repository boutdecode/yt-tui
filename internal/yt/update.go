package yt

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// latestReleaseURL est l'API publique GitHub (aucune donnée utilisateur n'est envoyée).
var latestReleaseURL = "https://api.github.com/repos/yt-dlp/yt-dlp/releases/latest"

// CheckUpdate compare la version de yt-dlp installée à la dernière version publiée.
// Il renvoie les deux versions et si une mise à jour est disponible.
func CheckUpdate(ctx context.Context) (current, latest string, outdated bool, err error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "yt-dlp", "--version").Output()
	if err != nil {
		return "", "", false, err
	}
	current = strings.TrimSpace(string(out))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestReleaseURL, nil)
	if err != nil {
		return current, "", false, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "yt-tui")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return current, "", false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return current, "", false, fmt.Errorf("GitHub: %s", resp.Status)
	}
	var rel struct {
		Tag string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return current, "", false, err
	}
	latest = strings.TrimSpace(rel.Tag)
	return current, latest, versionLess(current, latest), nil
}

// versionLess compare deux versions yt-dlp de la forme AAAA.MM.JJ[.N].
func versionLess(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			return x < y
		}
	}
	return false
}
