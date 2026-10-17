package main

// Interface do instalador. Duas caras para o mesmo roteiro:
//   - GUI (gui.go): tela gráfica desenhada direto no framebuffer, com cara de instalador do Mac;
//   - TextUI (este arquivo): modo texto, reserva para PCs sem vídeo compatível.
// Só teclado: setas, Tab e Enter.
//
// Marcações nas linhas de texto: "!" no começo = aviso, "+" = boa notícia, "*" = destaque, "• " = item de lista.

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"unicode/utf8"
)

type Tone int

const (
	ToneNormal Tone = iota
	ToneOK
	ToneWarn
	ToneDanger
)

// Etapas mostradas na lateral do instalador gráfico.
var Stages = []string{"Introdução", "Discos", "Confirmação", "Nome e senha", "Instalação", "Resumo"}

const (
	StageIntro = iota
	StageDisks
	StageConfirm
	StageName
	StageInstall
	StageDone
)

type Frontend interface {
	Stage(i int)
	// Choose devolve o botão escolhido, ou -1 se abort disparar antes.
	Choose(title string, tone Tone, lines []string, btns []string, def int, abort <-chan struct{}) int
	Input(title string, lines []string, label, initial string, masked bool) string
	Progress(title string, steps []string, current int, note string)
	// Notice mostra uma mensagem sem esperar tecla.
	Notice(title string, tone Tone, lines []string)
	// Cards: escolha entre cartões (↑↓) e botões (←→). Devolve o cartão marcado e o botão apertado.
	Cards(title string, lines []string, cards []Card, sel int, btns []string) (card, btn int)
	// Manual: papel de cada disco + espelho. ok=false = Voltar.
	Manual(title string, lines []string, disks []Disk, roles []Role, mirror bool,
		preview func([]Role, bool) ([]string, bool)) ([]Role, bool, bool)
	Close()
}

type Card struct {
	Title, Value, Desc string
	Risk               Risk
	Recommended        bool
	Plain              bool // sem selo de risco (escolhas que não envolvem risco)
}

func riskLabel(r Risk) string {
	switch r {
	case RiskGreen:
		return T("Protegido")
	case RiskYellow:
		return T("Sem proteção")
	}
	return T("Desaconselhado")
}

// nextRole: próximo papel permitido (dir = +1 ou -1).
func nextRole(d Disk, cur Role, dir int) Role {
	al := AllowedRoles(d)
	for i, r := range al {
		if r == cur {
			return al[(i+dir+len(al))%len(al)]
		}
	}
	return al[0]
}

// ---------- teclado (compartilhado pelas duas interfaces) ----------

type key int

const (
	kOther key = iota
	kEnter
	kLeft
	kRight
	kBack
	kChar
	kUp
	kDown
	kEsc
	kTab
)

type keyEvent struct {
	k key
	s string
}

var (
	keysOnce sync.Once
	keys     chan keyEvent
	rawMode  bool
)

func rawOn() {
	if !rawMode {
		_ = exec.Command("stty", "-F", "/dev/tty", "raw", "-echo").Run()
		rawMode = true
	}
}

func rawOff() {
	if rawMode {
		_ = exec.Command("stty", "-F", "/dev/tty", "sane").Run()
		rawMode = false
	}
}

// startKeys lê o teclado numa goroutine, para poder esperar tecla OU outro evento (pendrive retirado).
func startKeys() {
	keysOnce.Do(func() {
		rawOn()
		keys = make(chan keyEvent, 16)
		go func() {
			buf := make([]byte, 16)
			for {
				n, err := os.Stdin.Read(buf)
				if err != nil {
					close(keys)
					return
				}
				if n > 0 {
					k, s := parseKey(buf[:n])
					keys <- keyEvent{k, s}
				}
			}
		}()
	})
}

func parseKey(b []byte) (key, string) {
	n := len(b)
	switch {
	case b[0] == '\r' || b[0] == '\n':
		return kEnter, ""
	case b[0] == '\t':
		return kTab, ""
	case n == 1 && b[0] == 0x1b:
		return kEsc, ""
	case b[0] == 127 || b[0] == 8:
		return kBack, ""
	case n >= 3 && b[0] == 0x1b && b[1] == '[':
		switch b[2] {
		case 'A':
			return kUp, ""
		case 'B':
			return kDown, ""
		case 'C':
			return kRight, ""
		case 'D':
			return kLeft, ""
		}
		return kOther, ""
	case b[0] >= 32 && b[0] != 0x1b:
		return kChar, string(b)
	}
	return kOther, ""
}

// horiz: nas telas só com botões, cima/baixo/Tab também trocam o botão.
func horiz(k key) key {
	switch k {
	case kUp:
		return kLeft
	case kDown, kTab:
		return kRight
	}
	return k
}

// nextKey espera uma tecla; ok=false se abort disparou (ou o teclado sumiu).
func nextKey(abort <-chan struct{}) (keyEvent, bool) {
	startKeys()
	select {
	case e, ok := <-keys:
		return e, ok
	case <-abort:
		return keyEvent{}, false
	}
}

