package notify

import "testing"

func TestNilNotifierIsSafe(t *testing.T) {
	var n *Notifier
	n.Show("titre", "corps", "")
	n.Close()
}

func TestHints(t *testing.T) {
	h := Hints()
	for _, k := range []string{"urgency", "category", "x-canonical-private-synchronous"} {
		if _, ok := h[k]; !ok {
			t.Errorf("indication %q manquante", k)
		}
	}
}
