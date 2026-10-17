package main

// Instalador gráfico com cara de instalador do Mac: janela clara no centro, etapas na lateral,
// botões no canto inferior direito. Desenhado direto no framebuffer, só teclado.

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Medidas "lógicas" (multiplicadas pela escala da tela).
const (
	designW = 1024.0
	designH = 700.0
	winW    = 880.0
	winH    = 560.0
	titleH  = 44.0
	sideW   = 230.0
	padX    = 44.0
)

var (
	cWindow    = hex(0xFFFFFF)
	cSidebar   = hex(0xF5F5F7)
	cSeparator = hex(0xE3E3E8)
	cText      = hex(0x1D1D1F)
	cText2     = hex(0x6E6E73)
	cText3     = hex(0xA1A1A6)
	cAccent    = hex(0x007AFF)
	cOK        = hex(0x34C759)
	cOKText    = hex(0x1E7F34)
	cWarn      = hex(0xFF9F0A)
	cWarnText  = hex(0xA85400)
	cDanger    = hex(0xFF3B30)
	cDangerTxt = hex(0xD70015)
	cBorder    = hex(0xC7C7CC)
	cTrack     = hex(0xE5E5EA)
)

type GUI struct {
	fb     *Framebuffer
	img    *image.RGBA
	chrome *image.RGBA // fundo + janela vazia (desenhado uma vez)
	W, H   int
	s      float64
	ox, oy float64 // canto da janela, em pixels
	stage  int

	mu       sync.Mutex
	animStop chan struct{}
	animDone chan struct{}
}

// NewGUI abre a tela gráfica; erro = usar o modo texto.
func NewGUI() (*GUI, error) {
	fb, err := OpenFramebuffer("/dev/fb0")
	if err != nil {
		return nil, err
	}
	g := newGUI(fb.W, fb.H)
	g.fb = fb
	return g, nil
}

func newGUI(w, h int) *GUI {
	g := &GUI{W: w, H: h, img: image.NewRGBA(image.Rect(0, 0, w, h))}
	g.s = math.Max(0.7, math.Min(3, math.Min(float64(w)/designW, float64(h)/designH)))
	g.ox = math.Round((float64(w) - winW*g.s) / 2)
	g.oy = math.Round((float64(h) - winH*g.s) / 2)
	g.drawChrome()
	return g
}

func (g *GUI) L(v float64) float64 { return v * g.s }
func (g *GUI) X(v float64) float64 { return g.ox + v*g.s }
func (g *GUI) Y(v float64) float64 { return g.oy + v*g.s }

func (g *GUI) drawChrome() {
	c := image.NewRGBA(g.img.Bounds())
	// fundo: degradê azul → lilás, no estilo dos papéis de parede do macOS
	gradient(c, hex(0x8FB8EA), hex(0xB7C6F2), hex(0xC9D7F2), hex(0xE2CFEF))
	x, y, w, h, r := g.X(0), g.Y(0), g.L(winW), g.L(winH), g.L(12)
	shadow(c, x, y, w, h, r, g.L(26), g.L(10), 0.22)
	fillRRect(c, x-1, y-1, w+2, h+2, r+1, color.NRGBA{0, 0, 0, 40})
	fillRRect(c, x, y, w, h, r, cWindow)
	// lateral (só os cantos de baixo/esquerda arredondados: desenha e cobre o lado direito)
	fillRRect(c, x, y, g.L(sideW)+r, h, r, cSidebar)
	fillPolys(c, [][]pt{{{g.X(sideW), y + 1}, {g.X(sideW) + r + 1, y + 1}, {g.X(sideW) + r + 1, y + h - 1}, {g.X(sideW), y + h - 1}}}, cWindow)
	fillPolys(c, [][]pt{{{g.X(sideW), g.Y(titleH)}, {g.X(sideW) + 1, g.Y(titleH)}, {g.X(sideW) + 1, y + h}, {g.X(sideW), y + h}}}, cSeparator)
	// barra de título
	fillPolys(c, [][]pt{{{x, g.Y(titleH)}, {x + w, g.Y(titleH)}, {x + w, g.Y(titleH) + 1}, {x, g.Y(titleH) + 1}}}, cSeparator)
	textCentered(c, face(true, g.L(13)), x+w/2, g.Y(titleH/2+4.5), "Instalar o Backeddy Server", cText2)
	// ícone e versão
	g.drawLogo(c, g.X((sideW-76)/2), g.Y(titleH+34), g.L(76))
	textCentered(c, face(false, g.L(12)), g.X(sideW/2), g.Y(winH-22), "Versão "+version, cText3)
	g.chrome = c
}

