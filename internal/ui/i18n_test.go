package ui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/kevin/yt-tui/internal/i18n"
)

// uiStrings liste les textes français passés à txt(), plural() et hint{} dans les sources
// non-test du paquet : chacun doit avoir une traduction anglaise.
func uiStrings(t *testing.T) []string {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool { return !strings.HasSuffix(fi.Name(), "_test.go") }, 0)
	if err != nil {
		t.Fatal(err)
	}
	set := map[string]bool{}
	lit := func(e ast.Expr) {
		if b, ok := e.(*ast.BasicLit); ok && b.Kind == token.STRING {
			s, _ := strconv.Unquote(b.Value)
			set[s] = true
		}
	}
	allLits := func(e ast.Expr) { // tous les littéraux d'une expression (tableaux, maps…)
		ast.Inspect(e, func(n ast.Node) bool {
			if b, ok := n.(*ast.BasicLit); ok {
				lit(b)
			}
			return true
		})
	}
	for _, p := range pkgs {
		for _, f := range p.Files {
			ast.Inspect(f, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.CallExpr:
					if id, ok := x.Fun.(*ast.Ident); ok {
						switch {
						case id.Name == "txt" && len(x.Args) > 0:
							allLits(x.Args[0])
						case id.Name == "plural" && len(x.Args) == 3:
							lit(x.Args[1])
							lit(x.Args[2])
						}
					}
				case *ast.CompositeLit:
					if at, ok := x.Type.(*ast.ArrayType); ok { // []hint{{clé, libellé}, …}
						if id, ok := at.Elt.(*ast.Ident); ok && id.Name == "hint" {
							for _, e := range x.Elts {
								if c, ok := e.(*ast.CompositeLit); ok {
									for j, f := range c.Elts {
										if j == 1 { // libellé
											lit(f)
										} else if b, ok := f.(*ast.BasicLit); ok { // touche : seuls les mots (entrée, espace) se traduisent
											if k, _ := strconv.Unquote(b.Value); len(k) > 3 && !strings.ContainsAny(k, "/+") {
												lit(f)
											}
										}
									}
								}
							}
						}
					}
				}
				return true
			})
		}
	}
	var out []string
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// Tout texte de l'interface doit être traduit en anglais.
func TestEveryStringHasEnglish(t *testing.T) {
	var missing []string
	for _, s := range uiStrings(t) {
		if !i18n.Has(s) {
			missing = append(missing, strconv.Quote(s))
		}
	}
	if len(missing) > 0 {
		t.Errorf("%d textes sans traduction anglaise :\n%s", len(missing), strings.Join(missing, ",\n"))
	}
}

func useEnglish(t *testing.T) {
	t.Helper()
	i18n.Set(i18n.EN)
	t.Cleanup(func() { i18n.Set(i18n.FR) })
}

// L'interface tient dans le terminal aussi en anglais.
func TestViewFitsTerminalEnglish(t *testing.T) {
	useEnglish(t)
	viewFits(t)
}

func TestEnglishInterface(t *testing.T) {
	useEnglish(t)
	m := testModel(t, 160, 40)
	m.tab, m.searching = tabQueue, false
	out := ansi.Strip(m.View())
	for _, want := range []string{"1 Queue", "2 Search", "3 Library", "4 Playlists", "5 Downloads", "Up next", "Now playing", "repeat"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q absent de la vue anglaise", want)
		}
	}
	for _, fr := range []string{"File d'attente", "En lecture", "répéter", "À suivre"} {
		if strings.Contains(out, fr) {
			t.Errorf("texte français %q dans la vue anglaise", fr)
		}
	}
	if plDisplay(recentName) != "Recent" || plDisplay(favName) != "Favorites" || plDisplay("Rock") != "Rock" {
		t.Error("noms de playlists réservés mal traduits")
	}
	if got := plural(2, "piste", "pistes"); got != "2 tracks" {
		t.Errorf("pluriel : %q", got)
	}
}
