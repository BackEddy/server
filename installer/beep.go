package main

// Bipe pelo alto-falante interno do PC (o mesmo que o servidor usa nos alertas).

import (
	"os"
	"os/exec"
	"syscall"
	"time"
)

func tone(freq int, d time.Duration) {
	f, err := os.OpenFile("/dev/console", os.O_WRONLY, 0)
	if err != nil {
		time.Sleep(d)
		return
	}
	defer f.Close()
	const kiocsound = 0x4B2F
	_, _, _ = syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), kiocsound, uintptr(1193180/freq))
	time.Sleep(d)
	_, _, _ = syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), kiocsound, 0)
}

func beepPattern() {
	_ = exec.Command("modprobe", "pcspkr").Run()
	for i := 0; i < 3; i++ {
		tone(1760, 180*time.Millisecond)
		time.Sleep(120 * time.Millisecond)
	}
}