// drawLogo: ícone próprio do Backeddy Server, na família do Backeddy for Mac (degradê laranja → rosa):
// dois discos empilhados (o espelho), cada um com sua luz verde, e uma seta de "guardar".
func (g *GUI) drawLogo(dst *image.RGBA, x, y, size float64) {
	shadow(dst, x, y, size, size, size*0.23, size*0.08, size*0.03, 0.18)
	n := 24
	for i := 0; i < n; i++ {
		t := float64(i) / float64(n-1)
		c := color.NRGBA{uint8(255 + (232-255)*t), uint8(176 + (69-176)*t), uint8(66 + (90-66)*t), 255}
		yy := y + size*float64(i)/float64(n)
		fillPolys(dst, [][]pt{clipRRectRow(x, y, size, size, size*0.23, yy, yy+size/float64(n)+0.6)}, c)
	}
	white := color.NRGBA{255, 255, 255, 255}
	soft := color.NRGBA{255, 255, 255, 235}
	led := hex(0x2BD46B)
	dw, dh := size*0.56, size*0.17
	dx := x + (size-dw)/2
	for k, dy := range []float64{y + size*0.50, y + size*0.71} {
		col := white
		if k == 1 {
			col = soft
		}
		fillRRect(dst, dx, dy, dw, dh, dh*0.32, col)
		fillCircle(dst, dx+dw-dh*0.5, dy+dh/2, dh*0.17, led)
		line(dst, pt{dx + dh*0.45, dy + dh/2}, pt{dx + dw*0.45, dy + dh/2}, dh*0.13, hex(0xF0B9A8))
	}
	// seta para baixo, entrando nos discos
	cx := x + size/2
	line(dst, pt{cx, y + size*0.15}, pt{cx, y + size*0.36}, size*0.075, white)
	fillPolys(dst, [][]pt{{{cx - size*0.12, y + size*0.32}, {cx + size*0.12, y + size*0.32}, {cx, y + size*0.45}}}, white)
}

// clipRRectRow devolve a faixa horizontal [y0,y1] de um retângulo arredondado como polígono.
func clipRRectRow(x, y, w, h, r, y0, y1 float64) []pt {
	inset := func(yy float64) float64 {
		switch {
		case yy < y+r:
			d := y + r - yy
			return r - math.Sqrt(math.Max(0, r*r-d*d))
		case yy > y+h-r:
			d := yy - (y + h - r)
			return r - math.Sqrt(math.Max(0, r*r-d*d))
		}
		return 0
	}
	y0, y1 = math.Max(y0, y), math.Min(y1, y+h)
	var left, right []pt
	const n = 6
	for i := 0; i <= n; i++ {
		yy := y0 + (y1-y0)*float64(i)/n
		in := inset(yy)
		left = append(left, pt{x + in, yy})
		right = append(right, pt{x + w - in, yy})
	}
	p := append([]pt{}, left...)
	for i := len(right) - 1; i >= 0; i-- {
		p = append(p, right[i])
	}
	return p
}

// ---------- quadro base ----------

func (g *GUI) begin() {
	copy(g.img.Pix, g.chrome.Pix)
	// etapas na lateral
	y := titleH + 34 + 76 + 46
	for i, s := range Stages {
		cy := y + float64(i)*34
		dot, col, bold := hex(0xD1D1D6), cText3, false
		switch {
		case i < g.stage:
			dot, col = hex(0x8E8E93), cText2
		case i == g.stage:
			dot, col, bold = cAccent, cText, true
		}
		fillCircle(g.img, g.X(38), g.Y(cy), g.L(4.5), dot)
		text(g.img, face(bold, g.L(14.5)), g.X(54), g.Y(cy+5), s, col)
	}
}

