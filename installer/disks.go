package main

// Detecção de discos e modos de uso: o instalador recomenda um arranjo (já marcado) e oferece
// os outros com o espaço e o risco de cada um. Liberdade total, com os riscos avisados.

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

const (
	GB          = int64(1) << 30
	minDisk     = 8 * GB  // discos menores são ignorados (pendrives pequenos, cartões)
	rootSize    = 20 * GB // partição do sistema
	minShared   = 40 * GB // disco que divide sistema + backups
	minSystemSD = 16 * GB // disco dedicado ao sistema
	minMirrorPt = 40 * GB // sobra mínima do disco do sistema para entrar num espelho
	sharedOver  = rootSize + GB
)

// Health: situação do disco segundo o SMART.
type Health int

const (
	HealthUnknown Health = iota // sem SMART (máquina virtual, caixa USB)
	HealthOK
	HealthOld // muitas horas ou alguns setores remapeados
	HealthBad // reprovado ou com setores pendentes/irrecuperáveis
)

// Disk é um disco físico encontrado no PC.
type Disk struct {
	Name       string // ex.: sda, nvme0n1
	Path       string // /dev/sda
	Size       int64
	Model      string
	Rotational bool // true = HD; false = SSD/NVMe
	USB        bool
	Installer  bool // é o próprio pendrive/disco do instalador

	Content    string // "Windows", "Linux", "dados" ou "" (vazio)
	Backeddy   bool   // tem parte de um volume de backups do Backeddy
	RootPart   string // partição backeddy-root existente (sistema antigo do Backeddy)
	EFIPart    string // partição EFI existente no mesmo disco
	Health     Health
	HealthNote string
}

func (d Disk) Kind() string {
	switch {
	case d.USB:
		return "USB"
	case d.Rotational:
		return "HD"
	default:
		return "SSD"
	}
}

func (d Disk) Label() string {
	m := strings.TrimSpace(d.Model)
	if m == "" {
		m = "Disco"
	}
	return fmt.Sprintf("%s %s · %s (%s)", d.Kind(), HumanSize(d.Size), m, d.Path)
}

// Short: "SSD 240 GB" — para textos corridos.
func (d Disk) Short() string { return d.Kind() + " " + HumanSize(d.Size) }

// EraseNote: o que se perde ao apagar, para o aviso ficar concreto.
func (d Disk) EraseNote() string {
	switch {
	case d.Backeddy:
		return " — tem backups do Backeddy"
	case d.Content == "Windows":
		return " — tem Windows"
	case d.Content == "Linux":
		return " — tem Linux"
	case d.Content == "dados":
		return " — tem arquivos"
	}
	return ""
}

// Target é um lugar onde os backups vão morar: um disco inteiro ou a sobra do disco do sistema.
type Target struct {
	Disk  Disk
	Whole bool // false = partição de dados no disco do sistema
}

func (t Target) Size() int64 {
	if t.Whole {
		return t.Disk.Size
	}
	return t.Disk.Size - sharedOver
}

type Mode int

const (
	ModeProtected Mode = iota
	ModeSpace
	ModeAll
	ModeOneDisk
	ModeManual
	ModeRecover
)

type Risk int

const (
	RiskGreen Risk = iota
	RiskYellow
	RiskRed
)

// Layout é um arranjo possível dos discos.
type Layout struct {
	Mode        Mode
	Risk        Risk
	Title       string // título do cartão
	Desc        string // uma linha explicando o modo
	System      Disk
	SystemWhole bool // true = disco do sistema só com o sistema
	ReuseSystem bool // recuperação: reaproveita as partições do sistema antigo (não reparticiona)
	Data        []Target
	Mirror      bool
	Usable      int64 // espaço estimado para backups
	Summary     []string
	Warns       []string // avisos amarelos
	Reds        []string // motivos de risco vermelho (exigem digitar CONTINUAR)
}

