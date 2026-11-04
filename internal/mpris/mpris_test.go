package mpris

import (
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

func TestTrackPathIsValidObjectPath(t *testing.T) {
	for _, id := range []string{"abc12345678", "-UfI1X-MSig", "A-b_1234567", "../../x"} {
		p := TrackPath(id)
		if !p.IsValid() {
			t.Errorf("TrackPath(%q) = %q invalide", id, p)
		}
	}
	if TrackPath("-a") == TrackPath("_a") {
		t.Error("collision entre '-' et '_'")
	}
	if TrackPath("") != noTrack {
		t.Error("id vide doit donner NoTrack")
	}
}

func TestMetadata(t *testing.T) {
	md := Metadata(State{HasTrack: true, ID: "abc12345678", Title: "T", Artist: "A", URL: "u",
		Length: 90 * time.Second, ArtURL: "file:///x.jpg"})
	if md["mpris:length"].Value().(int64) != 90_000_000 {
		t.Errorf("length = %v", md["mpris:length"])
	}
	if got := md["xesam:artist"].Value().([]string); len(got) != 1 || got[0] != "A" {
		t.Errorf("artist = %v", got)
	}
	if md["mpris:artUrl"].Value() != "file:///x.jpg" {
		t.Errorf("artUrl = %v", md["mpris:artUrl"])
	}
	if _, ok := Metadata(State{HasTrack: true, ID: "abc12345678"})["mpris:artUrl"]; ok {
		t.Error("artUrl présent sans image")
	}
	empty := Metadata(State{})
	if empty["mpris:trackid"].Value().(dbus.ObjectPath) != noTrack || len(empty) != 1 {
		t.Errorf("metadata vide = %v", empty)
	}
}
