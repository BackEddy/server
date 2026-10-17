package main

// Interface de texto simples para o console do PC: letras grandes (fonte Terminus 32),
// só setas, Tab e Enter. Sem bibliotecas externas.

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"unicode/utf8"
)

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

type UI struct {
	cols, rows int
	raw        bool
}

func NewUI() *UI {
	u := &UI{cols: 80, rows: 25}
	if out, err := exec.Command("stty", "-F", "/dev/tty", "size").Output(); err == nil {
		var r, c int
		if _, err := fmt.Sscanf(string(out), "%d %d", &r, &c); err == nil && c > 20 {
			u.rows, u.cols = r, c
		}
	}
	return u
}

func (u *UI) rawOn() {
	if !u.raw {
		_ = exec.Command("stty", "-F", "/dev/tty", "raw", "-echo").Run()
		u.raw = true
	}
}

func (u *UI) Close() {
	if u.raw {
		_ = exec.Command("stty", "-F", "/dev/tty", "sane").Run()
		u.raw = false
	}
	fmt.Print(esc + "?25h")
}

func (u *UI) width() int {
	w := u.cols - 6
	if w > 90 {
		w = 90
	}
	return w
}

func (u *UI) left() string { return strings.Repeat(" ", (u.cols-u.width())/2) }

// p escreve uma linha (em modo raw, \n precisa de \r).
func (u *UI) p(s string) { fmt.Print(u.left() + s + "\r\n") }

func (u *UI) header(title, color string) {
	fmt.Print(esc + "2J" + esc + "H" + esc + "?25l")
	fmt.Print("\r\n")
	u.p(cyan + bold + "Backeddy Server" + reset)
	u.p(cyan + strings.Repeat("─", u.width()) + reset)
	fmt.Print("\r\n")
	u.p(color + bold + title + reset)
	fmt.Print("\r\n")
}

func (u *UI) body(lines []string) {
	for _, l := range lines {
		if l == "" {
			fmt.Print("\r\n")
			continue
		}
		prefix := ""
		if strings.HasPrefix(l, "!") { // linha de aviso
			prefix, l = yellow, l[1:]
		}
		for _, w := range wrap(l, u.width()) {
			u.p(prefix + w + reset)
		}
	}
}

func (u *UI) buttons(btns []string, sel int) {
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
	u.p(esc + "2m" + "Setas para escolher · Enter para confirmar" + reset)
}

type key int

const (
	kOther key = iota
	kEnter
	kLeft
	kRight
	kBack
	kChar
)

func readKey() (key, string) {
	buf := make([]byte, 16)
	n, err := os.Stdin.Read(buf)
	if err != nil || n == 0 {
		return kOther, ""
	}
	b := buf[:n]
	switch {
	case b[0] == '\r' || b[0] == '\n':
		return kEnter, ""
	case b[0] == '\t':
		return kRight, ""
	case b[0] == 127 || b[0] == 8:
		return kBack, ""
	case n >= 3 && b[0] == 0x1b && b[1] == '[':
		switch b[2] {
		case 'C', 'B':
			return kRight, ""
		case 'D', 'A':
			return kLeft, ""
		}
		return kOther, ""
	case b[0] >= 32 && b[0] != 0x1b:
		return kChar, string(b)
	}
	return kOther, ""
}

// Choose mostra uma tela com botões e devolve o índice escolhido.
func (u *UI) Choose(title, color string, lines []string, btns []string, def int) int {
	u.rawOn()
	sel := def
	for {
		u.header(title, color)
		u.body(lines)
		u.buttons(btns, sel)
		k, _ := readKey()
		switch k {
		case kLeft:
			sel = (sel - 1 + len(btns)) % len(btns)
		case kRight:
			sel = (sel + 1) % len(btns)
		case kEnter:
			return sel
		}
	}
}

// Input pede um texto. masked esconde o que é digitado (senhas).
func (u *UI) Input(title string, lines []string, label, initial string, masked bool) string {
	u.rawOn()
	val := initial
	errMsg := ""
	for {
		u.header(title, bold)
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
		u.p(esc + "2m" + "Digite e aperte Enter · Backspace apaga" + reset)
		k, s := readKey()
		switch k {
		case kEnter:
			if strings.TrimSpace(val) == "" {
				errMsg = "Não pode ficar em branco."
				continue
			}
			return val
		case kBack:
			if val != "" {
				_, size := utf8.DecodeLastRuneInString(val)
				val = val[:len(val)-size]
			}
		case kChar:
			if utf8.RuneCountInString(val) < 60 {
				val += s
			}
		}
		errMsg = ""
	}
}

// Progress desenha a lista de etapas (não espera tecla).
func (u *UI) Progress(title string, steps []string, current int, note string) {
	u.header(title, bold)
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

func wrap(s string, width int) []string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	cur := ""
	for _, w := range words {
		if cur == "" {
			cur = w
		} else if utf8.RuneCountInString(cur)+1+utf8.RuneCountInString(w) <= width {
			cur += " " + w
		} else {
			lines = append(lines, cur)
			cur = w
		}
	}
	return append(lines, cur)
}
