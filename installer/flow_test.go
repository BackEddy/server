package main

// Roteiro completo do instalador com uma interface de mentira: cada tela recebe a resposta
// programada, e o teste confere a ordem das telas e o arranjo final (em dry-run, sem apagar nada).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type answer struct {
	title  string // trecho do título esperado
	choose int
	card   int
	btn    int
	input  string
	roles  []Role
	mirror bool
	ok     bool
}

type fakeUI struct {
	t       *testing.T
	script  []answer
	seen    []string
	lastIn  *Installer
	stageAt []int
}

func (f *fakeUI) next(title string) answer {
	f.seen = append(f.seen, title)
	if len(f.script) == 0 {
		f.t.Fatalf("tela sem resposta programada: %q (vistas: %v)", title, f.seen)
	}
	a := f.script[0]
	f.script = f.script[1:]
	if !strings.Contains(title, a.title) {
		f.t.Fatalf("esperava tela %q, veio %q (vistas: %v)", a.title, title, f.seen)
	}
	return a
}

func (f *fakeUI) Stage(i int)                            { f.stageAt = append(f.stageAt, i) }
func (f *fakeUI) Notice(string, Tone, []string)          {}
func (f *fakeUI) Progress(string, []string, int, string) {}
func (f *fakeUI) Close()                                 {}
func (f *fakeUI) Choose(title string, _ Tone, _ []string, _ []string, _ int, _ <-chan struct{}) int {
	return f.next(title).choose
}
func (f *fakeUI) Input(title string, _ []string, _ string, initial string, _ bool) string {
	a := f.next(title)
	if a.input == "" {
		return initial
	}
	return a.input
}
func (f *fakeUI) Cards(title string, _ []string, _ []Card, _ int, _ []string) (int, int) {
	a := f.next(title)
	return a.card, a.btn
}
func (f *fakeUI) Manual(title string, _ []string, disks []Disk, roles []Role, mirror bool,
	preview func([]Role, bool) ([]string, bool)) ([]Role, bool, bool) {
	a := f.next(title)
	if a.ok {
		if _, valid := preview(a.roles, a.mirror); !valid {
			f.t.Fatalf("papéis inválidos no roteiro: %v", a.roles)
		}
	}
	return a.roles, a.mirror, a.ok
}

const hyperV = `{"blockdevices":[
 {"name":"sda","path":"/dev/sda","size":64424509440,"type":"disk","model":"Virtual Disk","rota":true,"ro":false},
 {"name":"sdb","path":"/dev/sdb","size":64424509440,"type":"disk","model":"Virtual Disk","rota":true,"ro":false},
 {"name":"sdc","path":"/dev/sdc","size":25769803776,"type":"disk","model":"Virtual Disk","rota":true,"ro":false},
 {"name":"sdd","path":"/dev/sdd","size":1610612736,"type":"disk","model":"Virtual Disk","rota":true,"ro":false,
  "children":[{"name":"sdd2","path":"/dev/sdd2","size":1,"type":"part","partlabel":"backeddy-inst","label":"backeddy-inst","fstype":"ext4"}]}
]}`

const reinstall = `{"blockdevices":[
 {"name":"sda","path":"/dev/sda","size":25769803776,"type":"disk","model":"SSD novo","rota":false,"ro":false},
 {"name":"sdb","path":"/dev/sdb","size":64424509440,"type":"disk","model":"HD","rota":true,"ro":false,
  "children":[{"name":"sdb1","path":"/dev/sdb1","size":1,"type":"part","partlabel":"backeddy-data","label":"backeddy","fstype":"btrfs"}]},
 {"name":"sdc","path":"/dev/sdc","size":64424509440,"type":"disk","model":"HD","rota":true,"ro":false,
  "children":[{"name":"sdc1","path":"/dev/sdc1","size":1,"type":"part","partlabel":"backeddy-data","label":"backeddy","fstype":"btrfs"}]}
]}`

func runFlow(t *testing.T, lsblk string, script []answer) *fakeUI {
	t.Helper()
	p := filepath.Join(t.TempDir(), "lsblk.json")
	if err := os.WriteFile(p, []byte(lsblk), 0o644); err != nil {
		t.Fatal(err)
	}
	f := &fakeUI{t: t, script: append([]answer{{title: "Idioma"}}, script...)}
	if err := run(f, true, p, filepath.Join(t.TempDir(), "sem-plano.json")); err != nil {
		t.Fatalf("roteiro terminou com %v (vistas: %v)", err, f.seen)
	}
	if len(f.script) != 0 {
		t.Fatalf("sobraram respostas: %v (vistas: %v)", f.script, f.seen)
	}
	return f
}

var tail = []answer{
	{title: "Atenção"}, {title: "Confirme", input: "APAGAR"},
	{title: "Nome"},
	{title: "Senha", input: "segredo1"}, {title: "Senha", input: "segredo1"},
	{title: "Teste do alarme", choose: 1}, {title: "Teste do alarme", choose: 0}, {title: "Silêncio", card: 2}, {title: "Começo", input: "23h"}, {title: "Fim", input: "x"}, {title: "Fim", input: "7"},
	{title: "Pronto"},
}

func TestFlowRecommended(t *testing.T) {
	runFlow(t, hyperV, append([]answer{{title: "Transforme"}, {title: "Como usar os discos", card: 0, btn: 0}}, tail...))
}

func TestFlowMoreSpaceAsksForRisk(t *testing.T) {
	runFlow(t, hyperV, append([]answer{{title: "Transforme"},
		{title: "Como usar os discos", card: 1, btn: 0},
		{title: "Sem proteção", choose: 1}, // Voltar
		{title: "Como usar os discos", card: 1, btn: 0},
		{title: "Sem proteção", choose: 0}, // Entendi
	}, tail...))
}

func TestFlowManualSystemOnUSBIsRed(t *testing.T) {
	// sda/sdb/sdc ordenados por tamanho: sdc (24), sda (60), sdb (60)
	runFlow(t, hyperV, append([]answer{{title: "Transforme"},
		{title: "Como usar os discos", card: 0, btn: 1},
		{title: "à mão", roles: []Role{RoleBoth, RoleBackup, RoleNone}, mirror: true, ok: false}, // Voltar
		{title: "Como usar os discos", card: 0, btn: 1},
		{title: "à mão", roles: []Role{RoleNone, RoleBoth, RoleBackup}, mirror: true, ok: true},
	}, tail...))
}

func TestFlowRecover(t *testing.T) {
	runFlow(t, reinstall, []answer{{title: "Transforme"},
		{title: "Encontrei backups", card: 0, btn: 0},
		{title: "Recuperar o servidor", choose: 1}, // Voltar
		{title: "Encontrei backups", card: 0, btn: 0},
		{title: "Recuperar o servidor", choose: 0},
		{title: "Atenção"}, {title: "Confirme", input: "APAGAR"}, // só o SSD novo
		{title: "Senha", input: "segredo1"}, {title: "Senha", input: "segredo1"},
		{title: "Teste do alarme", choose: 2}, {title: "Sem alto-falante"},
		{title: "Pronto"},
	})
}

func TestFlowEraseOldBackups(t *testing.T) {
	runFlow(t, reinstall, append([]answer{{title: "Transforme"},
		{title: "Encontrei backups", card: 1, btn: 0},
		{title: "Como usar os discos", card: 0, btn: 0},
	}, tail...))
}
