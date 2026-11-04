package playlist

import (
	"strings"
	"testing"
	"time"

	"github.com/kevin/yt-tui/internal/yt"
)

func TestM3U(t *testing.T) {
	ts := []yt.Track{
		{ID: "aaaaaaaaaaa", Title: "Un\ntitre", Channel: "Artiste", Duration: 125 * time.Second},
		{ID: "bbbbbbbbbbb", Title: "Local"},
	}
	out := M3U("Ma liste", ts, func(id string) (string, bool) {
		if id == "bbbbbbbbbbb" {
			return "/musique/Local [bbbbbbbbbbb].mp3", true
		}
		return "", false
	})
	want := []string{"#EXTM3U", "#PLAYLIST:Ma liste", "#EXTINF:125,Artiste - Un titre",
		"https://www.youtube.com/watch?v=aaaaaaaaaaa", "#EXTINF:0,Local", "/musique/Local [bbbbbbbbbbb].mp3"}
	if got := strings.Split(strings.TrimSpace(out), "\n"); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("M3U =\n%s", out)
	}
}

func TestFileName(t *testing.T) {
	for in, want := range map[string]string{"Rock/Pop": "Rock_Pop", "  ..  ": "playlist", "a:b*c": "a_b_c", "Été": "Été"} {
		if got := FileName(in); got != want {
			t.Errorf("FileName(%q) = %q, attendu %q", in, got, want)
		}
	}
}