func (g *GUI) show() {
	if g.fb != nil {
		g.fb.Show(g.img)
	}
}

func (g *GUI) contentX() float64 { return sideW + padX }
func (g *GUI) contentW() float64 { return winW - sideW - 2*padX }

// title desenha o título (com ícone do tom) e devolve o y lógico onde começa o texto.
func (g *GUI) title(t string, tone Tone) float64 {
	x := g.contentX()
	y := titleH + 40.0
	if tone != ToneNormal {
		g.toneIcon(tone, g.X(x+15), g.Y(y+15), g.L(15))
		x += 42
	}
	f := face(true, g.L(24))
	lines := wrapPx(f, t, g.L(g.contentW()-(x-g.contentX())))
	for i, l := range lines {
		text(g.img, f, g.X(x), g.Y(y+24+float64(i)*31), l, cText)
	}
	return y + 24 + float64(len(lines)-1)*31 + 30
}

func (g *GUI) toneIcon(t Tone, cx, cy, r float64) {
	white := color.NRGBA{255, 255, 255, 255}
	switch t {
	case ToneOK:
		fillCircle(g.img, cx, cy, r, cOK)
		line(g.img, pt{cx - r*0.42, cy + r*0.02}, pt{cx - r*0.1, cy + r*0.34}, r*0.2, white)
		line(g.img, pt{cx - r*0.1, cy + r*0.34}, pt{cx + r*0.45, cy - r*0.32}, r*0.2, white)
	case ToneWarn:
		p := rrectTriangle(cx, cy, r*1.12)
		fillPolys(g.img, [][]pt{p}, cWarn)
		line(g.img, pt{cx, cy - r*0.28}, pt{cx, cy + r*0.2}, r*0.17, white)
		fillCircle(g.img, cx, cy+r*0.5, r*0.11, white)
	case ToneDanger:
		fillCircle(g.img, cx, cy, r, cDanger)
		line(g.img, pt{cx, cy - r*0.48}, pt{cx, cy + r*0.12}, r*0.19, white)
		fillCircle(g.img, cx, cy+r*0.46, r*0.12, white)
	}
}

func rrectTriangle(cx, cy, r float64) []pt {
	return []pt{{cx, cy - r}, {cx + r*1.05, cy + r*0.8}, {cx - r*1.05, cy + r*0.8}}
}

// body escreve as linhas (com marcações ! + •) a partir de y lógico; devolve o y final.
func (g *GUI) body(lines []string, y float64) float64 {
	f := face(false, g.L(15))
	fb := face(true, g.L(15))
	lh := 23.0
	for _, l := range lines {
		if l == "" {
			y += 12
			continue
		}
		col, fc := cText, f
		x, w := g.contentX(), g.contentW()
		bullet := false
		switch {
		case strings.HasPrefix(l, "!"):
			col, fc, l = cWarnText, fb, l[1:]
		case strings.HasPrefix(l, "+"):
			col, fc, l = cOKText, fb, l[1:]
		case strings.HasPrefix(l, "*"):
			fc, l = fb, l[1:]
		}
		if t := strings.TrimLeft(l, " "); strings.HasPrefix(t, "• ") {
			bullet, l = true, strings.TrimPrefix(t, "• ")
			x, w = x+20, w-20
		}
		for i, wl := range wrapPx(fc, l, g.L(w)) {
			if bullet && i == 0 {
				fillCircle(g.img, g.X(x-11), g.Y(y+12), g.L(2.6), cText2)
			}
			text(g.img, fc, g.X(x), g.Y(y+16), wl, col)
			y += lh
		}
	}
	return y
}

