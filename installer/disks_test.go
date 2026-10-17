package main

import (
	"fmt"
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

func titles(opts []*Layout) string {
	var t []string
	for _, o := range opts {
		t = append(t, fmt.Sprintf("%s=%s", o.Title, HumanSize(o.Usable)))
	}
	return strings.Join(t, " | ")
}

func TestOptionsMatrix(t *testing.T) {
	cases := []struct {
		name  string
		disks []Disk
		want  string
	}{
		{"1 disco", []Disk{d("sda", 512, false, false)}, "Um disco só=491 GB"},
		{"SSD + 1 HD", []Disk{d("sda", 240, false, false), d("sdb", 1000, true, false)}, "Um disco de backup=1000 GB | Tudo junto=1.2 TB"},
		{"SSD + 2 HDs iguais", []Disk{d("sda", 240, false, false), d("sdb", 2000, true, false), d("sdc", 2000, true, false)},
			"Protegido=2 TB | Mais espaço=3.9 TB | Tudo junto=4.1 TB"},
		{"SSD + 2 HDs diferentes", []Disk{d("sda", 240, false, false), d("sdb", 1024, true, false), d("sdc", 4096, true, false)},
			"Protegido=1 TB | Mais espaço=5 TB | Tudo junto=5.2 TB"},
		{"2 HDs iguais sem SSD", []Disk{d("sda", 1000, true, false), d("sdb", 1000, true, false)},
			"Protegido=979 GB | Mais espaço=1.9 TB"},
		{"SSD grande + HD pequeno", []Disk{d("sda", 1000, false, false), d("sdb", 500, true, false)},
			"Protegido=500 GB | Tudo junto=1.4 TB"},
		{"teste Hyper-V", []Disk{d("sdc", 24, true, false), d("sda", 60, true, false), d("sdb", 60, true, false)},
			"Protegido=60 GB | Mais espaço=120 GB"},
	}
	for _, c := range cases {
		opts, _, err := Options(c.disks)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got := titles(opts); got != c.want {
			t.Errorf("%s:\n got  %s\n want %s", c.name, got, c.want)
		}
	}
}

func TestBadDiskLeftOut(t *testing.T) {
	bad := d("sdc", 2000, true, false)
	bad.Health, bad.HealthNote = HealthBad, "8 setores ilegíveis"
	opts, notes, err := Options([]Disk{d("sda", 240, false, false), d("sdb", 2000, true, false), bad})
	if err != nil || len(notes) != 1 || opts[0].Mirror {
		t.Fatalf("disco ruim deveria ficar de fora: %s %v %v", titles(opts), notes, err)
	}
	// no Manual ele pode entrar, mas o risco fica vermelho
	l, err := ManualLayout([]Disk{d("sda", 240, false, false), d("sdb", 2000, true, false), bad},
		[]Role{RoleSystem, RoleBackup, RoleBackup}, true)
	if err != nil || l.Risk != RiskRed || !l.Mirror {
		t.Fatalf("manual com disco ruim: %+v %v", l, err)
	}
}

func TestManualValidation(t *testing.T) {
	ds := []Disk{d("sda", 240, false, false), d("sdb", 1000, true, false)}
	if _, err := ManualLayout(ds, []Role{RoleBackup, RoleBackup}, true); err == nil {
		t.Fatal("sem disco de sistema deveria dar erro")
	}
	if _, err := ManualLayout(ds, []Role{RoleSystem, RoleSystem}, true); err == nil {
		t.Fatal("dois discos de sistema deveria dar erro")
	}
	if _, err := ManualLayout(ds, []Role{RoleSystem, RoleNone}, true); err == nil {
		t.Fatal("sem backup deveria dar erro")
	}
	l, err := ManualLayout(ds, []Role{RoleBoth, RoleBackup}, true)
	if err != nil || !l.Mirror || l.Risk != RiskGreen || len(l.AllDisks()) != 2 {
		t.Fatalf("sistema+backup espelhado: %+v %v", l, err)
	}
	usb := d("sdc", 32, false, true)
	l, err = ManualLayout([]Disk{usb, d("sdb", 1000, true, false)}, []Role{RoleSystem, RoleBackup}, false)
	if err != nil || l.Risk != RiskRed {
		t.Fatalf("sistema em pendrive deveria ser vermelho: %+v %v", l, err)
	}
	if r := DefaultRoles(ds, &Layout{System: ds[0], SystemWhole: true, Data: []Target{{Disk: ds[1], Whole: true}}}); r[0] != RoleSystem || r[1] != RoleBackup {
		t.Fatalf("papéis padrão: %v", r)
	}
}

func TestRecover(t *testing.T) {
	// SSD do sistema morreu e foi trocado: sistema vai no SSD novo, backups intactos
	b1, b2 := d("sdb", 2000, true, false), d("sdc", 2000, true, false)
	b1.Backeddy, b2.Backeddy = true, true
	l, err := RecoverLayout([]Disk{b1, d("sda", 240, false, false), b2})
	if err != nil || l.System.Name != "sda" || !l.SystemWhole || l.ReuseSystem || len(l.Data) != 0 {
		t.Fatalf("recuperação com SSD novo: %+v %v", l, err)
	}
	// só os discos de backup: falta disco para o sistema
	if _, err := RecoverLayout([]Disk{b1, b2}); err == nil {
		t.Fatal("deveria pedir um disco para o sistema")
	}
	// disco único (sistema + backups) ainda vivo: refaz só o sistema no lugar
	one := d("sda", 1000, true, false)
	one.Backeddy, one.RootPart, one.EFIPart = true, "/dev/sda2", "/dev/sda1"
	l, err = RecoverLayout([]Disk{one})
	if err != nil || !l.ReuseSystem {
		t.Fatalf("recuperação no mesmo disco: %+v %v", l, err)
	}
}

func TestJudgeSmart(t *testing.T) {
	cases := map[string]Health{
		`{"smart_status":{"passed":true},"power_on_time":{"hours":1200}}`:                                                   HealthOK,
		`{"smart_status":{"passed":false}}`:                                                                                 HealthBad,
		`{"smart_status":{"passed":true},"ata_smart_attributes":{"table":[{"id":197,"raw":{"value":8}}]}}`:                  HealthBad,
		`{"smart_status":{"passed":true},"ata_smart_attributes":{"table":[{"id":5,"raw":{"value":3}}]}}`:                    HealthOld,
		`{"smart_status":{"passed":true},"power_on_time":{"hours":61000}}`:                                                  HealthOld,
		`{"smart_status":{"passed":true},"nvme_smart_health_information_log":{"critical_warning":0,"percentage_used":100}}`: HealthOld,
		`{}`: HealthUnknown,
		``:   HealthUnknown,
	}
	for js, want := range cases {
		if got, _ := judgeSmart([]byte(js)); got != want {
			t.Errorf("%s: %v, esperado %v", js, got, want)
		}
	}
}

func TestParseContent(t *testing.T) {
	js := `{"blockdevices":[
	 {"name":"sda","path":"/dev/sda","size":256060514304,"type":"disk","model":"SSD","rota":false,"tran":"sata","ro":false,
	  "children":[{"name":"sda1","path":"/dev/sda1","size":1,"type":"part","partlabel":"EFI system partition","label":null,"fstype":"vfat"},
	              {"name":"sda3","path":"/dev/sda3","size":1,"type":"part","partlabel":"Basic data partition","label":"Windows","fstype":"ntfs"}]},
	 {"name":"sdb","path":"/dev/sdb","size":2000398934016,"type":"disk","model":"HD","rota":true,"tran":"sata","ro":false,
	  "children":[{"name":"sdb1","path":"/dev/sdb1","size":1,"type":"part","partlabel":"backeddy-data","label":"backeddy","fstype":"btrfs"}]},
	 {"name":"sdc","path":"/dev/sdc","size":1000204886016,"type":"disk","model":"HD","rota":true,"tran":"sata","ro":false,
	  "children":[{"name":"sdc1","path":"/dev/sdc1","size":1,"type":"part","partlabel":"EFI","label":"EFI","fstype":"vfat"},
	              {"name":"sdc2","path":"/dev/sdc2","size":1,"type":"part","partlabel":"backeddy-root","label":"backeddy-root","fstype":"ext4"},
	              {"name":"sdc3","path":"/dev/sdc3","size":1,"type":"part","partlabel":"backeddy-data","label":"backeddy","fstype":"btrfs"}]},
	 {"name":"mmcblk0boot0","path":"/dev/mmcblk0boot0","size":4194304,"type":"disk","rota":false,"ro":true}
	]}`
	ds, err := parseLsblk([]byte(js))
	if err != nil || len(ds) != 3 {
		t.Fatalf("parse: %+v %v", ds, err)
	}
	if ds[0].Content != "Windows" || !strings.Contains(ds[0].EraseNote(), "Windows") {
		t.Errorf("sda deveria ter Windows: %+v", ds[0])
	}
	if !ds[1].Backeddy || ds[1].Content != "" {
		t.Errorf("sdb deveria ser backup do Backeddy: %+v", ds[1])
	}
	if !ds[2].Backeddy || ds[2].RootPart != "/dev/sdc2" || ds[2].EFIPart != "/dev/sdc1" {
		t.Errorf("sdc deveria ter sistema antigo: %+v", ds[2])
	}
	if !HasBackeddy(ds) {
		t.Error("deveria achar backups")
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
