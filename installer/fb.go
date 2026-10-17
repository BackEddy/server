package main

// Framebuffer do Linux (/dev/fb0): a tela que o firmware UEFI (GOP) ou o driver de vídeo entrega
// pronta. Sem X, sem Wayland: o instalador desenha numa imagem na memória e copia para cá.

import (
	"errors"
	"image"
	"os"
	"syscall"
	"unsafe"
)

const (
	fbioGetVScreenInfo = 0x4600
	fbioGetFScreenInfo = 0x4602
	kdSetMode          = 0x4B3A
	kdText             = 0
	kdGraphics         = 1
)

type fbBitfield struct{ Offset, Length, MsbRight uint32 }

type fbVarScreenInfo struct {
	Xres, Yres, XresVirtual, YresVirtual, Xoffset, Yoffset, BitsPerPixel, Grayscale uint32
	Red, Green, Blue, Transp                                                        fbBitfield
	Nonstd, Activate, Height, Width, AccelFlags                                     uint32
	Pixclock, LeftMargin, RightMargin, UpperMargin, LowerMargin, HsyncLen, VsyncLen uint32
	Sync, Vmode, Rotate, Colorspace                                                 uint32
	Reserved                                                                        [4]uint32
}

type fbFixScreenInfo struct {
	ID                             [16]byte
	SmemStart                      uint64
	SmemLen, Type, TypeAux, Visual uint32
	Xpanstep, Ypanstep, Ywrapstep  uint16
	LineLength                     uint32
	MmioStart                      uint64
	MmioLen, Accel                 uint32
	Capabilities                   uint16
	Reserved                       [2]uint16
}

type Framebuffer struct {
	f           *os.File
	mem         []byte
	W, H        int
	stride, bpp int
	base        int
	r, g, b     fbBitfield
	tty         *os.File
}

func ioctl(fd uintptr, req uintptr, arg unsafe.Pointer) error {
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(arg)); e != 0 {
		return e
	}
	return nil
}

func OpenFramebuffer(path string) (*Framebuffer, error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	var v fbVarScreenInfo
	var x fbFixScreenInfo
	if err := ioctl(f.Fd(), fbioGetVScreenInfo, unsafe.Pointer(&v)); err != nil {
		f.Close()
		return nil, err
	}
	if err := ioctl(f.Fd(), fbioGetFScreenInfo, unsafe.Pointer(&x)); err != nil {
		f.Close()
		return nil, err
	}
	bpp := int(v.BitsPerPixel)
	if (bpp != 16 && bpp != 24 && bpp != 32) || v.Xres < 640 || v.Yres < 480 {
		f.Close()
		return nil, errors.New("framebuffer sem formato compatível")
	}
	size := int(x.LineLength) * int(v.YresVirtual)
	if x.SmemLen > 0 && int(x.SmemLen) < size {
		size = int(x.SmemLen)
	}
	mem, err := syscall.Mmap(int(f.Fd()), 0, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		f.Close()
		return nil, err
	}
	fb := &Framebuffer{f: f, mem: mem, W: int(v.Xres), H: int(v.Yres), stride: int(x.LineLength),
		bpp: bpp, r: v.Red, g: v.Green, b: v.Blue,
		base: int(v.Yoffset)*int(x.LineLength) + int(v.Xoffset)*bpp/8}
	if fb.base+fb.stride*fb.H > len(mem) {
		fb.Close()
		return nil, errors.New("framebuffer menor que a tela")
	}
	// console em modo gráfico: o texto (cursor, mensagens do sistema) para de desenhar por cima
	if tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		if ioctlInt(tty.Fd(), kdSetMode, kdGraphics) == nil {
			fb.tty = tty
		} else {
			tty.Close()
		}
	}
	return fb, nil
}

func ioctlInt(fd uintptr, req uintptr, val uintptr) error {
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, req, val); e != 0 {
		return e
	}
	return nil
}

func (fb *Framebuffer) Close() {
	if fb.tty != nil {
		_ = ioctlInt(fb.tty.Fd(), kdSetMode, kdText)
		fb.tty.Close()
		fb.tty = nil
	}
	if fb.mem != nil {
		_ = syscall.Munmap(fb.mem)
		fb.mem = nil
	}
	fb.f.Close()
}

func chan8(v uint8, bf fbBitfield) uint32 {
	return uint32(v) >> (8 - bf.Length) << bf.Offset
}

// Show copia a imagem (mesmo tamanho da tela) para o framebuffer.
func (fb *Framebuffer) Show(img *image.RGBA) {
	fast := fb.bpp == 32 && fb.r.Offset == 16 && fb.g.Offset == 8 && fb.b.Offset == 0 &&
		fb.r.Length == 8 && fb.g.Length == 8 && fb.b.Length == 8
	for y := 0; y < fb.H; y++ {
		src := img.Pix[y*img.Stride : y*img.Stride+fb.W*4]
		row := fb.mem[fb.base+y*fb.stride:]
		switch {
		case fast: // BGRX, o caso de quase todo PC
			for x := 0; x < fb.W; x++ {
				row[x*4], row[x*4+1], row[x*4+2], row[x*4+3] = src[x*4+2], src[x*4+1], src[x*4], 0xff
			}
		default:
			bytes := fb.bpp / 8
			for x := 0; x < fb.W; x++ {
				p := chan8(src[x*4], fb.r) | chan8(src[x*4+1], fb.g) | chan8(src[x*4+2], fb.b)
				for i := 0; i < bytes; i++ {
					row[x*bytes+i] = byte(p >> (8 * i))
				}
			}
		}
	}
}