// buttons: o primeiro da lista é o principal e fica mais à direita (como no Mac).
func (g *GUI) buttons(btns []string, sel int) {
	f := face(true, g.L(14))
	right := winW - 28
	top := winH - 28 - 34
	for i, b := range btns {
		w := math.Max(104, textWidth(f, b)/g.s+40)
		x := right - w
		if i == sel {
			fillRRect(g.img, g.X(x)-g.L(3.5), g.Y(top)-g.L(3.5), g.L(w)+g.L(7), g.L(34)+g.L(7), g.L(10), alpha(cAccent, 0.32))
			fillRRect(g.img, g.X(x), g.Y(top), g.L(w), g.L(34), g.L(7.5), cAccent)
			textCentered(g.img, f, g.X(x+w/2), g.Y(top+22), b, cWindow)
		} else {
			fillRRect(g.img, g.X(x)-1, g.Y(top)-1, g.L(w)+2, g.L(34)+2, g.L(7.5)+1, cBorder)
			fillRRect(g.img, g.X(x), g.Y(top), g.L(w), g.L(34), g.L(7.5), cWindow)
			textCentered(g.img, f, g.X(x+w/2), g.Y(top+22), b, cText)
		}
		right = x - 14
	}
}

func (g *GUI) hint(s string) {
	text(g.img, face(false, g.L(12.5)), g.X(g.contentX()), g.Y(winH-28-12), s, cText3)
}

// ---------- telas ----------

func (g *GUI) drawChoose(title string, tone Tone, lines, btns []string, sel int) {
	g.begin()
	g.body(lines, g.title(title, tone))
	g.buttons(btns, sel)
	if len(btns) > 1 {
		g.hint("← →  escolher      Enter  confirmar")
	} else {
		g.hint("Enter  confirmar")
	}
}

func (g *GUI) drawInput(title string, lines []string, label, val string, masked bool, errMsg string) {
	g.begin()
	y := g.body(lines, g.title(title, ToneNormal)) + 14
	x := g.contentX()
	text(g.img, face(true, g.L(13)), g.X(x), g.Y(y+12), label, cText2)
	y += 22
	fw, fh := 360.0, 38.0
	fillRRect(g.img, g.X(x)-g.L(3.5), g.Y(y)-g.L(3.5), g.L(fw+7), g.L(fh+7), g.L(10), alpha(cAccent, 0.30))
	fillRRect(g.img, g.X(x)-1, g.Y(y)-1, g.L(fw)+2, g.L(fh)+2, g.L(8)+1, cAccent)
	fillRRect(g.img, g.X(x), g.Y(y), g.L(fw), g.L(fh), g.L(8), cWindow)
	shown := val
	f := face(false, g.L(16))
	if masked {
		shown = strings.Repeat("•", utf8.RuneCountInString(val))
	}
	// se o texto não couber, mostra o final
	for textWidth(f, shown) > g.L(fw-30) && shown != "" {
		_, size := utf8.DecodeRuneInString(shown)
		shown = shown[size:]
	}
	text(g.img, f, g.X(x+12), g.Y(y+25), shown, cText)
	cx := g.X(x+12) + textWidth(f, shown) + g.L(1.5)
	line(g.img, pt{cx, g.Y(y + 10)}, pt{cx, g.Y(y + 28)}, g.L(1.8), cAccent)
	if errMsg != "" {
		text(g.img, face(true, g.L(13)), g.X(x), g.Y(y+fh+24), errMsg, cDangerTxt)
	}
	g.buttons([]string{"Continuar"}, 0)
	g.hint("Digite e aperte Enter      Backspace apaga")
}

func (g *GUI) drawProgress(title string, steps []string, current int, note string, tick int) {
	g.begin()
	y := g.title(title, ToneNormal) + 10
	x, w := g.contentX(), g.contentW()
	step := ""
	if current < len(steps) {
		step = steps[current]
	} else {
		step = "Concluído"
	}
	// spinner de raios (como o do Mac) ao lado da etapa atual
	if current < len(steps) {
		g.spinner(g.X(x+9), g.Y(y+10), g.L(10), tick)
		text(g.img, face(true, g.L(15)), g.X(x+28), g.Y(y+15), step, cText)
	} else {
		g.toneIcon(ToneOK, g.X(x+9), g.Y(y+10), g.L(9))
		text(g.img, face(true, g.L(15)), g.X(x+28), g.Y(y+15), step, cText)
	}
	y += 34
	frac := math.Min(1, float64(current)/float64(len(steps)))
	fillRRect(g.img, g.X(x), g.Y(y), g.L(w), g.L(8), g.L(4), cTrack)
	if frac > 0 {
		fillRRect(g.img, g.X(x), g.Y(y), math.Max(g.L(8), g.L(w)*frac), g.L(8), g.L(4), cAccent)
	}
	y += 30
	if current < len(steps) {
		text(g.img, face(false, g.L(13)), g.X(x), g.Y(y), "Etapa "+itoa(current+1)+" de "+itoa(len(steps)), cText2)
	}
	if note != "" {
		g.body([]string{note}, y+22)
	}
}

