package main

import (
	"image"
	"image/color"
	"testing"
	"unsafe"
)

// As estruturas precisam ter exatamente o tamanho das do kernel (linux/fb.h, amd64).
func TestFramebufferStructSizes(t *testing.T) {
	if s := unsafe.Sizeof(fbVarScreenInfo{}); s != 160 {
		t.Fatalf("fb_var_screeninfo: %d bytes, esperado 160", s)
	}
	if s := unsafe.Sizeof(fbFixScreenInfo{}); s != 80 {
		t.Fatalf("fb_fix_screeninfo: %d bytes, esperado 80", s)
	}
}

func TestFramebufferShow(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	img.Set(0, 0, color.RGBA{0x11, 0x22, 0x33, 0xff})
	img.Set(1, 0, color.RGBA{0xff, 0x80, 0x00, 0xff})
	// BGRX 32 bits
	fb := &Framebuffer{mem: make([]byte, 8), W: 2, H: 1, stride: 8, bpp: 32,
		r: fbBitfield{Offset: 16, Length: 8}, g: fbBitfield{Offset: 8, Length: 8}, b: fbBitfield{Offset: 0, Length: 8}}
	fb.Show(img)
	if got := fb.mem[:4]; got[0] != 0x33 || got[1] != 0x22 || got[2] != 0x11 {
		t.Fatalf("BGRX: % x", got)
	}
	// RGB565 16 bits
	fb = &Framebuffer{mem: make([]byte, 4), W: 2, H: 1, stride: 4, bpp: 16,
		r: fbBitfield{Offset: 11, Length: 5}, g: fbBitfield{Offset: 5, Length: 6}, b: fbBitfield{Offset: 0, Length: 5}}
	fb.Show(img)
	if v := uint16(fb.mem[2]) | uint16(fb.mem[3])<<8; v != 0xfc00 {
		t.Fatalf("RGB565: %04x", v)
	}
}

// Todas as telas desenham sem erro em resoluções comuns (inclusive pequenas).
func TestGUIScreens(t *testing.T) {
	for _, sz := range [][2]int{{640, 480}, {800, 600}, {1366, 768}, {1920, 1080}} {
		g := newGUI(sz[0], sz[1])
		g.drawChoose("Encontrei estes discos", ToneWarn, []string{"!aviso", "+ok", "*destaque", "• item"}, []string{"Continuar", "Desligar"}, 1)
		g.drawInput("Senha", nil, "Senha:", "123456", true, "erro")
		g.drawProgress("Instalando", Steps, 3, "nota", 5)
		g.drawProgress("Instalando", Steps, len(Steps), "", 0)
	}
}
