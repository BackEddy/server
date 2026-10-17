package main

// Os passos da instalação: particionar, formatar, copiar o sistema, configurar e instalar o boot.
// A configuração dos discos de backup (btrfs, espelho, Samba, Kopia) fica para o primeiro boot,
// que roda o mesmo install.sh já testado.

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"
)

const (
	target    = "/mnt/target"
	rootfsTar = "/opt/backeddy/rootfs.tar.zst"
	logPath   = "/tmp/backeddy-install.log"
)

type Installer struct {
	Layout               *Layout
	Name                 string
	Password             string
	Plan                 *Plan
	DryRun               bool
	TMPercent            int  // % do espaço para o Time Machine
	Recover              bool // reinstala só o sistema e mantém o volume de backups
	BeepOK               bool // o PC tem alto-falante (teste do alarme)
	QuietStart, QuietEnd int  // silêncio à noite (-1 = bipar a qualquer hora)

	log       *os.File
	espPart   string
	rootPart  string
	dataUUIDs []string
}

var Steps = []string{
	"Preparando os discos",
	"Criando o sistema de arquivos",
	"Copiando o sistema (alguns minutos)",
	"Configurando o servidor",
	"Instalando a inicialização",
	"Finalizando",
}

func (in *Installer) logf(format string, a ...any) {
	if in.log != nil {
		fmt.Fprintf(in.log, format+"\n", a...)
	}
}

// run executa um comando registrando tudo no log; em dry-run só registra.
func (in *Installer) run(name string, args ...string) error {
	return in.runIn("", name, args...)
}