func (g *GUI) spinner(cx, cy, r float64, tick int) {
	const n = 12
	for i := 0; i < n; i++ {
		a := 2*math.Pi*float64(i)/n - math.Pi/2
		age := (tick - i + n*1000) % n // o raio "mais novo" é o mais escuro
		al := 0.18 + 0.82*(1-float64(age)/n)
		in, out := pt{cx + r*0.45*math.Cos(a), cy + r*0.45*math.Sin(a)}, pt{cx + r*math.Cos(a), cy + r*math.Sin(a)}
		line(g.img, in, out, r*0.21, alpha(cText2, al))
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

// ---------- Frontend ----------

func (g *GUI) stopAnim() {
	if g.animStop != nil {
		close(g.animStop)
		<-g.animDone
		g.animStop = nil
	}
}

func (g *GUI) Stage(i int) { g.stage = i }

func (g *GUI) Choose(title string, tone Tone, lines []string, btns []string, def int, abort <-chan struct{}) int {
	g.stopAnim()
	sel := def
	for {
		g.mu.Lock()
		g.drawChoose(title, tone, lines, btns, sel)
		g.show()
		g.mu.Unlock()
		e, ok := nextKey(abort)
		if !ok {
			return -1
		}
		switch e.k {
		case kLeft: // botões aparecem em ordem inversa (principal à direita)
			sel = (sel + 1) % len(btns)
		case kRight:
			sel = (sel - 1 + len(btns)) % len(btns)
		case kEnter:
			return sel
		}
	}
}

func (g *GUI) Input(title string, lines []string, label, initial string, masked bool) string {
	g.stopAnim()
	val, errMsg := initial, ""
	for {
		g.mu.Lock()
		g.drawInput(title, lines, label, val, masked, errMsg)
		g.show()
		g.mu.Unlock()
		e, ok := nextKey(nil)
		if !ok {
			return val
		}
		var done bool
		if val, done, errMsg = editValue(val, e); done {
			return val
		}
	}
}

// Progress desenha a etapa atual e mantém o spinner girando até a próxima chamada.
func (g *GUI) Progress(title string, steps []string, current int, note string) {
	g.stopAnim()
	g.mu.Lock()
	g.drawProgress(title, steps, current, note, 0)
	g.show()
	g.mu.Unlock()
	if current >= len(steps) || g.fb == nil {
		return
	}
	stop, done := make(chan struct{}), make(chan struct{})
	g.animStop, g.animDone = stop, done
	go func() {
		defer close(done)
		t := time.NewTicker(90 * time.Millisecond)
		defer t.Stop()
		for tick := 1; ; tick++ {
			select {
			case <-stop:
				return
			case <-t.C:
				g.mu.Lock()
				g.drawProgress(title, steps, current, note, tick)
				g.show()
				g.mu.Unlock()
			}
		}
	}()
}

func (g *GUI) Notice(title string, tone Tone, lines []string) {
	g.stopAnim()
	g.mu.Lock()
	g.begin()
	g.body(lines, g.title(title, tone))
	g.show()
	g.mu.Unlock()
}

func (g *GUI) Close() {
	g.stopAnim()
	rawOff()
	if g.fb != nil {
		g.fb.Close()
		g.fb = nil
	}
}

// ---------- capturas de tela (desenvolvimento) ----------

func (g *GUI) snapshot() *image.RGBA {
	out := image.NewRGBA(g.img.Bounds())
	draw.Draw(out, out.Bounds(), g.img, image.Point{}, draw.Src)
	return out
}
