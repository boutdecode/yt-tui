package history

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/kevin/yt-tui/internal/yt"
)

func TestAddOrderDedupAndPersist(t *testing.T) {
	p := filepath.Join(t.TempDir(), "h.json")
	h := Open(p)
	for _, id := range []string{"a", "b", "c", "a"} {
		h.Add(yt.Track{ID: id, Title: id})
	}
	got := h.Tracks()
	if len(got) != 3 || got[0].ID != "a" || got[1].ID != "c" || got[2].ID != "b" {
		t.Fatalf("ordre/dédup = %+v", got)
	}
	if again := Open(p).Tracks(); len(again) != 3 || again[0].ID != "a" {
		t.Errorf("persistance = %+v", again)
	}
}

func TestMax(t *testing.T) {
	h := Open("")
	for i := 0; i < Max+50; i++ {
		h.Add(yt.Track{ID: fmt.Sprint(i)})
	}
	if n := len(h.Tracks()); n != Max {
		t.Errorf("len = %d, attendu %d", n, Max)
	}
}
