package main

// Backeddy Server — instalador do pendrive.
// "Espeta, confirma, pronto": apaga o PC (depois de avisar), instala o sistema e
// deixa o primeiro boot configurar os discos de backup.

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var version = "dev"

func main() {
	dry := flag.Bool("dry-run", false, "mostra o que seria feito, sem apagar nada")
	fake := flag.String("fake-lsblk", "", "arquivo JSON do lsblk (testes)")
	planFile := flag.String("plan", planPath, "arquivo de plano")
	flag.Parse()

	ui := NewUI()
	defer ui.Close()

	plan, perr := LoadPlan(*planFile)

	// 1. Boas-vindas
	if ui.Choose("Transforme este computador em um lugar seguro para os backups da sua casa", bold,
		[]string{
			"Macs (Time Machine) e PCs com Windows vão poder fazer backup aqui, automaticamente.",
			"",
			"A instalação leva uns 5 minutos. Versão " + version + ".",
		}, []string{"Começar", "Desligar"}, 0) == 1 {
		powerOff(*dry, ui)
		return
	}

	// 2. Discos
	var disks []Disk
	var err error
	if *fake != "" {
		var data []byte
		if data, err = os.ReadFile(*fake); err == nil {
			disks, err = parseLsblk(data)
		}
	} else {
		disks, err = ScanDisks()
	}
	var layout *Layout
	if err == nil {
		layout, err = ChooseLayout(disks)
	}
	if err != nil {
		ui.Choose("Não dá para instalar neste computador", red,
			[]string{err.Error(), "", "O Backeddy Server precisa de um disco de pelo menos 40 GB."},
			[]string{"Desligar"}, 0)
		powerOff(*dry, ui)
		return
	}

	lines := append([]string{}, layout.Summary...)
	if perr != nil {
		lines = append(lines, "", "!Não consegui ler o plano do pendrive ("+perr.Error()+"). Seguindo sem ele.")
	}
	for _, w := range layout.Warns {
		lines = append(lines, "", "!"+w)
	}
	fit := plan.Check(layout.Usable)
	if fit == FitGreen {
		lines = append(lines, "", green+"Cabe com folga: o recomendado no plano é "+HumanSize(plan.RecommendedBytes)+"."+reset)
	}
	if ui.Choose("Encontrei estes discos", bold, lines, []string{"Continuar", "Desligar"}, 0) == 1 {
		powerOff(*dry, ui)
		return
	}

	// 3. Comparação com o plano
	switch fit {
	case FitYellow:
		if ui.Choose("Cabe, mas com pouco histórico", yellow, []string{
			fmt.Sprintf("Espaço para backups: %s. Recomendado no plano: %s.", HumanSize(layout.Usable), HumanSize(plan.RecommendedBytes)),
			"",
			"Os backups vão funcionar, mas o servidor vai guardar menos tempo de histórico.",
			"Para guardar mais, use um disco de " + HumanSize(plan.RecommendedBytes) + " ou maior.",
		}, []string{"Continuar", "Desligar"}, 0) == 1 {
			powerOff(*dry, ui)
			return
		}
	case FitRed:
		if ui.Choose("O espaço não é suficiente", red, []string{
			fmt.Sprintf("Quando você criou este pendrive, o recomendado era pelo menos %s.", HumanSize(plan.MinBytes)),
			fmt.Sprintf("Este computador só tem %s para backups.", HumanSize(layout.Usable)),
			"",
			"Sugestão: instale um disco de " + HumanSize(plan.RecommendedBytes) + " ou maior e comece de novo.",
		}, []string{"Desligar", "Continuar mesmo assim"}, 0) != 1 {
			powerOff(*dry, ui)
			return
		}
		for {
			v := ui.Input("Tem certeza?", []string{
				fmt.Sprintf("Com %s, %s não vão caber juntos.", HumanSize(layout.Usable), plan.ComputerNames()),
				"Quando o disco encher, algum computador vai parar de fazer backup.",
				"",
				"Para seguir mesmo assim, digite CONTINUAR. Para desistir, digite SAIR.",
			}, "Digite:", "", false)
			if v == "CONTINUAR" {
				break
			}
			if strings.EqualFold(v, "SAIR") {
				powerOff(*dry, ui)
				return
			}
		}
	}

	// 4. Avisos de apagar
	var dl []string
	for _, d := range layout.AllDisks() {
		dl = append(dl, "  • "+d.Label())
	}
	warn := append([]string{"Este computador vai ser apagado por completo:", ""}, dl...)
	warn = append(warn, "", "Windows, fotos, documentos: tudo o que estiver nesses discos será apagado para sempre.")
	if ui.Choose("Atenção", red, warn, []string{"Entendi, continuar", "Desligar"}, 1) == 1 {
		powerOff(*dry, ui)
		return
	}
	for {
		v := ui.Input("Confirme que pode apagar", append([]string{"Último aviso. Discos que serão apagados:", ""}, dl...),
			"Para confirmar, digite APAGAR:", "", false)
		if v == "APAGAR" {
			break
		}
		if strings.EqualFold(v, "SAIR") {
			powerOff(*dry, ui)
			return
		}
	}

	// 5. Nome e senha
	name := "Backup da Casa"
	if plan != nil && strings.TrimSpace(plan.Name) != "" {
		name = strings.TrimSpace(plan.Name)
	}
	name = ui.Input("Nome do servidor", []string{"É o nome que vai aparecer no Mac e no Windows."}, "Nome:", name, false)
	var pass string
	for {
		p1 := ui.Input("Senha do servidor", []string{
			"Macs e PCs vão usar esta senha para fazer backup.",
			"Ela também serve para administrar o servidor (usuário: admin).",
			"Mínimo de 6 caracteres.",
		}, "Senha:", "", true)
		if len([]rune(p1)) < 6 {
			continue
		}
		p2 := ui.Input("Senha do servidor", []string{"Digite a senha de novo, para conferir."}, "Senha:", "", true)
		if p1 == p2 {
			pass = p1
			break
		}
		ui.Choose("As senhas não conferem", yellow, []string{"Vamos tentar de novo."}, []string{"OK"}, 0)
	}

	// 6. Rede (só cabo nesta versão)
	for !*dry && !hasNetwork() {
		if ui.Choose("Sem conexão de rede", yellow, []string{
			"Conecte este computador ao roteador com um cabo de rede.",
			"",
			"O servidor precisa estar na mesma rede dos Macs e PCs. (Wi-Fi chega numa próxima versão.)",
		}, []string{"Tentar de novo", "Continuar sem rede"}, 0) == 1 {
			break
		}
	}

	// 7. Instalação
	in := &Installer{Layout: layout, Name: name, Password: pass, Plan: plan, DryRun: *dry}
	for {
		err = in.Run(func(step int) { ui.Progress("Instalando", Steps, step, "Não desligue o computador.") })
		if err == nil {
			break
		}
		if ui.Choose("Algo deu errado", red, []string{err.Error(), "", "Detalhes em " + logPath + "."},
			[]string{"Tentar de novo", "Desligar"}, 0) == 1 {
			powerOff(*dry, ui)
			return
		}
	}
	ui.Progress("Instalando", Steps, len(Steps), "")

	// 8. Pronto
	ui.Choose("Pronto!", green, []string{
		"Retire o pendrive e aperte Enter para reiniciar.",
		"",
		"No primeiro boot, o servidor prepara os discos de backup (alguns minutos).",
		"Quando terminar, a tela mostra como conectar o Mac e o Windows.",
	}, []string{"Reiniciar"}, 0)
	ui.Close()
	if *dry {
		fmt.Println("dry-run: comandos registrados em", logPath)
		return
	}
	_ = exec.Command("systemctl", "reboot").Run()
}

func hasNetwork() bool {
	out, err := exec.Command("ip", "-4", "route", "show", "default").Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

func powerOff(dry bool, ui *UI) {
	ui.Close()
	if dry {
		fmt.Println("dry-run: desligaria o computador")
		return
	}
	_ = exec.Command("systemctl", "poweroff").Run()
}