// editValue aplica uma tecla a um campo de texto. done=true quando Enter com valor válido.
func editValue(val string, e keyEvent) (string, bool, string) {
	switch e.k {
	case kEnter:
		if strings.TrimSpace(val) == "" {
			return val, false, T("Não pode ficar em branco.")
		}
		return val, true, ""
	case kBack:
		if val != "" {
			_, size := utf8.DecodeLastRuneInString(val)
			val = val[:len(val)-size]
		}
	case kChar:
		if utf8.RuneCountInString(val) < 60 {
			val += e.s
		}
	}
	return val, false, ""
}

// ---------- modo texto ----------

const (
	esc     = "\x1b["
	reset   = esc + "0m"
	bold    = esc + "1m"
	reverse = esc + "7m"
	red     = esc + "31m"
	green   = esc + "32m"
	yellow  = esc + "33m"
	cyan    = esc + "36m"
)

type TextUI struct {
	cols, rows int
}

func NewTextUI() *TextUI {
	// letras grandes no console do PC (no modo gráfico isso não é usado)
	if os.Getenv("TERM") == "linux" {
		_ = exec.Command("setfont", "/usr/share/consolefonts/Uni3-TerminusBold32x16.psf.gz").Run()
	}
	u := &TextUI{cols: 80, rows: 25}
	if out, err := exec.Command("stty", "-F", "/dev/tty", "size").Output(); err == nil {
		var r, c int
		if _, err := fmt.Sscanf(string(out), "%d %d", &r, &c); err == nil && c > 20 {
			u.rows, u.cols = r, c
		}
	}
	return u
}

func (u *TextUI) Stage(int) {}

func (u *TextUI) Close() {
	rawOff()
	fmt.Print(esc + "?25h")
}

func toneColor(t Tone) string {
	switch t {
	case ToneOK:
		return green
	case ToneWarn:
		return yellow
	case ToneDanger:
		return red
	}
	return bold
}

func (u *TextUI) width() int {
	w := u.cols - 6
	if w > 90 {
		w = 90
	}
	return w
}

func (u *TextUI) left() string { return strings.Repeat(" ", (u.cols-u.width())/2) }

// p escreve uma linha (em modo raw, \n precisa de \r).
func (u *TextUI) p(s string) { fmt.Print(u.left() + s + "\r\n") }

func (u *TextUI) header(title string, t Tone) {
	fmt.Print(esc + "2J" + esc + "H" + esc + "?25l")
	fmt.Print("\r\n")
	u.p(cyan + bold + "Backeddy Server" + reset)
	u.p(cyan + strings.Repeat("─", u.width()) + reset)
	fmt.Print("\r\n")
	for _, w := range wrap(title, u.width()) {
		u.p(toneColor(t) + bold + w + reset)
	}
	fmt.Print("\r\n")
}

func (u *TextUI) body(lines []string) {
	for _, l := range lines {
		if l == "" {
			fmt.Print("\r\n")
			continue
		}
		prefix := ""
		switch {
		case strings.HasPrefix(l, "!"):
			prefix, l = yellow, l[1:]
		case strings.HasPrefix(l, "+"):
			prefix, l = green, l[1:]
		case strings.HasPrefix(l, "*"):
			prefix, l = bold, l[1:]
		}
		for _, w := range wrap(l, u.width()) {
			u.p(prefix + w + reset)
		}
	}
}

func (u *TextUI) buttons(btns []string, sel int) {
	fmt.Print("\r\n")
	var b strings.Builder
	for i, t := range btns {
		if i > 0 {
			b.WriteString("    ")
		}
		if i == sel {
			b.WriteString(reverse + bold + "  " + t + "  " + reset)
		} else {
			b.WriteString("[ " + t + " ]")
		}
	}
	u.p(b.String())
	fmt.Print("\r\n")
	u.p(esc + "2m" + T("Setas para escolher · Enter para confirmar") + reset)
}

func (u *TextUI) Choose(title string, t Tone, lines []string, btns []string, def int, abort <-chan struct{}) int {
	sel := def
	for {
		u.header(title, t)
		u.body(lines)
		u.buttons(btns, sel)
		e, ok := nextKey(abort)
		if !ok {
			return -1
		}
		switch horiz(e.k) {
		case kLeft:
			sel = (sel - 1 + len(btns)) % len(btns)
		case kRight:
			sel = (sel + 1) % len(btns)
		case kEnter:
			return sel
		}
	}
}

func (u *TextUI) Input(title string, lines []string, label, initial string, masked bool) string {
	val, errMsg := initial, ""
	for {
		u.header(title, ToneNormal)
		u.body(lines)
		fmt.Print("\r\n")
		shown := val
		if masked {
			shown = strings.Repeat("•", utf8.RuneCountInString(val))
		}
		u.p(label + " " + reverse + " " + shown + "_ " + reset)
		if errMsg != "" {
			fmt.Print("\r\n")
			u.p(red + errMsg + reset)
		}
		fmt.Print("\r\n")
		u.p(esc + "2m" + T("Digite e aperte Enter · Backspace apaga") + reset)
		e, ok := nextKey(nil)
		if !ok {
			return val
		}
		var done bool
		val, done, errMsg = editValue(val, e)
		if done {
			return val
		}
	}
}

