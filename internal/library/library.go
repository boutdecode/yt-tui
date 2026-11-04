// Package library indexe les fichiers audio téléchargés et leurs métadonnées.
package library

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kevin/yt-tui/internal/yt"
)

// Les fichiers sont nommés « Titre [id].ext » par le téléchargeur.
var fileRe = regexp.MustCompile(`^(.+) \[([A-Za-z0-9_-]{11})\]\.(mp3|opus|m4a|ogg|webm|flac|wav|aac)$`)

// Meta est ce qui est mémorisé par fichier dans le cache d'index.
type Meta struct {
	Title      string  `json:"title"`
	Artist     string  `json:"artist"`
	Album      string  `json:"album"`
	Year       int     `json:"year"`
	Duration   float64 `json:"duration"`
	Codec      string  `json:"codec"`
	SampleRate int     `json:"sample_rate"`
	// clé d'invalidation du cache
	Size    int64 `json:"size"`
	ModTime int64 `json:"mtime"`
}

// indexVersion invalide les index écrits par une version qui lisait moins de tags
// (v2 : tags de flux d'Opus/Ogg).
const indexVersion = 2

type indexFile struct {
	Version   int             `json:"version"`
	ScannedAt int64           `json:"scanned_at"`
	Files     map[string]Meta `json:"files"` // clé = nom de fichier
}

// Library est l'index des fichiers locaux, sûr pour un usage concurrent.
type Library struct {
	dir       string
	indexPath string
	probe     func(path string) (Meta, error) // remplaçable dans les tests

	mu        sync.RWMutex
	tracks    []yt.Track
	byID      map[string]string
	scannedAt time.Time
}

// New crée une bibliothèque sur dir ; indexPath (peut être vide) est le cache JSON.
func New(dir, indexPath string) (*Library, error) {
	l := &Library{dir: dir, indexPath: indexPath, probe: ffprobe, byID: map[string]string{}}
	return l, l.Refresh()
}

// Refresh rescanne le dossier ; seuls les fichiers nouveaux ou modifiés sont ré-analysés.
func (l *Library) Refresh() error {
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return err
	}
	old := l.loadIndex()
	next := indexFile{Version: indexVersion, Files: map[string]Meta{}}

	type job struct {
		name string
		path string
		info os.FileInfo
	}
	var jobs []job
	var tracks []yt.Track
	byID := map[string]string{}
	metas := map[string]Meta{}
	var mu sync.Mutex
	for _, e := range entries {
		if e.IsDir() || fileRe.FindStringSubmatch(e.Name()) == nil {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		p := filepath.Join(l.dir, e.Name())
		if m, ok := old.Files[e.Name()]; ok && m.Size == info.Size() && m.ModTime == info.ModTime().Unix() {
			metas[e.Name()] = m
			continue
		}
		jobs = append(jobs, job{e.Name(), p, info})
	}
	// ffprobe en parallèle (4 à la fois) pour les fichiers nouveaux
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for _, j := range jobs {
		wg.Add(1)
		sem <- struct{}{}
		go func(j job) {
			defer wg.Done()
			defer func() { <-sem }()
			m, err := l.probe(j.path)
			if err != nil {
				m = Meta{} // fichier lisible mais sans tags : on garde les infos du nom
			}
			m.Size, m.ModTime = j.info.Size(), j.info.ModTime().Unix()
			mu.Lock()
			metas[j.name] = m
			mu.Unlock()
		}(j)
	}
	wg.Wait()

	for name, m := range metas {
		match := fileRe.FindStringSubmatch(name)
		title := strings.TrimSpace(m.Title)
		if title == "" {
			title = match[1]
		}
		artist := strings.TrimSpace(m.Artist)
		if artist == "" {
			artist = "Inconnu"
		}
		album := strings.TrimSpace(m.Album)
		if album == "" {
			album = "Singles"
		}
		p := filepath.Join(l.dir, name)
		tracks = append(tracks, yt.Track{
			ID: match[2], Title: title, Channel: artist, Album: album, Year: m.Year, Path: p,
			Duration: time.Duration(m.Duration * float64(time.Second)),
			Format:   formatLabel(m.Codec, m.SampleRate, match[3]),
		})
		byID[match[2]] = p
		next.Files[name] = m
	}
	sort.Slice(tracks, func(i, j int) bool {
		return strings.ToLower(tracks[i].Title) < strings.ToLower(tracks[j].Title)
	})
	now := time.Now()
	next.ScannedAt = now.Unix()
	l.saveIndex(next)

	l.mu.Lock()
	l.tracks, l.byID, l.scannedAt = tracks, byID, now
	l.mu.Unlock()
	return nil
}

func formatLabel(codec string, rate int, ext string) string {
	name := strings.ToUpper(codec)
	if name == "" || strings.HasPrefix(name, "PCM") {
		name = strings.ToUpper(ext)
	}
	if rate > 0 {
		return fmt.Sprintf("%s · %s kHz", name, strings.ReplaceAll(strconv.FormatFloat(float64(rate)/1000, 'f', -1, 64), ".", ","))
	}
	return name
}

