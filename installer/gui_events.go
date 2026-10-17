package main

// Eventos do instalador gráfico (teclado + mouse), cursor e as telas de cartões e Manual.

import (
	"image"
	"image/color"
	"math"
	"strings"

	"golang.org/x/image/font"
)

type evKind int

const (
	evKey evKind = iota
	evClick
	evHover // o mouse entrou ou saiu de um botão: redesenhar
	evAbort
)

type uiEvent struct {
	kind evKind
	key  keyEvent
	id   int
}

func (g *GUI) hitAt(x, y int) int {
	p := image.Pt(x, y)
	for i := len(g.hits) - 1; i >= 0; i-- {
		if p.In(g.hits[i].r) {
			return g.hits[i].id
		}
	}
	return 0
}

// wait espera tecla, clique, mudança de destaque do mouse ou abort.
func (g *GUI) wait(abort <-chan struct{}) uiEvent {
	startKeys()
	var moved <-chan struct{}
	var clicks <-chan [2]int
	if g.mouse != nil {
		moved, clicks = g.mouse.moved, g.mouse.clicks
	}
	for {
		select {
		case e, ok := <-keys:
			if !ok {
				return uiEvent{kind: evAbort}
			}
			return uiEvent{kind: evKey, key: e}
		case <-abort:
			return uiEvent{kind: evAbort}
		case c := <-clicks:
			g.mu.Lock()
			id := g.hitAt(c[0], c[1])
			g.mu.Unlock()
			if id != 0 {
				return uiEvent{kind: evClick, id: id}
			}
		case <-moved:
			x, y, _ := g.mouse.pos()
			g.mu.Lock()
			h := g.hitAt(x, y)
			if h != g.hover {
				g.hover = h
				g.mu.Unlock()
				return uiEvent{kind: evHover}
			}
			g.moveCursor(x, y)
			g.mu.Unlock()
		}
	}
}

// present mostra o quadro desenhado (guardando uma cópia sem o cursor) e o cursor por cima.
func (g *GUI) present() {
	if g.frame == nil {
		g.frame = image.NewRGBA(g.img.Bounds())
	}
	copy(g.frame.Pix, g.img.Pix)
	g.curShown = false
	if g.mouse != nil {
		if x, y, ok := g.mouse.pos(); ok {
			g.curRect = g.drawCursor(x, y)
			g.curShown = true
		}
	}
	g.show()
}

// moveCursor redesenha só a área do cursor (rápido).
func (g *GUI) moveCursor(x, y int) {
	if g.frame == nil {
		return
	}
	old := g.curRect
	restore(g.img, g.frame, old)
	g.curRect = g.drawCursor(x, y)
	g.curShown = true
	if g.fb != nil {
		g.fb.ShowRect(g.img, old.Union(g.curRect))
	}
}

func restore(dst, src *image.RGBA, r image.Rectangle) {
	r = r.Intersect(dst.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		o := y*dst.Stride + r.Min.X*4
		copy(dst.Pix[o:o+r.Dx()*4], src.Pix[o:o+r.Dx()*4])
	}
}

// drawCursor: seta clássica (branca com contorno escuro); devolve a área ocupada.
func (g *GUI) drawCursor(x, y int) image.Rectangle {
	k := math.Max(1, g.s) * 1.15
	shape := []pt{{0, 0}, {0, 17}, {4.2, 13.2}, {7.2, 19.6}, {10, 18.4}, {7.1, 12.1}, {12.4, 12.1}}
	px, py := float64(x), float64(y)
	outer := make([]pt, len(shape))
	for i, p := range shape {
		outer[i] = pt{px + p.X*k, py + p.Y*k}
	}
	fillPolys(g.img, [][]pt{outer}, color.NRGBA{20, 20, 20, 255})
	// miolo branco: a mesma forma, encolhida em direção a um ponto interno
	cx, cy := px+4.2*k, py+11*k
	inner := make([]pt, len(outer))
	for i, p := range outer {
		inner[i] = pt{cx + (p.X-cx)*0.78, cy + (p.Y-cy)*0.84}
	}
	fillPolys(g.img, [][]pt{inner}, color.NRGBA{255, 255, 255, 255})
	return image.Rect(x-2, y-2, x+int(14*k)+3, y+int(21*k)+3)
}

// ---------- Cartões (modos de uso dos discos) ----------

func riskColors(r Risk) (bg, fg color.NRGBA) {
	switch r {
	case RiskGreen:
		return hex(0xE3F6E8), cOKText
	case RiskYellow:
		return hex(0xFFF1DC), cWarnText
	}
	return hex(0xFFE5E3), cDangerTxt
}

