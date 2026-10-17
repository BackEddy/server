package main

// Detecção de discos e "receitas": decide sozinho onde fica o sistema e onde ficam os backups.

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

const (
	GB          = int64(1) << 30
	minDisk     = 8 * GB  // discos menores são ignorados (pendrives, cartões)
	rootSize    = 20 * GB // partição do sistema
	minShared   = 40 * GB // disco único precisa caber sistema + algum backup
	minSystemSD = 16 * GB // disco dedicado ao sistema
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

// Target é um lugar onde os backups vão morar: um disco inteiro ou a sobra do disco do sistema.
type Target struct {
	Disk  Disk
	Whole bool // false = partição de dados no disco do sistema
}

// Layout é a receita escolhida.
type Layout struct {
	System  Disk
	Data    []Target
	Mirror  bool
	Usable  int64 // espaço estimado para backups
	Summary []string
	Warns   []string
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
	Children  []lsblkDev `json:"children"`
}

// ScanDisks lista os discos do PC (exceto o do instalador, leitores de CD e discos minúsculos).
func ScanDisks() ([]Disk, error) {
	out, err := exec.Command("lsblk", "--json", "--bytes", "-o",
		"NAME,PATH,SIZE,TYPE,MODEL,ROTA,TRAN,RO,PARTLABEL,LABEL").Output()
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
		if b.Type != "disk" || b.RO || b.Size < minDisk {
			continue
		}
		if strings.HasPrefix(b.Name, "zram") || strings.HasPrefix(b.Name, "loop") {
			continue
		}
		d := Disk{Name: b.Name, Path: b.Path, Size: b.Size, Rotational: b.Rota}
		if d.Path == "" {
			d.Path = "/dev/" + b.Name
		}
		if b.Model != nil {
			d.Model = *b.Model
		}
		if b.Tran != nil && *b.Tran == "usb" {
			d.USB = true
		}
		for _, c := range b.Children {
			if (c.PartLabel != nil && *c.PartLabel == "backeddy-inst") || (c.Label != nil && *c.Label == "backeddy-inst") {
				d.Installer = true
			}
		}
		disks = append(disks, d)
	}
	return disks, nil
}

// ChooseLayout aplica as receitas:
//   - 1 disco: sistema + backups no mesmo disco
//   - disco pequeno/SSD + outros: sistema no pequeno, backups nos outros (2+ = espelho)
//   - discos parecidos: sistema numa parte do primeiro, backups no resto + demais discos (espelho)
func ChooseLayout(all []Disk) (*Layout, error) {
	var disks []Disk
	for _, d := range all {
		if !d.Installer {
			disks = append(disks, d)
		}
	}
	if len(disks) == 0 {
		return nil, fmt.Errorf("nenhum disco encontrado para instalar (só o pendrive do instalador)")
	}
	// discos internos primeiro, depois do menor para o maior
	sort.SliceStable(disks, func(i, j int) bool {
		if disks[i].USB != disks[j].USB {
			return !disks[i].USB
		}
		return disks[i].Size < disks[j].Size
	})

	l := &Layout{}
	if len(disks) == 1 {
		d := disks[0]
		if d.Size < minShared {
			return nil, fmt.Errorf("o disco %s tem só %s; o mínimo é %s", d.Path, HumanSize(d.Size), HumanSize(minShared))
		}
		l.System = d
		l.Data = []Target{{Disk: d, Whole: false}}
		l.Usable = d.Size - rootSize - GB
		l.Summary = []string{
			"Sistema e backups no mesmo disco: " + d.Label(),
			"Backups: " + HumanSize(l.Usable) + " (sem cópia em outro disco)",
		}
		if d.USB {
			l.Warns = append(l.Warns, "O único disco é USB: não desconecte este disco.")
		}
		return l, nil
	}

	small, next := disks[0], disks[1]
	dedicated := !small.USB && small.Size >= minSystemSD &&
		((!small.Rotational && next.Rotational) || small.Size*2 <= next.Size)

	if dedicated {
		l.System = small
		for _, d := range disks[1:] {
			l.Data = append(l.Data, Target{Disk: d, Whole: true})
		}
	} else {
		if small.Size < minShared {
			return nil, fmt.Errorf("os discos são pequenos demais (%s); o mínimo é %s", HumanSize(small.Size), HumanSize(minShared))
		}
		l.System = small
		l.Data = append(l.Data, Target{Disk: small, Whole: false})
		for _, d := range disks[1:] {
			l.Data = append(l.Data, Target{Disk: d, Whole: true})
		}
	}

	sizes := make([]int64, len(l.Data))
	for i, t := range l.Data {
		sizes[i] = t.Disk.Size
		if !t.Whole {
			sizes[i] = t.Disk.Size - rootSize - GB
		}
	}
	l.Mirror = len(l.Data) >= 2
	l.Usable = usableFor(sizes, l.Mirror)

	l.Summary = append(l.Summary, "Sistema: "+l.System.Label())
	if l.Mirror {
		l.Summary = append(l.Summary, fmt.Sprintf("Backups: %d discos ESPELHADOS = %s protegidos", len(l.Data), HumanSize(l.Usable)))
		l.Summary = append(l.Summary, "Se um disco morrer, nada se perde.")
		min, max := sizes[0], sizes[0]
		for _, s := range sizes {
			if s < min {
				min = s
			}
			if s > max {
				max = s
			}
		}
		if (max-min)*100/max >= 5 {
			l.Warns = append(l.Warns, "Os discos de backup têm tamanhos diferentes: parte do maior não entra no espelho.")
		}
	} else {
		l.Summary = append(l.Summary, "Backups: "+l.Data[0].Disk.Label())
		l.Summary = append(l.Summary, "Espaço: "+HumanSize(l.Usable)+" (sem cópia em outro disco)")
	}
	for _, t := range l.Data {
		if t.Disk.USB {
			l.Warns = append(l.Warns, "Há disco USB nos backups: não desconecte "+t.Disk.Path+".")
		}
	}
	return l, nil
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

// AllDisks devolve os discos que serão apagados (sem repetição).
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
