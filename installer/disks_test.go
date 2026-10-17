package main

import (
	"strings"
	"testing"
)

func d(name string, gb int64, rota, usb bool) Disk {
	return Disk{Name: name, Path: "/dev/" + name, Size: gb * GB, Rotational: rota, USB: usb, Model: "Teste"}
}

func TestOneDisk(t *testing.T) {
	l, err := ChooseLayout([]Disk{d("sda", 512, false, false)})
	if err != nil {
		t.Fatal(err)
	}
	if l.System.Name != "sda" || len(l.Data) != 1 || l.Data[0].Whole || l.Mirror {
		t.Fatalf("layout errado: %+v", l)
	}
	if l.Usable != 512*GB-rootSize-GB {
		t.Fatalf("usable %d", l.Usable)
	}
}

func TestOneDiskTooSmall(t *testing.T) {
	if _, err := ChooseLayout([]Disk{d("sda", 32, false, false)}); err == nil {
		t.Fatal("deveria recusar disco de 32 GB")
	}
}

func TestSSDPlusTwoHDs(t *testing.T) {
	l, err := ChooseLayout([]Disk{d("sdb", 2000, true, false), d("sda", 120, false, false), d("sdc", 2000, true, false)})
	if err != nil {
		t.Fatal(err)
	}
	if l.System.Name != "sda" || len(l.Data) != 2 || !l.Mirror || !l.Data[0].Whole {
		t.Fatalf("layout errado: %+v", l)
	}
	if l.Usable != 2000*GB {
		t.Fatalf("usable %d", l.Usable)
	}
	if len(l.Warns) != 0 {
		t.Fatalf("não deveria avisar: %v", l.Warns)
	}
}

func TestSSDPlusOneHD(t *testing.T) {
	l, _ := ChooseLayout([]Disk{d("sda", 240, false, false), d("sdb", 1000, true, false)})
	if l.System.Name != "sda" || len(l.Data) != 1 || l.Mirror || l.Usable != 1000*GB {
		t.Fatalf("layout errado: %+v", l)
	}
}

func TestTwoEqualHDs(t *testing.T) {
	l, _ := ChooseLayout([]Disk{d("sda", 1000, true, false), d("sdb", 1000, true, false)})
	if len(l.Data) != 2 || !l.Mirror || l.Data[0].Whole || !l.Data[1].Whole {
		t.Fatalf("layout errado: %+v", l)
	}
	// espelho limitado pela parte de dados do disco do sistema
	if l.Usable != 1000*GB-rootSize-GB {
		t.Fatalf("usable %d", l.Usable)
	}
}

func TestDifferentSizesWarn(t *testing.T) {
	l, _ := ChooseLayout([]Disk{d("sda", 128, false, false), d("sdb", 1000, true, false), d("sdc", 2000, true, false)})
	if !l.Mirror || l.Usable != 1000*GB || len(l.Warns) == 0 {
		t.Fatalf("esperava espelho de 1 TB com aviso: %+v", l)
	}
}

func TestLaptopWithUSB(t *testing.T) {
	l, _ := ChooseLayout([]Disk{d("sdb", 1000, true, true), d("sda", 256, false, false)})
	if l.System.Name != "sda" || !l.Data[0].Whole || l.Data[0].Disk.Name != "sdb" {
		t.Fatalf("layout errado: %+v", l)
	}
	if len(l.Warns) == 0 || !strings.Contains(l.Warns[0], "USB") {
		t.Fatalf("esperava aviso de USB: %v", l.Warns)
	}
}

func TestInstallerExcluded(t *testing.T) {
	inst := d("sdz", 16, false, true)
	inst.Installer = true
	l, err := ChooseLayout([]Disk{inst, d("sda", 500, false, false)})
	if err != nil || l.System.Name != "sda" {
		t.Fatalf("instalador não foi ignorado: %+v %v", l, err)
	}
	if _, err := ChooseLayout([]Disk{inst}); err == nil {
		t.Fatal("só com o pendrive deveria dar erro")
	}
}

func TestParseLsblk(t *testing.T) {
	js := `{"blockdevices":[
	 {"name":"sda","path":"/dev/sda","size":17179869184,"type":"disk","model":"Virtual Disk","rota":true,"tran":null,"ro":false,"partlabel":null,"label":null,
	  "children":[{"name":"sda1","path":"/dev/sda1","size":1,"type":"part","model":null,"rota":true,"tran":null,"ro":false,"partlabel":"EFI","label":"EFI"},
	              {"name":"sda2","path":"/dev/sda2","size":1,"type":"part","model":null,"rota":true,"tran":null,"ro":false,"partlabel":"backeddy-inst","label":"backeddy-inst"}]},
	 {"name":"sdb","path":"/dev/sdb","size":1099511627776,"type":"disk","model":"Virtual Disk","rota":true,"tran":null,"ro":false,"partlabel":null,"label":null},
	 {"name":"sr0","path":"/dev/sr0","size":1073741312,"type":"rom","model":"DVD","rota":true,"tran":"sata","ro":false,"partlabel":null,"label":null},
	 {"name":"sdc","path":"/dev/sdc","size":4000000000,"type":"disk","model":"Card","rota":false,"tran":"usb","ro":false,"partlabel":null,"label":null}
	]}`
	disks, err := parseLsblk([]byte(js))
	if err != nil {
		t.Fatal(err)
	}
	if len(disks) != 2 || !disks[0].Installer || disks[1].Name != "sdb" {
		t.Fatalf("parse errado: %+v", disks)
	}
}

func TestPlanCheck(t *testing.T) {
	p := &Plan{MinBytes: 750 * GB, RecommendedBytes: 1000 * GB}
	if p.Check(2000*GB) != FitGreen || p.Check(800*GB) != FitYellow || p.Check(500*GB) != FitRed {
		t.Fatal("faixas erradas")
	}
	var none *Plan
	if none.Check(1) != FitUnknown || none.TMPercent() != 60 {
		t.Fatal("plano nulo deveria ser neutro")
	}
	mac := &Plan{Computers: []PlanComputer{{Name: "Mac", Kind: "mac"}}}
	if mac.TMPercent() != 85 {
		t.Fatal("só Mac deveria dar 85%")
	}
	both := &Plan{Computers: []PlanComputer{{Name: "MacBook do Eddy", Kind: "mac"}, {Name: "PC da Sala", Kind: "windows"}}}
	if both.TMPercent() != 60 || both.ComputerNames() != `"MacBook do Eddy" e "PC da Sala"` {
		t.Fatalf("plano misto: %d %s", both.TMPercent(), both.ComputerNames())
	}
}

func TestPartPath(t *testing.T) {
	if partPath("/dev/sda", 2) != "/dev/sda2" || partPath("/dev/nvme0n1", 1) != "/dev/nvme0n1p1" {
		t.Fatal("partPath errado")
	}
}

func TestDryRunInstall(t *testing.T) {
	l, _ := ChooseLayout([]Disk{d("sda", 120, false, false), d("sdb", 1000, true, false), d("sdc", 1000, true, false)})
	in := &Installer{Layout: l, Name: "Backup da Casa", Password: "segredo123", DryRun: true}
	var steps []int
	if err := in.Run(func(s int) { steps = append(steps, s) }); err != nil {
		t.Fatal(err)
	}
	if len(steps) != len(Steps) || len(in.dataUUIDs) != 2 || in.rootPart != "/dev/sda2" {
		t.Fatalf("dry-run errado: steps=%v uuids=%v root=%s", steps, in.dataUUIDs, in.rootPart)
	}
}
