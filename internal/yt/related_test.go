package yt

import "testing"

func TestExcludeIDs(t *testing.T) {
	in := []Track{{ID: "a"}, {ID: "b"}, {ID: "a"}, {ID: "c"}}
	out := ExcludeIDs(in, "b")
	if len(out) != 2 || out[0].ID != "a" || out[1].ID != "c" {
		t.Errorf("ExcludeIDs = %+v", out)
	}
}

func TestRelatedRejectsBadID(t *testing.T) {
	if _, err := Related(nil, "pas un id; rm -rf", 5); err == nil {
		t.Error("id invalide accepté")
	}
}