func (g *GUI) pill(x, y float64, s string, bg, fg color.NRGBA) float64 {
	f := face(true, g.L(11.5))
	w := textWidth(f, s)/g.s + 16
	fillRRect(g.img, g.X(x), g.Y(y), g.L(w), g.L(20), g.L(10), bg)
	text(g.img, f, g.X(x+8), g.Y(y+14), s, fg)
	return w
}

// ellipsize corta o texto com "…" para caber em maxW pixels.
func ellipsize(f font.Face, s string, maxW float64) string {
	if textWidth(f, s) <= maxW {
		return s
	}
	r := []rune(s)
	for len(r) > 1 && textWidth(f, string(r)+"…") > maxW {
		r = r[:len(r)-1]
	}
	return strings.TrimSpace(string(r)) + "…"
}

func (g *GUI) drawCards(title string, lines []string, cards []Card, sel int, btns []string, bsel int) {
	g.begin()
	y := g.body(lines, g.title(title, ToneNormal)) + 6
	x, w := g.contentX(), g.contentW()
	ch := 60.0
	for i, c := range cards {
		hov := g.hover == hitCard+i
		g.addHit(x, y, w, ch, hitCard+i)
		border, bg := cSeparator, cWindow
		switch {
		case i == sel:
			border, bg = cAccent, hex(0xF2F7FF)
		case hov:
			border, bg = hex(0xC7C7CC), hex(0xF8F8FA)
		}
		bw := 1.0
		if i == sel {
			bw = 2
		}
		fillRRect(g.img, g.X(x)-bw, g.Y(y)-bw, g.L(w)+2*bw, g.L(ch)+2*bw, g.L(10)+bw, border)
		fillRRect(g.img, g.X(x), g.Y(y), g.L(w), g.L(ch), g.L(10), bg)
		// botão de rádio
		rcx, rcy := x+22, y+21
		if i == sel {
			fillCircle(g.img, g.X(rcx), g.Y(rcy), g.L(8), cAccent)
			fillCircle(g.img, g.X(rcx), g.Y(rcy), g.L(3.2), cWindow)
		} else {
			fillCircle(g.img, g.X(rcx), g.Y(rcy), g.L(8), cBorder)
			fillCircle(g.img, g.X(rcx), g.Y(rcy), g.L(6.6), cWindow)
		}
		ft := face(true, g.L(15))
		text(g.img, ft, g.X(x+42), g.Y(y+26), c.Title, cText)
		px := x + 42 + textWidth(ft, c.Title)/g.s + 10
		if c.Recommended {
			px += g.pill(px, y+11, T("Recomendado"), hex(0xE5F0FF), cAccent) + 6
		}
		if !c.Plain && c.Title != riskLabel(c.Risk) {
			rb, rf := riskColors(c.Risk)
			g.pill(px, y+11, riskLabel(c.Risk), rb, rf)
		}
		fv := face(true, g.L(17))
		text(g.img, fv, g.X(x+w-16)-textWidth(fv, c.Value), g.Y(y+27), c.Value, cText)
		fd := face(false, g.L(12.5))
		text(g.img, fd, g.X(x+42), g.Y(y+47), ellipsize(fd, c.Desc, g.L(w-58)), cText2)
		y += ch + 9
	}
	g.buttons(btns, bsel)
	g.hint(T("Clique, ou ↑↓ modo · ← → botão · Enter"))
}

func (g *GUI) Cards(title string, lines []string, cards []Card, sel int, btns []string) (int, int) {
	g.stopAnim()
	b := 0
	for {
		g.mu.Lock()
		g.drawCards(title, lines, cards, sel, btns, b)
		g.present()
		g.mu.Unlock()
		ev := g.wait(nil)
		switch ev.kind {
		case evAbort:
			return sel, 0
		case evClick:
			switch {
			case ev.id >= hitCard && ev.id < hitCard+len(cards):
				sel = ev.id - hitCard
			case ev.id >= hitBtn && ev.id < hitBtn+len(btns):
				return sel, ev.id - hitBtn
			}
		case evKey:
			switch ev.key.k {
			case kUp:
				sel = (sel - 1 + len(cards)) % len(cards)
			case kDown:
				sel = (sel + 1) % len(cards)
			case kLeft:
				b = (b + 1) % len(btns)
			case kRight, kTab:
				b = (b - 1 + len(btns)) % len(btns)
			case kEnter:
				return sel, b
			}
		}
	}
}