func (u *TextUI) Progress(title string, steps []string, current int, note string) {
	u.header(title, ToneNormal)
	for i, s := range steps {
		switch {
		case i < current:
			u.p(green + "  ✓ " + reset + s)
		case i == current:
			u.p(cyan + bold + "  ▸ " + s + "…" + reset)
		default:
			u.p("    " + esc + "2m" + s + reset)
		}
	}
	if note != "" {
		fmt.Print("\r\n")
		u.body([]string{note})
	}
}

func (u *TextUI) Notice(title string, t Tone, lines []string) {
	u.header(title, t)
	u.body(lines)
}

func (u *TextUI) Cards(title string, lines []string, cards []Card, sel int, btns []string) (int, int) {
	b := 0
	for {
		u.header(title, ToneNormal)
		u.body(lines)
		fmt.Print("\r\n")
		for i, c := range cards {
			mark, st := "   ", ""
			if i == sel {
				mark, st = cyan+bold+" ▸ ", bold
			}
			col := green
			if c.Risk == RiskYellow {
				col = yellow
			} else if c.Risk == RiskRed {
				col = red
			}
			rec := ""
			if c.Recommended {
				rec = T(" (recomendado)")
			}
			tag := "  " + col + "[" + riskLabel(c.Risk) + "]" + reset
			if c.Plain {
				tag = ""
			}
			u.p(mark + st + c.Title + rec + reset + "  " + bold + c.Value + reset + tag)
			for _, w := range wrap(c.Desc, u.width()-6) {
				u.p("      " + esc + "2m" + w + reset)
			}
		}
		u.buttons(btns, b)
		u.p(esc + "2m" + T("↑↓ escolhe o modo") + reset)
		e, ok := nextKey(nil)
		if !ok {
			return sel, 0
		}
		switch e.k {
		case kUp:
			sel = (sel - 1 + len(cards)) % len(cards)
		case kDown:
			sel = (sel + 1) % len(cards)
		case kLeft:
			b = (b - 1 + len(btns)) % len(btns)
		case kRight, kTab:
			b = (b + 1) % len(btns)
		case kEnter:
			return sel, b
		}
	}
}

func (u *TextUI) Manual(title string, lines []string, disks []Disk, roles []Role, mirror bool,
	preview func([]Role, bool) ([]string, bool)) ([]Role, bool, bool) {
	roles = append([]Role{}, roles...)
	row := 0
	for {
		u.header(title, ToneNormal)
		u.body(lines)
		fmt.Print("\r\n")
		for i, d := range disks {
			mark := "   "
			if i == row {
				mark = cyan + bold + " ▸ " + reset
			}
			u.p(mark + d.Label() + d.EraseNote() + "  " + reverse + " " + roles[i].String() + " " + reset)
		}
		mark := "   "
		if row == len(disks) {
			mark = cyan + bold + " ▸ " + reset
		}
		mv := T("Não")
		if mirror {
			mv = T("Sim")
		}
		u.p(mark + T("Espelhar os backups (cada arquivo em 2 discos)  ") + reverse + " " + mv + " " + reset)
		pl, valid := preview(roles, mirror)
		fmt.Print("\r\n")
		u.body(pl)
		fmt.Print("\r\n")
		u.p(esc + "2m" + T("↑↓ escolhe · ←→ muda · Enter continua · Esc volta") + reset)
		e, ok := nextKey(nil)
		if !ok {
			return roles, mirror, false
		}
		switch e.k {
		case kUp:
			row = (row - 1 + len(disks) + 1) % (len(disks) + 1)
		case kDown, kTab:
			row = (row + 1) % (len(disks) + 1)
		case kLeft, kRight:
			dir := 1
			if e.k == kLeft {
				dir = -1
			}
			if row < len(disks) {
				roles[row] = nextRole(disks[row], roles[row], dir)
			} else {
				mirror = !mirror
			}
		case kEnter:
			if valid {
				return roles, mirror, true
			}
		case kEsc:
			return roles, mirror, false
		}
	}
}

func wrap(s string, width int) []string {
	// recuo do começo da linha (itens de lista) é mantido nas linhas seguintes
	indent := len(s) - len(strings.TrimLeft(s, " "))
	pad := strings.Repeat(" ", indent)
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	cur := ""
	for _, w := range words {
		if cur == "" {
			cur = pad + w
		} else if utf8.RuneCountInString(cur)+1+utf8.RuneCountInString(w) <= width {
			cur += " " + w
		} else {
			lines = append(lines, cur)
			cur = pad + w
		}
	}
	return append(lines, cur)
}
