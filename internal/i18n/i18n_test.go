package i18n

import "testing"

func TestDetect(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	cases := []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{"LANG": "fr_FR.UTF-8"}, FR},
		{map[string]string{"LANG": "en_US.UTF-8"}, EN},
		{map[string]string{"LANG": "de_DE.UTF-8"}, EN},
		{map[string]string{"LC_ALL": "fr_CA.UTF-8", "LANG": "en_US.UTF-8"}, FR},
		{map[string]string{"LC_ALL": "C", "LANG": "fr_FR.UTF-8"}, FR},
		{map[string]string{"LANGUAGE": "fr:en", "LANG": "en_US"}, FR},
		{map[string]string{"YT_TUI_LANG": "en", "LANG": "fr_FR"}, EN},
		{map[string]string{}, EN},
	}
	for _, c := range cases {
		if got := detect(env(c.env)); got != c.want {
			t.Errorf("%v → %s, attendu %s", c.env, got, c.want)
		}
	}
}

func TestT(t *testing.T) {
	defer Set(FR)
	Set(EN)
	if got := T("piste"); got != "track" {
		t.Errorf("en: %q", got)
	}
	if got := T("Inconnue %d", 3); got != "Inconnue 3" { // sans traduction : texte source
		t.Errorf("repli: %q", got)
	}
	Set(FR)
	if got := T("piste"); got != "piste" {
		t.Errorf("fr: %q", got)
	}
	Set("zz") // ignorée
	if Lang() != FR {
		t.Error("langue inconnue acceptée")
	}
}

func TestFromArgs(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"--lang", "en"}, EN}, {[]string{"-lang=fr"}, FR}, {[]string{"--lang=FR_fr"}, FR},
		{[]string{"--dir", "lang"}, ""}, {[]string{"--lang", "zz"}, ""}, {nil, ""},
	}
	for _, c := range cases {
		if got := FromArgs(c.args); got != c.want {
			t.Errorf("%v → %q, attendu %q", c.args, got, c.want)
		}
	}
}

func TestWithoutLang(t *testing.T) {
	got := WithoutLang([]string{"--lang", "en", "search", "-n", "3", "--lang=fr", "x"})
	if len(got) != 4 || got[0] != "search" || got[3] != "x" {
		t.Errorf("%v", got)
	}
}
