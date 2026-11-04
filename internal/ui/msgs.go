package ui

import "github.com/kevin/yt-tui/internal/yt"

type searchResultMsg struct {
	videos    []yt.Track
	playlists []yt.Entry
	channels  []yt.Entry
	err       error
}

// previewMsg : premières vidéos d'une playlist/chaîne sélectionnée (panneau Aperçu).
type previewMsg struct {
	key    string
	tracks []yt.Track
	err    error
}

// collectionMsg : contenu complet d'une playlist/chaîne ouverte avec Entrée.
type collectionMsg struct {
	entry  yt.Entry
	tracks []yt.Track
	err    error
}

type libRefreshedMsg struct{ err error }

type playerClosedMsg struct{}

type dlUpdateMsg struct {
	id  int
	pct float64
}

type dlDoneMsg struct {
	id   int
	path string
	err  error
}

// radioMsg : suggestions YouTube pour prolonger la file.
type radioMsg struct {
	seed   string
	tracks []yt.Track
	err    error
}

// tagsDoneMsg : fin de la réécriture des tags d'un fichier.
type tagsDoneMsg struct {
	title string
	err   error
}

// themeMsg : thème dérivé de la pochette de la piste id (thème par défaut si elle n'en a pas).
type themeMsg struct {
	id    string
	theme Theme
}

// thumbMsg signale qu'une image est prête (ou a échoué) : l'écran doit être redessiné.
type thumbMsg struct {
	id  string
	err error
}
