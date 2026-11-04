// Package config regroupe la configuration et les chemins de l'application.
package config

import (
	"os"
	"os/exec"
	"path/filepath"
)

// Config est la configuration d'exécution de yt-tui.
type Config struct {
	MusicDir    string // dossier des fichiers téléchargés
	ConfigDir   string // dossier des playlists
	CacheDir    string // cache (miniatures)
	Format      string // mp3 | opus | m4a
	AutoCache   bool   // télécharge en arrière-plan ce qui est lu en streaming
	Results     int    // nombre de résultats de recherche
	Transparent bool   // ne pas peindre le fond : garde celui du terminal
	NoNotify    bool   // pas de notifications de bureau
	NoResume    bool   // ne pas restaurer la session précédente
	StaticTheme bool   // palette fixe : ne suit pas les couleurs de la pochette
}

var formats = map[string]bool{"mp3": true, "opus": true, "m4a": true, "flac": true, "wav": true}

// ValidFormat indique si f est un format audio supporté.
func ValidFormat(f string) bool { return formats[f] }

// Default renvoie la configuration par défaut (chemins XDG).
func Default() (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, err
	}
	cfgBase, err := os.UserConfigDir()
	if err != nil {
		cfgBase = filepath.Join(home, ".config")
	}
	cacheBase, err := os.UserCacheDir()
	if err != nil {
		cacheBase = filepath.Join(home, ".cache")
	}
	return Config{
		CacheDir:  filepath.Join(cacheBase, "yt-tui"),
		MusicDir:  filepath.Join(home, "Music", "yt-tui"),
		ConfigDir: filepath.Join(cfgBase, "yt-tui"),
		Format:    "mp3",
		Results:   20,
	}, nil
}

// PlaylistsPath est le fichier JSON des playlists.
func (c Config) PlaylistsPath() string { return filepath.Join(c.ConfigDir, "playlists.json") }

// SessionPath est le fichier JSON de la session (file, position, options).
func (c Config) SessionPath() string {
	if c.ConfigDir == "" {
		return ""
	}
	return filepath.Join(c.ConfigDir, "session.json")
}

// HistoryPath est le fichier JSON de l'historique de lecture.
func (c Config) HistoryPath() string {
	if c.ConfigDir == "" {
		return ""
	}
	return filepath.Join(c.ConfigDir, "history.json")
}

// ThumbDir est le cache disque des miniatures.
func (c Config) ThumbDir() string { return filepath.Join(c.CacheDir, "thumbs") }

// LogPath est le fichier de log du mode --debug.
func (c Config) LogPath() string { return filepath.Join(c.ConfigDir, "yt-tui.log") }

// MpvLogPath reçoit les logs détaillés de mpv en mode --debug.
func (c Config) MpvLogPath() string { return filepath.Join(c.ConfigDir, "mpv.log") }

// YtDlpConfPath est le fichier de configuration yt-dlp partagé avec mpv.
func (c Config) YtDlpConfPath() string { return filepath.Join(c.ConfigDir, "yt-dlp.conf") }

// Ensure crée les dossiers nécessaires et un yt-dlp.conf par défaut s'il manque.
// YouTube exige aujourd'hui un runtime JavaScript (deno, node ou bun) et certains
// clients renvoient des 403 ; le fichier est éditable par l'utilisateur.
func (c Config) Ensure() error {
	if err := os.MkdirAll(c.MusicDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(c.ConfigDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(c.ThumbDir(), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(c.YtDlpConfPath()); err == nil {
		return nil
	}
	conf := "# Options yt-dlp utilisées par yt-tui (recherche, téléchargement et streaming mpv).\n" +
		"--extractor-args youtube:player_client=web,mweb\n"
	for _, rt := range []string{"deno", "node", "bun"} {
		if _, err := exec.LookPath(rt); err == nil {
			conf += "--js-runtimes " + rt + "\n"
			break
		}
	}
	return os.WriteFile(c.YtDlpConfPath(), []byte(conf), 0o644)
}