// ---------- Manual (papel de cada disco) ----------

func (g *GUI) drawManual(title string, lines []string, disks []Disk, roles []Role, mirror bool, row int, prev []string) {
	g.begin()
	y := g.body(lines, g.title(title, ToneNormal)) + 4
	x, w := g.contentX(), g.contentW()
	rh := 38.0
	fl := face(false, g.L(13.5))
	fr := face(true, g.L(13))
	pw := 150.0
	for i := 0; i <= len(disks); i++ {
		hov := g.hover == hitRow+i || g.hover == hitRole+i || (i == len(disks) && g.hover == hitMirror)
		bg := cWindow
		switch {
		case i == row:
			bg = hex(0xF2F7FF)
		case hov:
			bg = hex(0xF8F8FA)
		}
		fillRRect(g.img, g.X(x), g.Y(y), g.L(w), g.L(rh-4), g.L(8), bg)
		if i < len(disks) {
			d := disks[i]
			g.addHit(x, y, w-pw-8, rh-4, hitRow+i)
			label := d.Label() + d.EraseNote()
			col := cText
			if d.Health == HealthBad {
				label = T("FALHANDO · ") + label
				col = cDangerTxt
			}
			text(g.img, fl, g.X(x+12), g.Y(y+22), ellipsize(fl, label, g.L(w-pw-28)), col)
			// seletor de papel (clique troca)
			g.addHit(x+w-pw-4, y+4, pw, rh-12, hitRole+i)
			rbg, rfg := hex(0xE9E9EE), cText
			switch roles[i] {
			case RoleSystem, RoleBoth:
				rbg, rfg = hex(0xE5F0FF), cAccent
			case RoleBackup:
				rbg, rfg = hex(0xE3F6E8), cOKText
			}
			fillRRect(g.img, g.X(x+w-pw-4), g.Y(y+4), g.L(pw), g.L(rh-12), g.L(7), rbg)
			textCentered(g.img, fr, g.X(x+w-pw/2-4), g.Y(y+22), "‹  "+roles[i].String()+"  ›", rfg)
		} else {
			g.addHit(x, y, w, rh-4, hitMirror)
			text(g.img, face(true, g.L(13.5)), g.X(x+12), g.Y(y+22), T("Espelhar os backups (cada arquivo em 2 discos)"), cText)
			// chave liga/desliga
			sw, sh := 40.0, 22.0
			sx, sy := x+w-sw-12, y+6
			on := cOK
			if !mirror {
				on = hex(0xD1D1D6)
			}
			fillRRect(g.img, g.X(sx), g.Y(sy), g.L(sw), g.L(sh), g.L(sh/2), on)
			kx := sx + 11
			if mirror {
				kx = sx + sw - 11
			}
			fillCircle(g.img, g.X(kx), g.Y(sy+sh/2), g.L(9), cWindow)
		}
		y += rh
	}
	g.body(prev, y+6)
	g.buttons([]string{T("Continuar"), T("Voltar")}, 0)
	g.hint(T("↑↓ disco · ← → muda · Enter · Esc volta"))
}

func (g *GUI) Manual(title string, lines []string, disks []Disk, roles []Role, mirror bool,
	preview func([]Role, bool) ([]string, bool)) ([]Role, bool, bool) {
	g.stopAnim()
	roles = append([]Role{}, roles...)
	row := 0
	for {
		prev, valid := preview(roles, mirror)
		g.mu.Lock()
		g.drawManual(title, lines, disks, roles, mirror, row, prev)
		g.present()
		g.mu.Unlock()
		ev := g.wait(nil)
		switch ev.kind {
		case evAbort:
			return roles, mirror, false
		case evClick:
			switch {
			case ev.id == hitBtn:
				if valid {
					return roles, mirror, true
				}
			case ev.id == hitBtn+1:
				return roles, mirror, false
			case ev.id >= hitRole && ev.id < hitRole+len(disks):
				row = ev.id - hitRole
				roles[row] = nextRole(disks[row], roles[row], 1)
			case ev.id >= hitRow && ev.id <= hitRow+len(disks):
				row = ev.id - hitRow
			case ev.id == hitMirror:
				row, mirror = len(disks), !mirror
			}
		case evKey:
			switch ev.key.k {
			case kUp:
				row = (row - 1 + len(disks) + 1) % (len(disks) + 1)
			case kDown, kTab:
				row = (row + 1) % (len(disks) + 1)
			case kLeft, kRight:
				dir := 1
				if ev.key.k == kLeft {
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
}
