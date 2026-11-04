package yt

import (
	"testing"
	"time"
)

func TestParseSearch(t *testing.T) {
	data := []byte(`{"entries":[
	 {"id":"abc12345678","title":"Lofi beats","channel":"Chill","duration":125.0},
	 {"id":"def12345678","title":"Live","channel":null,"uploader":"Someone","duration":null},
	 {"id":"","title":"sans id"}]}`)
	got, err := ParseSearch(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, attendu 2", len(got))
	}
	if got[0].Duration != 125*time.Second || got[0].Channel != "Chill" {
		t.Errorf("entrée 0 = %+v", got[0])
	}
	if got[1].Channel != "Someone" || got[1].Duration != 0 {
		t.Errorf("entrée 1 = %+v", got[1])
	}
	if _, err := ParseSearch([]byte("nope")); err == nil {
		t.Error("JSON invalide accepté")
	}
}

func TestParseProgressLine(t *testing.T) {
	for line, want := range map[string]float64{"YTTUI  12.5%": 12.5, "YTTUI 100%": 100, "YTTUI   0.0%": 0} {
		if got, ok := ParseProgressLine(line); !ok || got != want {
			t.Errorf("%q = %v,%v ; attendu %v", line, got, ok, want)
		}
	}
	for _, line := range []string{"FILE:/x", "YTTUI NA", "[download] 5%"} {
		if _, ok := ParseProgressLine(line); ok {
			t.Errorf("%q ne devrait pas être reconnue", line)
		}
	}
}

func TestParseEntriesAndURL(t *testing.T) {
	data := []byte(`{"entries":[
	 {"_type":"url","ie_key":"YoutubeTab","id":"PLabc_-123","title":"Album complet","channel":"Chaîne A"},
	 {"id":"","title":"sans id"}]}`)
	es, err := ParseEntries(data, KindPlaylist)
	if err != nil || len(es) != 1 {
		t.Fatalf("entries = %+v, err = %v", es, err)
	}
	if es[0].Kind != KindPlaylist || es[0].Channel != "Chaîne A" {
		t.Errorf("entrée = %+v", es[0])
	}
	if got := es[0].URL(); got != "https://www.youtube.com/playlist?list=PLabc_-123" {
		t.Errorf("URL playlist = %q", got)
	}
	ch := Entry{Kind: KindChannel, ID: "UCxyz"}
	if got := ch.URL(); got != "https://www.youtube.com/channel/UCxyz/videos" {
		t.Errorf("URL chaîne = %q", got)
	}
	for _, id := range []string{"a b", "x;rm", "", "../x"} {
		if (Entry{Kind: KindPlaylist, ID: id}).URL() != "" {
			t.Errorf("id suspect %q accepté", id)
		}
	}
}
