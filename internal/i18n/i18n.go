// Package i18n traduit l'interface : le texte source est en français et sert de clé ;
// le catalogue anglais le traduit (repli : le texte français).
package i18n

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"
)

// Langues disponibles.
const (
	FR = "fr"
	EN = "en"
)

var cur atomic.Value // string

func init() { cur.Store(FR) }

// Detect devine la langue du système (YT_TUI_LANG, LC_ALL, LC_MESSAGES, LANGUAGE, LANG) :
// « fr… » donne le français, toute autre langue l'anglais.
func Detect() string {
	return detect(os.Getenv)
}

func detect(get func(string) string) string {
	for _, k := range []string{"YT_TUI_LANG", "LC_ALL", "LC_MESSAGES", "LANGUAGE", "LANG"} {
		v := strings.TrimSpace(get(k))
		if v == "" || v == "C" || v == "POSIX" {
			continue
		}
		if i := strings.IndexByte(v, ':'); i >= 0 { // LANGUAGE=fr:en
			v = v[:i]
		}
		if l, ok := Normalize(v); ok {
			return l
		}
		return EN
	}
	return EN
}

// Normalize ramène « fr_FR.UTF-8 », « en-US », « FR »… à fr ou en ; ok=false si inconnue.
func Normalize(s string) (string, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch {
	case strings.HasPrefix(s, "fr"):
		return FR, true
	case strings.HasPrefix(s, "en"):
		return EN, true
	}
	return "", false
}

// Set choisit la langue (fr ou en) ; une valeur inconnue est ignorée.
func Set(lang string) {
	if l, ok := Normalize(lang); ok {
		cur.Store(l)
	}
}

// Lang renvoie la langue courante.
func Lang() string { return cur.Load().(string) }

// T traduit s (texte français) et le formate avec args (fmt.Sprintf) s'il y en a.
func T(s string, args ...any) string {
	if Lang() == EN {
		if t, ok := en[s]; ok {
			s = t
		}
	}
	if len(args) > 0 {
		return fmt.Sprintf(s, args...)
	}
	return s
}

// Has indique si s possède une traduction anglaise (utilisé par les tests).
func Has(s string) bool { _, ok := en[s]; return ok }

// FromArgs lit « --lang fr », « --lang=fr » (ou -lang) dans args ; "" si absent ou inconnu.
func FromArgs(args []string) string {
	for i, a := range args {
		a = strings.TrimLeft(a, "-")
		if a == "lang" && i+1 < len(args) && strings.HasPrefix(args[i], "-") {
			l, _ := Normalize(args[i+1])
			return l
		}
		if v, ok := strings.CutPrefix(a, "lang="); ok && strings.HasPrefix(args[i], "-") {
			l, _ := Normalize(v)
			return l
		}
	}
	return ""
}

// WithoutLang retire « --lang x » / « --lang=x » de args (pour les sous-commandes).
func WithoutLang(args []string) []string {
	var out []string
	for i := 0; i < len(args); i++ {
		a := strings.TrimLeft(args[i], "-")
		if strings.HasPrefix(args[i], "-") && a == "lang" && i+1 < len(args) {
			i++
			continue
		}
		if strings.HasPrefix(args[i], "-") && strings.HasPrefix(a, "lang=") {
			continue
		}
		out = append(out, args[i])
	}
	return out
}