func (l *Library) loadIndex() indexFile {
	idx := indexFile{Files: map[string]Meta{}}
	if l.indexPath == "" {
		return idx
	}
	if b, err := os.ReadFile(l.indexPath); err == nil {
		var f indexFile
		if json.Unmarshal(b, &f) == nil && f.Files != nil && f.Version == indexVersion {
			return f
		}
	}
	return idx
}

func (l *Library) saveIndex(f indexFile) {
	if l.indexPath == "" {
		return
	}
	b, err := json.Marshal(f)
	if err != nil || os.MkdirAll(filepath.Dir(l.indexPath), 0o755) != nil {
		return
	}
	tmp := l.indexPath + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		os.Rename(tmp, l.indexPath)
	}
}

// Tracks renvoie une copie des morceaux locaux triés par titre.
func (l *Library) Tracks() []yt.Track {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return append([]yt.Track(nil), l.tracks...)
}

// Path renvoie le fichier local d'une vidéo, s'il existe.
func (l *Library) Path(id string) (string, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	p, ok := l.byID[id]
	return p, ok
}

// ScannedAt renvoie l'heure de la dernière analyse.
func (l *Library) ScannedAt() time.Time {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.scannedAt
}

// Dir renvoie le dossier analysé.
func (l *Library) Dir() string { return l.dir }

// ParseProbe décode la sortie JSON de ffprobe (tags insensibles à la casse : ID3 vs Vorbis).
func ParseProbe(data []byte) (Meta, error) {
	var r struct {
		Format struct {
			Duration string            `json:"duration"`
			Tags     map[string]string `json:"tags"`
		} `json:"format"`
		Streams []struct {
			Codec      string            `json:"codec_name"`
			SampleRate string            `json:"sample_rate"`
			Tags       map[string]string `json:"tags"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return Meta{}, err
	}
	tags := map[string]string{}
	// Opus/Ogg rangent leurs tags dans le flux et non dans le conteneur : ceux du
	// conteneur (MP3, M4A, FLAC…) restent prioritaires, le flux complète.
	if len(r.Streams) > 0 {
		for k, v := range r.Streams[0].Tags {
			tags[strings.ToLower(k)] = v
		}
	}
	for k, v := range r.Format.Tags {
		tags[strings.ToLower(k)] = v
	}
	m := Meta{Title: tags["title"], Artist: tags["artist"], Album: tags["album"]}
	if m.Artist == "" {
		m.Artist = firstNonEmpty(tags["album_artist"], tags["uploader"], tags["channel"])
	}
	m.Duration, _ = strconv.ParseFloat(r.Format.Duration, 64)
	if d := firstNonEmpty(tags["date"], tags["year"], tags["release_date"]); len(d) >= 4 {
		m.Year, _ = strconv.Atoi(d[:4])
	}
	if len(r.Streams) > 0 {
		m.Codec = r.Streams[0].Codec
		m.SampleRate, _ = strconv.Atoi(r.Streams[0].SampleRate)
	}
	return m, nil
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

func ffprobe(path string) (Meta, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffprobe", "-v", "quiet", "-print_format", "json",
		"-show_entries", "format=duration:format_tags:stream=codec_name,sample_rate:stream_tags", "-select_streams", "a:0", "--", path)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return Meta{}, err
	}
	return ParseProbe(out.Bytes())
}

// ErrOutside signale un fichier hors du dossier de la bibliothèque.
var ErrOutside = errors.New("fichier hors du dossier de la bibliothèque")

// within vérifie que path est dans le dossier de la bibliothèque (sans « .. »).
func (l *Library) within(path string) bool {
	rel, err := filepath.Rel(l.dir, path)
	return err == nil && rel != "." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."
}

// Remove supprime le fichier de la piste id, puis réanalyse la bibliothèque.
func (l *Library) Remove(id string) error {
	p, ok := l.Path(id)
	if !ok {
		return errors.New("piste absente de la bibliothèque")
	}
	if !l.within(p) {
		return ErrOutside
	}
	if err := os.Remove(p); err != nil {
		return err
	}
	return l.Refresh()
}

// WriteTags réécrit l'artiste et l'album du fichier de la piste id (ffmpeg, flux copiés
// sans réencodage), puis réanalyse la bibliothèque.
func (l *Library) WriteTags(id, artist, album string) error {
	p, ok := l.Path(id)
	if !ok {
		return errors.New("piste absente de la bibliothèque")
	}
	if !l.within(p) {
		return ErrOutside
	}
	ext := filepath.Ext(p)
	tmp := strings.TrimSuffix(p, ext) + ".tagtmp" + ext
	args := []string{"-y", "-loglevel", "error", "-i", p, "-map", "0", "-c", "copy"}
	if artist != "" {
		args = append(args, "-metadata", "artist="+artist, "-metadata", "album_artist="+artist,
			"-metadata:s:a:0", "artist="+artist, "-metadata:s:a:0", "album_artist="+artist)
	}
	if album != "" { // les tags du flux (Opus/Ogg) l'emportent sinon sur ceux du conteneur
		args = append(args, "-metadata", "album="+album, "-metadata:s:a:0", "album="+album)
	}
	args = append(args, tmp)
	if out, err := exec.Command("ffmpeg", args...).CombinedOutput(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("ffmpeg : %s", strings.TrimSpace(string(out)))
	}
	if err := os.Rename(tmp, p); err != nil {
		os.Remove(tmp)
		return err
	}
	return l.Refresh()
}
