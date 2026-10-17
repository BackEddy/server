package main

// Idiomas do instalador: português (padrão), inglês e espanhol. A primeira tela pergunta o idioma.
// Os textos ficam em português no código; T() troca pela tradução do catálogo (i18n_catalog.go).
// Telas montadas com texto fixo são traduzidas na entrada da interface (trUI); textos montados com
// valores (nomes de discos, tamanhos) passam por T() nas partes fixas, onde são criados.

import "strings"

const (
	LangPT = iota
	LangEN
	LangES
)

// LangCodes: nome curto de cada idioma, gravado no servidor (server.conf: LANG=pt|en|es).
var LangCodes = []string{"pt", "en", "es"}

var lang = LangPT

// T traduz um texto do instalador. Marcas de linha (•, !, +, *) no começo são mantidas.
func T(s string) string {
	if lang == LangPT || s == "" {
		return s
	}
	if v, ok := catalog[s]; ok {
		return v[lang-1]
	}
	for _, p := range []string{"• ", "!", "+", "*"} {
		if rest, ok := strings.CutPrefix(s, p); ok {
			if v, ok := catalog[rest]; ok {
				return p + v[lang-1]
			}
		}
	}
	return s
}

func tAll(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = T(s)
	}
	return out
}

// askLanguage: primeira tela, nos três idiomas.
func askLanguage(ui Frontend) {
	c, _ := ui.Cards("Idioma · Language · Idioma", []string{
		"Escolha o idioma da instalação e do servidor.",
		"Choose the language for setup and for the server.",
		"Elige el idioma de la instalación y del servidor.",
	}, []Card{
		{Title: "Português", Plain: true},
		{Title: "English", Plain: true},
		{Title: "Español", Plain: true},
	}, lang, []string{"OK"})
	if c >= 0 && c < len(LangCodes) {
		lang = c
	}
}

// trUI traduz tudo o que chega à interface.
type trUI struct{ Frontend }

func (t trUI) Choose(title string, tone Tone, lines []string, btns []string, def int, abort <-chan struct{}) int {
	return t.Frontend.Choose(T(title), tone, tAll(lines), tAll(btns), def, abort)
}

func (t trUI) Input(title string, lines []string, label, initial string, masked bool) string {
	return t.Frontend.Input(T(title), tAll(lines), T(label), initial, masked)
}

func (t trUI) Progress(title string, steps []string, current int, note string) {
	t.Frontend.Progress(T(title), tAll(steps), current, T(note))
}

func (t trUI) Notice(title string, tone Tone, lines []string) {
	t.Frontend.Notice(T(title), tone, tAll(lines))
}

func (t trUI) Cards(title string, lines []string, cards []Card, sel int, btns []string) (int, int) {
	cs := make([]Card, len(cards))
	for i, c := range cards {
		c.Title, c.Desc, c.Value = T(c.Title), T(c.Desc), T(c.Value)
		cs[i] = c
	}
	return t.Frontend.Cards(T(title), tAll(lines), cs, sel, tAll(btns))
}

func (t trUI) Manual(title string, lines []string, disks []Disk, roles []Role, mirror bool,
	preview func([]Role, bool) ([]string, bool)) ([]Role, bool, bool) {
	return t.Frontend.Manual(T(title), tAll(lines), disks, roles, mirror, func(r []Role, m bool) ([]string, bool) {
		out, ok := preview(r, m)
		return tAll(out), ok
	})
}