func (in *Installer) runIn(stdin string, name string, args ...string) error {
	in.logf("$ %s %s", name, strings.Join(args, " "))
	if in.DryRun {
		return nil
	}
	cmd := exec.Command(name, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	out, err := cmd.CombinedOutput()
	if len(out) > 0 {
		in.logf("%s", strings.TrimRight(string(out), "\n"))
	}
	if err != nil {
		return fmt.Errorf("%s falhou: %v (veja %s)", name, err, logPath)
	}
	return nil
}

func (in *Installer) chroot(args ...string) error {
	return in.run("chroot", append([]string{target}, args...)...)
}

func (in *Installer) output(name string, args ...string) string {
	if in.DryRun {
		return "DRYRUN-" + strings.ToUpper(filepath.Base(args[len(args)-1]))
	}
	out, _ := exec.Command(name, args...).Output()
	return strings.TrimSpace(string(out))
}

func partPath(disk string, n int) string {
	if r := rune(disk[len(disk)-1]); unicode.IsDigit(r) {
		return fmt.Sprintf("%sp%d", disk, n) // nvme0n1p1, mmcblk0p1
	}
	return fmt.Sprintf("%s%d", disk, n) // sda1
}

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Run executa todos os passos, chamando progress(i) no início de cada um.
func (in *Installer) Run(progress func(step int)) error {
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err == nil {
		in.log = f
		defer f.Close()
	}
	steps := []func() error{in.partition, in.format, in.copySystem, in.configure, in.bootloader, in.finish}
	for i, s := range steps {
		progress(i)
		in.logf("== %s", Steps[i])
		if err := s(); err != nil {
			in.cleanup()
			return err
		}
	}
	return nil
}

func (in *Installer) partition() error {
	sys := in.Layout.System
	if in.Layout.ReuseSystem { // recuperação no mesmo disco: só refaz as partições do sistema
		in.espPart, in.rootPart = sys.EFIPart, sys.RootPart
		return nil
	}
	shared := false
	for _, t := range in.Layout.Data {
		if !t.Whole {
			shared = true
		}
	}
	for _, d := range in.Layout.AllDisks() {
		_ = in.run("wipefs", "-a", d.Path)
		if err := in.run("sgdisk", "--zap-all", d.Path); err != nil {
			return err
		}
	}
	// disco só do sistema: a partição do sistema ocupa o disco inteiro; dividido: 20 GB
	rootEnd := "0"
	if shared {
		rootEnd = "+20G"
	}
	args := []string{
		"-n1:0:+512M", "-t1:ef00", "-c1:EFI",
		"-n2:0:" + rootEnd, "-t2:8304", "-c2:backeddy-root",
	}
	if shared {
		u := newUUID()
		in.dataUUIDs = append(in.dataUUIDs, u)
		args = append(args, "-n3:0:0", "-t3:8300", "-c3:backeddy-data", "-u3:"+u)
	}
	if err := in.run("sgdisk", append(args, sys.Path)...); err != nil {
		return err
	}
	for _, t := range in.Layout.Data {
		if !t.Whole {
			continue
		}
		u := newUUID()
		in.dataUUIDs = append(in.dataUUIDs, u)
		if err := in.run("sgdisk", "-n1:0:0", "-t1:8300", "-c1:backeddy-data", "-u1:"+u, t.Disk.Path); err != nil {
			return err
		}
	}
	for _, d := range in.Layout.AllDisks() {
		_ = in.run("partprobe", d.Path)
	}
	_ = in.run("udevadm", "settle")
	in.espPart = partPath(sys.Path, 1)
	in.rootPart = partPath(sys.Path, 2)
	return nil
}

func (in *Installer) format() error {
	if err := in.run("mkfs.vfat", "-F", "32", "-n", "EFI", in.espPart); err != nil {
		return err
	}
	return in.run("mkfs.ext4", "-q", "-F", "-L", "backeddy-root", in.rootPart)
}

func (in *Installer) copySystem() error {
	if err := in.run("mkdir", "-p", target); err != nil {
		return err
	}
	if err := in.run("mount", in.rootPart, target); err != nil {
		return err
	}
	if err := in.run("mkdir", "-p", target+"/boot/efi"); err != nil {
		return err
	}
	if err := in.run("mount", in.espPart, target+"/boot/efi"); err != nil {
		return err
	}
	return in.run("tar", "--zstd", "--xattrs", "--xattrs-include=*", "--numeric-owner",
		"-xpf", rootfsTar, "-C", target)
}

func (in *Installer) writeFile(path, content string, mode os.FileMode) error {
	in.logf("> escreve %s", path)
	if in.DryRun {
		if !strings.Contains(path, "firstboot.env") { // nunca registra a senha
			in.logf("%s", content)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), mode)
}

func (in *Installer) configure() error {
	rootUUID := in.output("blkid", "-s", "UUID", "-o", "value", in.rootPart)
	espUUID := in.output("blkid", "-s", "UUID", "-o", "value", in.espPart)
	fstab := fmt.Sprintf("# Backeddy Server\nUUID=%s / ext4 errors=remount-ro 0 1\nUUID=%s /boot/efi vfat umask=0077 0 1\n", rootUUID, espUUID)
	if err := in.writeFile(target+"/etc/fstab", fstab, 0644); err != nil {
		return err
	}

	var disks []string
	for _, u := range in.dataUUIDs {
		disks = append(disks, "/dev/disk/by-partuuid/"+u)
	}
	tm := in.TMPercent
	if tm <= 0 {
		tm = in.Plan.TMPercent()
	}
	env := fmt.Sprintf("# Lido pelo primeiro boot e apagado em seguida\nNAME_B64=%s\nPASSWORD_B64=%s\nTM_PERCENT=%d\nDISKS=\"%s\"\n",
		base64.StdEncoding.EncodeToString([]byte(in.Name)),
		base64.StdEncoding.EncodeToString([]byte(in.Password)),
		tm, strings.Join(disks, " "))
	if in.Recover {
		env += "RECOVER=1\n"
	}
	if in.QuietStart >= 0 && in.QuietEnd >= 0 && (in.QuietStart != 0 || in.QuietEnd != 0) {
		env += fmt.Sprintf("QUIET_START=%d\nQUIET_END=%d\n", in.QuietStart, in.QuietEnd)
	} else if in.QuietStart < 0 {
		env += "QUIET_START=\nQUIET_END=\n"
	}
	if in.BeepOK {
		env += "BEEP_OK=1\n"
	} else {
		env += "BEEP_OK=0\n"
	}
	if !in.Layout.Mirror && len(in.Layout.Data) >= 2 {
		env += "NO_MIRROR=1\n"
	}
	if err := in.writeFile(target+"/etc/backeddy/firstboot.env", env, 0600); err != nil {
		return err
	}

	if in.Plan != nil && in.Plan.Timezone != "" && !strings.Contains(in.Plan.Timezone, "..") {
		_ = in.run("ln", "-sf", "/usr/share/zoneinfo/"+in.Plan.Timezone, target+"/etc/localtime")
		_ = in.writeFile(target+"/etc/timezone", in.Plan.Timezone+"\n", 0644)
	}

	for _, m := range [][]string{
		{"--rbind", "/dev", target + "/dev"},
		{"-t", "proc", "proc", target + "/proc"},
		{"--rbind", "/sys", target + "/sys"},
		{"-t", "tmpfs", "tmpfs", target + "/run"},
	} {
		if err := in.run("mount", m...); err != nil {
			return err
		}
	}
	// identidade única desta instalação (a imagem é igual para todo mundo)
	_ = in.run("sh", "-c", "rm -f "+target+"/etc/ssh/ssh_host_* "+target+"/etc/machine-id")
	if err := in.chroot("ssh-keygen", "-A"); err != nil {
		return err
	}
	_ = in.chroot("systemd-machine-id-setup")
	// usuário de administração (mesma senha do servidor)
	_ = in.chroot("useradd", "-m", "-s", "/bin/bash", "-G", "sudo", "admin")
	return in.runIn("admin:"+in.Password+"\n", "chroot", target, "chpasswd")
}

func (in *Installer) bootloader() error {
	// caminho "removível" (EFI/BOOT/BOOTX64.EFI): funciona sem gravar nada na NVRAM do PC
	if err := in.chroot("grub-install", "--target=x86_64-efi", "--efi-directory=/boot/efi",
		"--uefi-secure-boot", "--removable", "--no-nvram"); err != nil {
		return err
	}
	// também registra "debian" na NVRAM quando possível (não é obrigatório)
	_ = in.chroot("grub-install", "--target=x86_64-efi", "--efi-directory=/boot/efi",
		"--bootloader-id=debian", "--uefi-secure-boot")
	return in.chroot("update-grub")
}

func (in *Installer) finish() error {
	in.cleanup()
	return in.run("sync")
}

func (in *Installer) cleanup() {
	_ = in.run("umount", "-R", target)
}
