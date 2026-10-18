package main

// backeddy-agent — vigia do Backeddy Server.
// A cada minuto pergunta ao "backeddy check" se há algo preocupante. Havendo alerta não
// reconhecido (e fora do horário de silêncio), bipa. Qualquer tecla no teclado do próprio
// servidor reconhece os alertas por 1 dia; a interface web (http://nome.local) e os apps,
// pela API (web.go), escolhendo quando lembrar de novo.

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	acksPath  = "/var/lib/backeddy/acks"
	stateDir  = "/run/backeddy"
	confPath  = "/etc/backeddy/alerts.conf"
	kiocsound = 0x4B2F
)

type Alert struct{ ID, Text string }

// backeddy: o comando do servidor (variável para os testes trocarem)
var backeddy = "/usr/local/bin/backeddy"

var mu sync.Mutex
var pending []Alert

func main() {
	test := flag.Bool("test-beep", false, "bipa uma vez e sai (teste do setup)")
	flag.Parse()
	_ = exec.Command("modprobe", "pcspkr").Run()
	if *test {
		beepPattern()
		return
	}
	log.SetFlags(0)
	go watchKeyboards()
	go serveWeb(":80")
	time.Sleep(90 * time.Second) // deixa o boot terminar (discos e serviços subindo)
	for {
		check()
		mu.Lock()
		n := len(pending)
		mu.Unlock()
		if n > 0 && !quietNow(time.Now()) {
			beepPattern()
		}
		time.Sleep(60 * time.Second)
	}
}

func check() {
	out, err := exec.Command(backeddy, "check").Output()
	if err != nil {
		log.Printf("backeddy check: %v", err)
	}
	acks := readAcks()
	now := time.Now().Unix()
	var p []Alert
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		id, txt, ok := strings.Cut(l, "|")
		if !ok || id == "" {
			continue
		}
		if until, ok := acks[id]; ok && until > now {
			continue
		}
		p = append(p, Alert{id, txt})
	}
	mu.Lock()
	pending = p
	mu.Unlock()
	writeState(p)
}

func readAcks() map[string]int64 {
	m := map[string]int64{}
	f, err := os.Open(acksPath)
	if err != nil {
		return m
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		parts := strings.Fields(sc.Text())
		if len(parts) == 2 {
			if v, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
				m[parts[0]] = v
			}
		}
	}
	return m
}

// writeState: alertas pendentes para a tela do servidor e a interface web.
func writeState(p []Alert) {
	_ = os.MkdirAll(stateDir, 0o755)
	var b strings.Builder
	for _, a := range p {
		fmt.Fprintf(&b, "%s|%s\n", a.ID, a.Text)
	}
	tmp := filepath.Join(stateDir, "alerts.tmp")
	if os.WriteFile(tmp, []byte(b.String()), 0o644) == nil {
		_ = os.Rename(tmp, filepath.Join(stateDir, "alerts"))
	}
}

// quietHours: horário de silêncio (padrão 22h–8h; -1 = desligado por "backeddy quiet off").
func quietHours() (int, int) {
	start, end := 22, 8
	if data, err := os.ReadFile(confPath); err == nil {
		start, end = -1, -1
		for _, l := range strings.Split(string(data), "\n") {
			k, v, _ := strings.Cut(strings.TrimSpace(l), "=")
			n, err := strconv.Atoi(strings.Trim(v, `"`))
			if err != nil {
				continue
			}
			switch k {
			case "QUIET_START":
				start = n
			case "QUIET_END":
				end = n
			}
		}
	}
	return start, end
}

func quietNow(t time.Time) bool {
	s, e := quietHours()
	return inQuiet(t.Hour(), s, e)
}

func inQuiet(h, start, end int) bool {
	if start < 0 || end < 0 || start == end {
		return false
	}
	if start < end {
		return h >= start && h < end
	}
	return h >= start || h < end // atravessa a meia-noite
}

// ---------- bipe (alto-falante interno do PC) ----------

func tone(freq int, d time.Duration) {
	f, err := os.OpenFile("/dev/console", os.O_WRONLY, 0)
	if err != nil {
		time.Sleep(d)
		return
	}
	defer f.Close()
	_, _, _ = syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), kiocsound, uintptr(1193180/freq))
	time.Sleep(d)
	_, _, _ = syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), kiocsound, 0)
}

func beepPattern() {
	for i := 0; i < 3; i++ {
		tone(1760, 180*time.Millisecond)
		time.Sleep(120 * time.Millisecond)
	}
}

func okBeep() {
	tone(880, 90*time.Millisecond)
	tone(1320, 140*time.Millisecond)
}

// ---------- teclado: qualquer tecla reconhece ----------

func watchKeyboards() {
	opened := map[string]bool{}
	var omu sync.Mutex
	for {
		paths, _ := filepath.Glob("/dev/input/event*")
		for _, p := range paths {
			omu.Lock()
			done := opened[p]
			omu.Unlock()
			if done || !isKeyboard(p) {
				continue
			}
			f, err := os.Open(p)
			if err != nil {
				continue
			}
			omu.Lock()
			opened[p] = true
			omu.Unlock()
			go func(p string, f *os.File) {
				defer func() { f.Close(); omu.Lock(); delete(opened, p); omu.Unlock() }()
				buf := make([]byte, 24*16)
				for {
					n, err := f.Read(buf)
					if err != nil {
						return
					}
					for o := 0; o+24 <= n; o += 24 {
						typ := uint16(buf[o+16]) | uint16(buf[o+17])<<8
						val := int32(uint32(buf[o+20]) | uint32(buf[o+21])<<8 | uint32(buf[o+22])<<16 | uint32(buf[o+23])<<24)
						if typ == 1 && val == 1 { // tecla apertada
							keyAck()
						}
					}
				}
			}(p, f)
		}
		time.Sleep(3 * time.Second)
	}
}

func isKeyboard(p string) bool {
	f, err := os.Open(p)
	if err != nil {
		return false
	}
	defer f.Close()
	var keys [96]byte
	req := uintptr(2<<30 | 96<<16 | 'E'<<8 | (0x20 + 1)) // EVIOCGBIT(EV_KEY)
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), req, uintptr(unsafe.Pointer(&keys[0]))); e != 0 {
		return false
	}
	has := func(k int) bool { return keys[k/8]&(1<<(uint(k)%8)) != 0 }
	return has(30) && has(28) // KEY_A e KEY_ENTER: é teclado (mouse não tem)
}

var lastAck time.Time

func keyAck() {
	mu.Lock()
	n := len(pending)
	recent := time.Since(lastAck) < 5*time.Second
	if n > 0 && !recent {
		lastAck = time.Now()
	}
	mu.Unlock()
	if n == 0 || recent {
		return
	}
	if err := exec.Command(backeddy, "ack", "1").Run(); err != nil {
		log.Printf("ack: %v", err)
		return
	}
	log.Printf("alertas reconhecidos pelo teclado do servidor")
	okBeep()
	check()
}
