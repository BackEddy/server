package main

// Desenho 2D simples com suavização (anti-aliasing): retângulos arredondados, círculos,
// polígonos, sombras, degradês e texto com a fonte Inter (embutida no programa).

import (
	_ "embed"
	"image"
	"image/color"
	"image/draw"
	"math"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

//go:embed fonts/Inter-Regular.ttf
var interRegular []byte

//go:embed fonts/Inter-SemiBold.ttf
var interSemiBold []byte

type pt struct{ X, Y float64 }

func hex(h uint32) color.NRGBA {
	return color.NRGBA{uint8(h >> 16), uint8(h >> 8), uint8(h), 0xff}
}

func alpha(c color.NRGBA, a float64) color.NRGBA {
	c.A = uint8(math.Round(float64(c.A) * a))
	return c
}

// fillPolys preenche um ou mais polígonos (regra "nonzero").
func fillPolys(dst *image.RGBA, polys [][]pt, c color.Color) {
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, p := range polys {
		for _, q := range p {
			minX, minY = math.Min(minX, q.X), math.Min(minY, q.Y)
			maxX, maxY = math.Max(maxX, q.X), math.Max(maxY, q.Y)
		}
	}
	r := image.Rect(int(math.Floor(minX)), int(math.Floor(minY)), int(math.Ceil(maxX))+1, int(math.Ceil(maxY))+1).
		Intersect(dst.Bounds())
	if r.Empty() {
		return
	}
	z := vector.NewRasterizer(r.Dx(), r.Dy())
	z.DrawOp = draw.Over
	ox, oy := float64(r.Min.X), float64(r.Min.Y)
	for _, p := range polys {
		if len(p) < 3 {
			continue
		}
		z.MoveTo(float32(p[0].X-ox), float32(p[0].Y-oy))
		for _, q := range p[1:] {
			z.LineTo(float32(q.X-ox), float32(q.Y-oy))
		}
		z.ClosePath()
	}
	z.Draw(dst, r, image.NewUniform(c), image.Point{})
}

func rrectPoly(x, y, w, h, r float64) []pt {
	r = math.Min(r, math.Min(w, h)/2)
	var p []pt
	corner := func(cx, cy, a0 float64) {
		const n = 10
		for i := 0; i <= n; i++ {
			a := a0 + float64(i)*math.Pi/2/n
			p = append(p, pt{cx + r*math.Cos(a), cy + r*math.Sin(a)})
		}
	}
	corner(x+w-r, y+r, -math.Pi/2)
	corner(x+w-r, y+h-r, 0)
	corner(x+r, y+h-r, math.Pi/2)
	corner(x+r, y+r, math.Pi)
	return p
}

func fillRRect(dst *image.RGBA, x, y, w, h, r float64, c color.Color) {
	fillPolys(dst, [][]pt{rrectPoly(x, y, w, h, r)}, c)
}

func circlePoly(cx, cy, r float64) []pt {
	n := int(math.Max(24, r*1.5))
	p := make([]pt, n)
	for i := range p {
		a := 2 * math.Pi * float64(i) / float64(n)
		p[i] = pt{cx + r*math.Cos(a), cy + r*math.Sin(a)}
	}
	return p
}

func fillCircle(dst *image.RGBA, cx, cy, r float64, c color.Color) {
	fillPolys(dst, [][]pt{circlePoly(cx, cy, r)}, c)
}

// line desenha um traço grosso com pontas arredondadas.
func line(dst *image.RGBA, a, b pt, w float64, c color.Color) {
	dx, dy := b.X-a.X, b.Y-a.Y
	l := math.Hypot(dx, dy)
	if l == 0 {
		return
	}
	nx, ny := -dy/l*w/2, dx/l*w/2
	fillPolys(dst, [][]pt{
		{{a.X + nx, a.Y + ny}, {b.X + nx, b.Y + ny}, {b.X - nx, b.Y - ny}, {a.X - nx, a.Y - ny}},
		circlePoly(a.X, a.Y, w/2), circlePoly(b.X, b.Y, w/2),
	}, c)
}

// arc é um anel parcial (ângulos em radianos, sentido horário na tela).
func arc(dst *image.RGBA, cx, cy, r, w, a0, a1 float64, c color.Color) {
	const n = 64
	var outer, inner []pt
	for i := 0; i <= n; i++ {
		a := a0 + (a1-a0)*float64(i)/n
		outer = append(outer, pt{cx + (r+w/2)*math.Cos(a), cy + (r+w/2)*math.Sin(a)})
		inner = append(inner, pt{cx + (r-w/2)*math.Cos(a), cy + (r-w/2)*math.Sin(a)})
	}
	for i := len(inner) - 1; i >= 0; i-- {
		outer = append(outer, inner[i])
	}
	fillPolys(dst, [][]pt{outer}, c)
}

// shadow desenha uma sombra suave embaixo de um retângulo arredondado.
func shadow(dst *image.RGBA, x, y, w, h, r, spread, dy, strength float64) {
	steps := int(math.Max(6, spread))
	for i := steps; i >= 1; i-- {
		e := spread * float64(i) / float64(steps)
		a := strength / float64(steps) * 1.6
		fillRRect(dst, x-e, y-e+dy, w+2*e, h+2*e, r+e, color.NRGBA{0, 0, 0, uint8(math.Min(255, a*255))})
	}
}

// gradient pinta um degradê diagonal entre quatro cantos.
func gradient(dst *image.RGBA, tl, tr, bl, br color.NRGBA) {
	b := dst.Bounds()
	w, h := float64(b.Dx()), float64(b.Dy())
	mix := func(a, b uint8, t float64) uint8 { return uint8(float64(a) + (float64(b)-float64(a))*t) }
	for y := 0; y < b.Dy(); y++ {
		ty := float64(y) / h
		for x := 0; x < b.Dx(); x++ {
			tx := float64(x) / w
			i := y*dst.Stride + x*4
			for k, f := range []func(color.NRGBA) uint8{
				func(c color.NRGBA) uint8 { return c.R }, func(c color.NRGBA) uint8 { return c.G }, func(c color.NRGBA) uint8 { return c.B },
			} {
				top := mix(f(tl), f(tr), tx)
				bot := mix(f(bl), f(br), tx)
				dst.Pix[i+k] = mix(top, bot, ty)
			}
			dst.Pix[i+3] = 0xff
		}
	}
}

// ---------- texto ----------

type faceKey struct {
	bold bool
	px   int
}

var (
	fontsOnce sync.Once
	fReg      *opentype.Font
	fBold     *opentype.Font
	facesMu   sync.Mutex
	faces     = map[faceKey]font.Face{}
)

func face(bold bool, px float64) font.Face {
	fontsOnce.Do(func() {
		var err error
		if fReg, err = opentype.Parse(interRegular); err != nil {
			panic(err)
		}
		if fBold, err = opentype.Parse(interSemiBold); err != nil {
			panic(err)
		}
	})
	k := faceKey{bold, int(math.Round(px))}
	facesMu.Lock()
	defer facesMu.Unlock()
	if f, ok := faces[k]; ok {
		return f
	}
	src := fReg
	if bold {
		src = fBold
	}
	f, err := opentype.NewFace(src, &opentype.FaceOptions{Size: float64(k.px), DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		panic(err)
	}
	faces[k] = f
	return f
}

func textWidth(f font.Face, s string) float64 {
	return float64(font.MeasureString(f, s)) / 64
}

// text escreve s com a linha de base em y.
func text(dst *image.RGBA, f font.Face, x, y float64, s string, c color.Color) {
	d := font.Drawer{Dst: dst, Src: image.NewUniform(c), Face: f,
		Dot: fixed.Point26_6{X: fixed.Int26_6(x * 64), Y: fixed.Int26_6(y * 64)}}
	d.DrawString(s)
}

func textCentered(dst *image.RGBA, f font.Face, cx, y float64, s string, c color.Color) {
	text(dst, f, cx-textWidth(f, s)/2, y, s, c)
}

// wrapPx quebra o texto para caber em maxW pixels.
func wrapPx(f font.Face, s string, maxW float64) []string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	cur := ""
	for _, w := range words {
		try := w
		if cur != "" {
			try = cur + " " + w
		}
		if cur != "" && textWidth(f, try) > maxW {
			lines = append(lines, cur)
			cur = w
		} else {
			cur = try
		}
	}
	return append(lines, cur)
}
