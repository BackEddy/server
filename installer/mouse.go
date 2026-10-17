package main

// Mouse direto do Linux (evdev, /dev/input/event*): mouse USB/PS2 (movimento relativo) e
// telas/mesas e o mouse do Hyper-V e outras VMs (posição absoluta). Mouses ligados depois
// também são encontrados (procura de novo a cada 2 segundos).

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	linSyn    = 0
	linKey    = 1
	linRel    = 2
	linAbs    = 3
	relX      = 0
	relY      = 1
	absX      = 0
	absY      = 1
	btnLeft   = 0x110
	btnTouch  = 0x14a
	btnFinger = 0x145 // BTN_TOOL_FINGER: touchpad de notebook
	eventSize = 24    // struct input_event em 64 bits
)

func eviocgbit(ev, length uintptr) uintptr {
	return 2<<30 | length<<16 | 'E'<<8 | (0x20 + ev)
}

func eviocgabs(axis uintptr) uintptr {
	return 2<<30 | 24<<16 | 'E'<<8 | (0x40 + axis)
}

type absInfo struct{ Value, Min, Max, Fuzz, Flat, Res int32 }

type mouseState struct {
	mu      sync.Mutex
	x, y    float64
	w, h    float64
	moved   chan struct{} // sinal de movimento (capacidade 1)
	clicks  chan [2]int   // posição de cada clique
	opened  map[string]bool
	present bool
}

var mouse *mouseState

func hasBit(bits []byte, n int) bool { return n/8 < len(bits) && bits[n/8]&(1<<(uint(n)%8)) != 0 }

// startMouse começa a ler os mouses; w,h = tamanho da tela.
func startMouse(w, h int) *mouseState {
	m := &mouseState{x: float64(w) / 2, y: float64(h) / 2, w: float64(w), h: float64(h),
		moved: make(chan struct{}, 1), clicks: make(chan [2]int, 8), opened: map[string]bool{}}
	mouse = m
	go func() {
		for {
			m.scan()
			time.Sleep(2 * time.Second)
		}
	}()
	return m
}

func (m *mouseState) scan() {
	paths, _ := filepath.Glob("/dev/input/event*")
	for _, p := range paths {
		m.mu.Lock()
		done := m.opened[p]
		m.mu.Unlock()
		if done {
			continue
		}
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		var types [4]byte
		var keys [96]byte
		_ = ioctl(f.Fd(), eviocgbit(0, 4), unsafe.Pointer(&types[0]))
		_ = ioctl(f.Fd(), eviocgbit(linKey, 96), unsafe.Pointer(&keys[0]))
		rel := hasBit(types[:], linRel)
		abs := hasBit(types[:], linAbs)
		button := hasBit(keys[:], btnLeft) || hasBit(keys[:], btnTouch)
		if !button || (!rel && !abs) {
			f.Close()
			continue
		}
		var ax, ay absInfo
		if abs {
			_ = ioctl(f.Fd(), eviocgabs(absX), unsafe.Pointer(&ax))
			_ = ioctl(f.Fd(), eviocgabs(absY), unsafe.Pointer(&ay))
		}
		pad := abs && hasBit(keys[:], btnFinger) // touchpad: posição absoluta vira movimento
		m.mu.Lock()
		m.opened[p] = true
		m.mu.Unlock()
		go m.read(f, p, ax, ay, pad)
	}
}

func (m *mouseState) read(f *os.File, path string, ax, ay absInfo, pad bool) {
	defer func() {
		f.Close()
		m.mu.Lock()
		delete(m.opened, path) // desconectado: pode voltar depois
		m.mu.Unlock()
	}()
	buf := make([]byte, eventSize*16)
	var dx, dy float64
	var nx, ny = -1.0, -1.0
	var lastX, lastY = -1.0, -1.0 // touchpad: última posição do dedo
	for {
		n, err := f.Read(buf)
		if err != nil {
			if err == syscall.EINTR {
				continue
			}
			return
		}
		for o := 0; o+eventSize <= n; o += eventSize {
			typ := binary.LittleEndian.Uint16(buf[o+16:])
			code := binary.LittleEndian.Uint16(buf[o+18:])
			val := int32(binary.LittleEndian.Uint32(buf[o+20:]))
			switch typ {
			case linRel:
				switch code {
				case relX:
					dx += float64(val)
				case relY:
					dy += float64(val)
				}
			case linAbs:
				switch code {
				case absX:
					if ax.Max > ax.Min {
						v := float64(val-ax.Min) / float64(ax.Max-ax.Min)
						if pad {
							if lastX >= 0 {
								dx += (v - lastX) * m.w * 1.2
							}
							lastX = v
						} else {
							nx = v
						}
					}
				case absY:
					if ay.Max > ay.Min {
						v := float64(val-ay.Min) / float64(ay.Max-ay.Min)
						if pad {
							if lastY >= 0 {
								dy += (v - lastY) * m.h * 1.2
							}
							lastY = v
						} else {
							ny = v
						}
					}
				}
			case linKey:
				if pad && code == btnTouch { // dedo encostou ou saiu do touchpad: recomeça o movimento
					lastX, lastY = -1, -1
					continue
				}
				if (code == btnLeft || code == btnTouch) && val == 1 {
					m.mu.Lock()
					m.present = true
					p := [2]int{int(m.x), int(m.y)}
					m.mu.Unlock()
					select {
					case m.clicks <- p:
					default:
					}
				}
			case linSyn:
				if dx != 0 || dy != 0 || nx >= 0 || ny >= 0 {
					m.mu.Lock()
					k := 1.4
					if pad {
						k = 1
					}
					m.x += dx * k
					m.y += dy * k
					if nx >= 0 {
						m.x = nx * m.w
					}
					if ny >= 0 {
						m.y = ny * m.h
					}
					m.x = clampF(m.x, 0, m.w-1)
					m.y = clampF(m.y, 0, m.h-1)
					m.present = true
					m.mu.Unlock()
					dx, dy, nx, ny = 0, 0, -1, -1
					select {
					case m.moved <- struct{}{}:
					default:
					}
				}
			}
		}
	}
}

func (m *mouseState) pos() (int, int, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return int(m.x), int(m.y), m.present
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
