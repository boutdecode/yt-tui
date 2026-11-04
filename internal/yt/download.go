package yt

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"log"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	progressPrefix = "YTTUI "
	filePrefix     = "FILE:"
)

var pctRe = regexp.MustCompile(`(\d+(?:\.\d+)?)%`)

// ParseProgressLine extrait un pourcentage (0..100) d'une ligne de progression.
func ParseProgressLine(line string) (float64, bool) {
	if !strings.HasPrefix(line, progressPrefix) {
		return 0, false
	}
	m := pctRe.FindStringSubmatch(line)
	if m == nil {
		return 0, false
	}
	v, err := strconv.ParseFloat(m[1], 64)
	return v, err == nil
}

// Download télécharge l'audio de t dans dir au format donné et renvoie le
// chemin du fichier final. onProgress reçoit un pourcentage 0..100.
func Download(ctx context.Context, dir, format string, t Track, onProgress func(float64)) (string, error) {
	args := append(baseArgs(),
		"-x", "--audio-format", format, "--embed-metadata",
		"--no-playlist", "--no-warnings", "--quiet", "--progress", "--newline", "--color", "never",
		"--progress-template", "download:"+progressPrefix+"%(progress._percent_str)s",
		"--print", "after_move:"+filePrefix+"%(filepath)s",
		"-o", filepath.Join(dir, "%(title)s [%(id)s].%(ext)s"),
		"--", t.URL())
	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	var path string
	sc := bufio.NewScanner(stdout)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if p, ok := ParseProgressLine(line); ok {
			if onProgress != nil {
				onProgress(p)
			}
		} else if strings.HasPrefix(line, filePrefix) {
			path = strings.TrimPrefix(line, filePrefix)
		}
	}
	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		log.Printf("download %s: yt-dlp a échoué: %v\n--- stderr ---\n%s", t.ID, err, stderr.String())
		return "", errors.New("yt-dlp: " + lastLine(stderr.String(), err))
	}
	if path == "" {
		return "", errors.New("yt-dlp n'a pas indiqué le fichier produit")
	}
	return path, nil
}
