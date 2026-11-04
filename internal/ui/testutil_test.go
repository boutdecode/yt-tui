package ui

import (
	"testing"
	"time"

	"github.com/kevin/yt-tui/internal/thumb"
	"github.com/kevin/yt-tui/internal/yt"
)

func newTestThumbs(t *testing.T) *thumb.Store { return thumb.New(t.TempDir()) }

func ytTrack(title, channel string) yt.Track {
	return yt.Track{ID: "abcdefghijk", Title: title, Channel: channel, Duration: 3*time.Minute + 5*time.Second}
}