// lsblk --json --bytes
type lsblkOut struct {
	Blockdevices []lsblkDev `json:"blockdevices"`
}
type lsblkDev struct {
	Name      string     `json:"name"`
	Path      string     `json:"path"`
	Size      int64      `json:"size"`
	Type      string     `json:"type"`
	Model     *string    `json:"model"`
	Rota      bool       `json:"rota"`
	Tran      *string    `json:"tran"`
	RO        bool       `json:"ro"`
	PartLabel *string    `json:"partlabel"`
	Label     *string    `json:"label"`
	FSType    *string    `json:"fstype"`
	Children  []lsblkDev `json:"children"`
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// ScanDisks lista os discos do PC (exceto leitores de CD e discos minúsculos).
func ScanDisks() ([]Disk, error) {
	out, err := exec.Command("lsblk", "--json", "--bytes", "-o",
		"NAME,PATH,SIZE,TYPE,MODEL,ROTA,TRAN,RO,PARTLABEL,LABEL,FSTYPE").Output()
	if err != nil {
		return nil, fmt.Errorf("lsblk: %w", err)
	}
	return parseLsblk(out)
}

func parseLsblk(data []byte) ([]Disk, error) {
	var l lsblkOut
	if err := json.Unmarshal(data, &l); err != nil {
		return nil, err
	}
	var disks []Disk
	for _, b := range l.Blockdevices {
		if b.Type != "disk" || b.RO {
			continue
		}
		if strings.HasPrefix(b.Name, "zram") || strings.HasPrefix(b.Name, "loop") ||
			strings.Contains(b.Name, "boot") || strings.Contains(b.Name, "rpmb") { // mmcblk0boot0 etc.
			continue
		}
		d := Disk{Name: b.Name, Path: b.Path, Size: b.Size, Rotational: b.Rota, Model: str(b.Model)}
		if d.Path == "" {
			d.Path = "/dev/" + b.Name
		}
		d.USB = str(b.Tran) == "usb"
		for _, c := range b.Children {
			pl, lb, fs := str(c.PartLabel), str(c.Label), str(c.FSType)
			cp := c.Path
			if cp == "" {
				cp = "/dev/" + c.Name
			}
			switch {
			case pl == "backeddy-inst" || lb == "backeddy-inst":
				d.Installer = true
			case lb == "backeddy" && fs == "btrfs":
				d.Backeddy = true
			case pl == "backeddy-root":
				d.RootPart = cp
			case pl == "EFI" && fs == "vfat":
				d.EFIPart = cp
			}
			switch fs {
			case "ntfs", "BitLocker":
				d.Content = "Windows"
			case "ext4", "ext3", "xfs", "btrfs", "LVM2_member", "crypto_LUKS":
				if d.Content == "" && !(lb == "backeddy" || pl == "backeddy-root") {
					d.Content = "Linux"
				}
			case "exfat", "vfat", "hfsplus", "apfs":
				if d.Content == "" && pl != "EFI" && lb != "EFI" {
					d.Content = "dados"
				}
			}
		}
		if d.Installer || d.Size >= minDisk {
			disks = append(disks, d)
		}
	}
	return disks, nil
}

// usable devolve os discos que o instalador pode usar (sem o pendrive).
func usable(all []Disk) []Disk {
	var out []Disk
	for _, d := range all {
		if !d.Installer {
			out = append(out, d)
		}
	}
	return out
}

// sortDisks: discos internos primeiro, depois do menor para o maior.
func sortDisks(disks []Disk) {
	sort.SliceStable(disks, func(i, j int) bool {
		if disks[i].USB != disks[j].USB {
			return !disks[i].USB
		}
		return disks[i].Size < disks[j].Size
	})
}

// usableFor estima o espaço útil: soma (sem espelho) ou o que cabe com 2 cópias (espelho btrfs RAID1).
func usableFor(sizes []int64, mirror bool) int64 {
	var total, max int64
	for _, s := range sizes {
		total += s
		if s > max {
			max = s
		}
	}
	if !mirror {
		return total
	}
	half, rest := total/2, total-max
	if half < rest {
		return half
	}
	return rest
}

// build monta um Layout e calcula espaço, textos e riscos.
func build(mode Mode, sys Disk, sysWhole bool, others []Disk, mirror bool) *Layout {
	l := &Layout{Mode: mode, System: sys, SystemWhole: sysWhole, Mirror: mirror}
	if !sysWhole {
		l.Data = append(l.Data, Target{Disk: sys, Whole: false})
	}
	for _, d := range others {
		l.Data = append(l.Data, Target{Disk: d, Whole: true})
	}
	sizes := make([]int64, len(l.Data))
	for i, t := range l.Data {
		sizes[i] = t.Size()
	}
	l.Mirror = mirror && len(l.Data) >= 2
	l.Usable = usableFor(sizes, l.Mirror)
	if l.Mirror {
		l.Risk = RiskGreen
	} else {
		l.Risk = RiskYellow
	}
	l.describe(sizes)
	return l
}

func (l *Layout) describe(sizes []int64) {
	nDisks := map[string]bool{}
	for _, t := range l.Data {
		nDisks[t.Disk.Path] = true
	}
	switch {
	case len(l.Data) == 1 && !l.SystemWhole:
		l.Summary = append(l.Summary, "Sistema e backups no mesmo disco: "+l.System.Label())
	default:
		if l.SystemWhole {
			l.Summary = append(l.Summary, "Sistema: "+l.System.Label()+" (só o sistema)")
		} else {
			l.Summary = append(l.Summary, "Sistema: 20 GB do "+l.System.Label())
		}
	}
	switch {
	case l.Mirror:
		l.Summary = append(l.Summary, fmt.Sprintf("Backups: %s ESPELHADOS em %d discos. Se um disco morrer, nada se perde.",
			HumanSize(l.Usable), len(nDisks)))
		min, max := sizes[0], sizes[0]
		for _, s := range sizes {
			if s < min {
				min = s
			}
			if s > max {
				max = s
			}
		}
		var total int64
		for _, s := range sizes {
			total += s
		}
		if waste := total - 2*l.Usable; max > 0 && (max-min)*100/max >= 5 && waste > 10*GB {
			l.Warns = append(l.Warns, fmt.Sprintf("Os discos têm tamanhos diferentes: %s ficam fora do espelho.", HumanSize(waste)))
		}
	case len(l.Data) == 1:
		l.Summary = append(l.Summary, "Backups: "+HumanSize(l.Usable)+", sem cópia em outro disco.")
	default:
		l.Summary = append(l.Summary, fmt.Sprintf("Backups: %s SOMADOS em %d discos, sem proteção.", HumanSize(l.Usable), len(l.Data)))
	}
	for _, t := range l.Data {
		if t.Disk.USB && t.Whole {
			l.Warns = append(l.Warns, "Disco USB ("+t.Disk.Path+"): se o cabo soltar, os backups param. Deixe-o bem preso e na tomada.")
		}
	}
	for _, d := range l.AllDisks() {
		switch d.Health {
		case HealthBad:
			l.Reds = append(l.Reds, d.Short()+" ("+d.Path+") está falhando: "+d.HealthNote+". Pode morrer a qualquer momento.")
		case HealthOld:
			l.Warns = append(l.Warns, d.Short()+" ("+d.Path+"): "+d.HealthNote+".")
		}
	}
	if l.System.USB && l.System.Size < 128*GB {
		l.Reds = append(l.Reds, "O sistema vai ficar num pendrive ou cartão: eles se desgastam e morrem rápido rodando um sistema.")
	} else if l.System.USB && l.SystemWhole {
		l.Warns = append(l.Warns, "O sistema vai ficar num disco USB: não desconecte este disco.")
	}
	if len(l.Reds) > 0 {
		l.Risk = RiskRed
	}
}

// AllDisks devolve os discos usados pelo arranjo (sem repetição).
func (l *Layout) AllDisks() []Disk {
	seen := map[string]bool{}
	var out []Disk
	add := func(d Disk) {
		if !seen[d.Path] {
			seen[d.Path] = true
			out = append(out, d)
		}
	}
	add(l.System)
	for _, t := range l.Data {
		add(t.Disk)
	}
	return out
}

// Options calcula os modos possíveis; o primeiro é o recomendado.
// Discos com defeito (SMART) ficam de fora dos modos automáticos (só no Manual).
func Options(all []Disk) ([]*Layout, []string, error) {
	var disks, bad []Disk
	for _, d := range usable(all) {
		if d.Health == HealthBad {
			bad = append(bad, d)
			continue
		}
		disks = append(disks, d)
	}
	var notes []string
	for _, d := range bad {
		notes = append(notes, "!"+d.Short()+" ("+d.Path+") está falhando ("+d.HealthNote+") e ficou de fora. Para usar mesmo assim: Escolher os discos à mão.")
	}
	if len(disks) == 0 {
		if len(bad) > 0 {
			return nil, notes, fmt.Errorf("os únicos discos encontrados estão falhando")
		}
		return nil, notes, fmt.Errorf("nenhum disco encontrado")
	}
	sortDisks(disks)

	if len(disks) == 1 {
		d := disks[0]
		if d.Size < minShared {
			return nil, notes, fmt.Errorf("o disco %s tem só %s; o mínimo é %s", d.Path, HumanSize(d.Size), HumanSize(minShared))
		}
		l := build(ModeOneDisk, d, false, nil, false)
		l.Title, l.Desc = "Um disco só", "Sistema e backups no mesmo disco. Um segundo disco pode entrar depois e virar espelho."
		return []*Layout{l}, notes, nil
	}

	// sistema: o menor SSD interno (se houver HD junto), senão o menor disco interno
	si := 0
	hasHD := false
	for _, d := range disks {
		if d.Rotational {
			hasHD = true
		}
	}
	if hasHD {
		for i, d := range disks {
			if !d.Rotational && !d.USB && d.Size >= minSystemSD {
				si = i
				break
			}
		}
	}
	small := disks[si]
	others := append(append([]Disk{}, disks[:si]...), disks[si+1:]...)
	next := others[0]
	dedicated := !small.USB && small.Size >= minSystemSD &&
		((!small.Rotational && next.Rotational) || small.Size*2 <= next.Size)
	canShare := small.Size >= minShared

	var protected, space, allIn *Layout
	if dedicated {
		if len(others) >= 2 {
			protected = build(ModeProtected, small, true, others, true)
		}
		space = build(ModeSpace, small, true, others, false)
		if canShare {
			allIn = build(ModeAll, small, false, others, false)
			// espelho com a sobra do disco do sistema (ex.: SSD grande + HD pequeno)
			if protected == nil && small.Size-sharedOver >= minMirrorPt && (small.Size-sharedOver)*2 >= next.Size {
				protected = build(ModeProtected, small, false, others, true)
			}
		}
	} else {
		if !canShare {
			return nil, notes, fmt.Errorf("os discos são pequenos demais (%s); o mínimo é %s", HumanSize(small.Size), HumanSize(minShared))
		}
		protected = build(ModeProtected, small, false, others, true)
		space = build(ModeSpace, small, false, others, false)
	}

	var opts []*Layout
	if protected != nil {
		protected.Title, protected.Desc = "Protegido", "Cada arquivo fica em 2 discos. Se um disco morrer, nada se perde."
		opts = append(opts, protected)
	}
	if len(space.Data) == 1 {
		space.Title, space.Desc = "Um disco de backup", "Backups num disco só. Se ele morrer, você perde o histórico de backups."
	} else {
		space.Title, space.Desc = "Mais espaço", "Soma os discos de backup. Se um morrer, você perde o histórico de backups."
	}
	if protected == nil || space.Usable > protected.Usable+GB {
		opts = append(opts, space)
	}
	if allIn != nil && allIn.Usable > space.Usable+GB && (protected == nil || allIn.Usable > protected.Usable+GB) {
		allIn.Title, allIn.Desc = "Tudo junto", "Usa também a sobra do disco do sistema: cada byte para backup, sem proteção."
		opts = append(opts, allIn)
	}
	return opts, notes, nil
}

// ChooseLayout devolve só o arranjo recomendado.
func ChooseLayout(all []Disk) (*Layout, error) {
	opts, _, err := Options(all)
	if err != nil {
		return nil, err
	}
	return opts[0], nil
}

// Explain: por que o sistema foi para aquele disco (texto da tela de discos).
func Explain(l *Layout) []string {
	if l.Mode == ModeOneDisk {
		return []string{"Encontrei um disco só: o sistema e os backups vão ficar nele."}
	}
	if l.SystemWhole {
		why := "o menor"
		if !l.System.Rotational && !l.System.USB {
			why = "um SSD (rápido) e o menor"
		}
		s := fmt.Sprintf("O sistema vai no %s (%s), que é %s, separado dos backups.", l.System.Short(), l.System.Path, why)
		if l.Mirror {
			s += " Se um disco de backup falhar, os backups continuam: é só trocar o disco, sem instalar de novo."
		}
		return []string{s}
	}
	return []string{"O sistema ocupa 20 GB do primeiro disco; o resto dele e os outros discos ficam para os backups."}
}

// ---------- Manual ----------

type Role int

const (
	RoleNone Role = iota
	RoleBackup
	RoleSystem
	RoleBoth
)

func (r Role) String() string {
	switch r {
	case RoleBackup:
		return "Backup"
	case RoleSystem:
		return "Sistema"
	case RoleBoth:
		return "Sistema + backup"
	}
	return "Não usar"
}

// AllowedRoles: papéis possíveis para um disco, pelo tamanho.
func AllowedRoles(d Disk) []Role {
	r := []Role{RoleNone, RoleBackup}
	if d.Size >= minSystemSD {
		r = append(r, RoleSystem)
	}
	if d.Size >= minShared {
		r = append(r, RoleBoth)
	}
	return r
}

// DefaultRoles: papéis do arranjo recomendado (ponto de partida do Manual).
func DefaultRoles(disks []Disk, l *Layout) []Role {
	roles := make([]Role, len(disks))
	if l == nil {
		return roles
	}
	for i, d := range disks {
		for _, t := range l.Data {
			if t.Disk.Path == d.Path {
				roles[i] = RoleBackup
			}
		}
		if d.Path == l.System.Path {
			if l.SystemWhole {
				roles[i] = RoleSystem
			} else {
				roles[i] = RoleBoth
			}
		}
	}
	return roles
}

// ManualLayout monta o arranjo escolhido à mão; erro = combinação inválida (explicada).
func ManualLayout(disks []Disk, roles []Role, mirror bool) (*Layout, error) {
	var sys *Disk
	whole := true
	var others []Disk
	for i, d := range disks {
		switch roles[i] {
		case RoleSystem, RoleBoth:
			if sys != nil {
				return nil, fmt.Errorf("escolha só um disco para o sistema")
			}
			dd := d
			sys = &dd
			whole = roles[i] == RoleSystem
		case RoleBackup:
			others = append(others, d)
		}
	}
	if sys == nil {
		return nil, fmt.Errorf("escolha um disco para o sistema")
	}
	if whole && len(others) == 0 {
		return nil, fmt.Errorf("escolha pelo menos um disco para os backups")
	}
	sortDisks(others)
	l := build(ModeManual, *sys, whole, others, mirror)
	l.Title = "Escolhido à mão"
	return l, nil
}

// ---------- Recuperação ----------

// RecoverLayout: reinstala só o sistema e mantém o volume de backups existente.
func RecoverLayout(all []Disk) (*Layout, error) {
	disks := usable(all)
	// 1) o disco do sistema antigo ainda existe (sistema + backups no mesmo disco): refaz só o sistema
	for _, d := range disks {
		if d.Backeddy && d.RootPart != "" && d.EFIPart != "" {
			return &Layout{Mode: ModeRecover, Risk: RiskGreen, System: d, ReuseSystem: true,
				Summary: []string{"Sistema: refeito no " + d.Label() + ", na parte do sistema antigo.",
					"Backups: mantidos, sem tocar em nada."}}, nil
		}
	}
	// 2) um disco sem backups para receber o sistema (ex.: SSD novo no lugar do que morreu)
	var free []Disk
	for _, d := range disks {
		if !d.Backeddy && d.Size >= minSystemSD && d.Health != HealthBad {
			free = append(free, d)
		}
	}
	if len(free) == 0 {
		return nil, fmt.Errorf("para recuperar, coloque um disco para o sistema (16 GB ou mais) além dos discos de backup")
	}
	sortDisks(free)
	// prefere SSD interno
	pick := free[0]
	for _, d := range free {
		if !d.Rotational && !d.USB {
			pick = d
			break
		}
	}
	l := &Layout{Mode: ModeRecover, Risk: RiskGreen, System: pick, SystemWhole: true,
		Summary: []string{"Sistema: " + pick.Label() + " (será apagado).", "Backups: mantidos, sem tocar em nada."}}
	if pick.USB && pick.Size < 128*GB {
		l.Risk = RiskRed
		l.Reds = append(l.Reds, "O sistema vai ficar num pendrive ou cartão: eles se desgastam e morrem rápido rodando um sistema.")
	}
	return l, nil
}

// HasBackeddy: algum disco tem backups do Backeddy?
func HasBackeddy(all []Disk) bool {
	for _, d := range usable(all) {
		if d.Backeddy {
			return true
		}
	}
	return false
}

func HumanSize(b int64) string {
	const tb = int64(1) << 40
	switch {
	case b >= tb:
		return trimZero(fmt.Sprintf("%.1f TB", float64(b)/float64(tb)))
	case b >= GB:
		return fmt.Sprintf("%d GB", b/GB)
	default:
		return fmt.Sprintf("%d MB", b>>20)
	}
}

func trimZero(s string) string { return strings.Replace(s, ".0 ", " ", 1) }
