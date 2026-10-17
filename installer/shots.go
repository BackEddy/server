package main

// --shots PASTA: desenha as telas do instalador gráfico em PNG, sem framebuffer nem discos.
// Serve para revisar o visual antes de gerar a imagem do pendrive.

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
)

func d0(name string, gb int64, rota bool) Disk {
	return Disk{Name: name, Path: "/dev/" + name, Size: gb * GB, Rotational: rota, Model: "Modelo"}
}

func runShots(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	disks := []string{"• HD 24 GB · Virtual Disk (/dev/sdc)", "• HD 60 GB · Virtual Disk (/dev/sda)", "• HD 60 GB · Virtual Disk (/dev/sdb)"}
	type shot struct {
		name string
		draw func(g *GUI)
	}
	list := []shot{
		{"01-boas-vindas", func(g *GUI) {
			g.stage = StageIntro
			g.drawChoose("Transforme este computador em um lugar seguro para os backups da sua casa", ToneNormal, []string{
				"Macs (Time Machine) e PCs com Windows vão poder fazer backup aqui, automaticamente.",
				"", "A instalação leva uns 5 minutos.",
			}, []string{"Começar", "Desligar"}, 0)
		}},
		{"02a-modos", func(g *GUI) {
			g.stage = StageDisks
			ds := []Disk{d0("sda", 240, false), d0("sdb", 2000, true), d0("sdc", 2000, true)}
			opts, _, _ := Options(ds)
			cards := make([]Card, len(opts))
			for i, o := range opts {
				cards[i] = Card{Title: o.Title, Value: HumanSize(o.Usable), Desc: o.Desc, Risk: o.Risk, Recommended: i == 0}
			}
			g.hover = hitCard + 1
			g.drawCards("Como usar os discos", Explain(opts[0]), cards, 0, []string{"Continuar", "Escolher os discos à mão", "Desligar"}, 0)
			g.hover = 0
		}},
		{"02b-manual", func(g *GUI) {
			g.stage = StageDisks
			ds := []Disk{d0("sda", 240, false), d0("sdb", 2000, true), d0("sdc", 2000, true), d0("sdd", 1000, true)}
			ds[3].Health, ds[3].Content = HealthBad, "Windows"
			roles := []Role{RoleSystem, RoleBackup, RoleBackup, RoleNone}
			l, _ := ManualLayout(ds, roles, true)
			g.drawManual("Escolher os discos à mão", []string{"Para cada disco: Sistema, Backup, Sistema + backup ou Não usar (fica intocado)."},
				ds, roles, true, 1, l.Summary)
		}},
		{"02c-sem-protecao", func(g *GUI) {
			g.stage = StageDisks
			ds := []Disk{d0("sda", 240, false), d0("sdb", 2000, true), d0("sdc", 2000, true)}
			opts, _, _ := Options(ds)
			l := opts[1]
			lines := append([]string{}, l.Summary...)
			lines = append(lines, "", "Se um disco morrer, você perde o histórico de backups. Os arquivos nos Macs e PCs continuam, e o servidor recomeça do zero.",
				"", "Dá para ligar o espelho depois, sem reinstalar, se houver espaço livre: sudo backeddy mirror")
			g.drawChoose("Sem proteção contra disco quebrado", ToneWarn, lines, []string{"Entendi, continuar", "Voltar"}, 0)
		}},
		{"02d-recuperar", func(g *GUI) {
			g.stage = StageIntro
			g.drawCards("Encontrei backups do Backeddy", []string{
				"Este computador já foi um Backeddy Server. Dá para reinstalar só o sistema e manter todos os backups: os Macs e PCs continuam de onde pararam.",
			}, []Card{
				{Title: "Recuperar o servidor", Desc: "Reinstala o sistema e mantém os backups e as configurações.", Risk: RiskGreen, Recommended: true},
				{Title: "Apagar tudo e começar do zero", Desc: "Os backups antigos serão apagados para sempre.", Risk: RiskRed},
			}, 0, []string{"Continuar", "Desligar"}, 0)
		}},
		{"02-discos", func(g *GUI) {
			g.stage = StageDisks
			g.drawChoose("Encontrei estes discos", ToneNormal, []string{
				"Sistema: HD 24 GB · Virtual Disk (/dev/sdc)",
				"Backups: 2 discos ESPELHADOS = 60 GB protegidos",
				"Se um disco morrer, nada se perde.",
				"", "+Cabe com folga: o recomendado no plano é 40 GB.",
			}, []string{"Continuar", "Desligar"}, 0)
		}},
		{"03-espaco-pouco", func(g *GUI) {
			g.stage = StageDisks
			g.drawChoose("O espaço não é suficiente", ToneDanger, []string{
				"Quando você criou este pendrive, o recomendado era pelo menos 1 TB.",
				"Este computador só tem 512 GB para backups.",
				"", "Sugestão: instale um disco de 2 TB ou maior e comece de novo.",
			}, []string{"Desligar", "Continuar mesmo assim"}, 0)
		}},
		{"04-atencao", func(g *GUI) {
			g.stage = StageConfirm
			lines := append([]string{"Estes discos vão ser apagados por completo:", ""}, disks...)
			lines = append(lines, "", "Windows, fotos, documentos: tudo o que estiver nesses discos será apagado para sempre.")
			g.drawChoose("Atenção: este computador vai ser apagado", ToneDanger, lines, []string{"Entendi, continuar", "Desligar"}, 1)
		}},
		{"05-apagar", func(g *GUI) {
			g.stage = StageConfirm
			g.drawInput("Confirme que pode apagar", append([]string{"Último aviso. Discos que serão apagados:", ""}, disks...),
				"Para confirmar, digite APAGAR:", "APAG", false, "")
		}},
		{"06-nome", func(g *GUI) {
			g.stage = StageName
			g.drawInput("Nome do servidor", []string{"É o nome que vai aparecer no Mac e no Windows."}, "Nome:", "Backup da Casa", false, "")
		}},
		{"07-senha", func(g *GUI) {
			g.stage = StageName
			g.drawInput("Senha do servidor", []string{
				"Macs e PCs vão usar esta senha para fazer backup.",
				"Ela também serve para administrar o servidor (usuário: admin).",
				"Mínimo de 6 caracteres.",
			}, "Senha:", "abc", true, "Não pode ficar em branco.")
		}},
		{"08-sem-rede", func(g *GUI) {
			g.stage = StageName
			g.drawChoose("Sem conexão de rede", ToneWarn, []string{
				"Conecte este computador ao roteador com um cabo de rede.",
				"", "O servidor precisa estar na mesma rede dos Macs e PCs. (Wi-Fi chega numa próxima versão.)",
			}, []string{"Tentar de novo", "Continuar sem rede"}, 0)
		}},
		{"09-instalando", func(g *GUI) {
			g.stage = StageInstall
			g.drawProgress("Instalando o Backeddy Server", Steps, 2, "Não desligue o computador.", 3)
		}},
		{"10-pronto", func(g *GUI) {
			g.stage = StageDone
			g.drawChoose("Pronto!", ToneOK, DoneLines, []string{"Reiniciar"}, 0)
		}},
	}
	for _, size := range [][2]int{{1280, 800}, {1024, 768}, {800, 600}, {640, 480}} {
		g := newGUI(size[0], size[1])
		for _, s := range list {
			s.draw(g)
			if s.name == "02a-modos" {
				g.drawCursor(int(g.X(winW-200)), int(g.Y(250)))
			}
			path := filepath.Join(dir, fmt.Sprintf("%dx%d-%s.png", size[0], size[1], s.name))
			f, err := os.Create(path)
			if err != nil {
				return err
			}
			if err := png.Encode(f, g.snapshot()); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}
	return nil
}
