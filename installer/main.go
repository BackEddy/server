package main

// Backeddy Server — instalador do pendrive.
// "Espeta, confirma, pronto": apaga o PC (depois de avisar), instala o sistema e
// deixa o primeiro boot configurar os discos de backup.

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

var version = "dev"

func main() {
	dry := flag.Bool("dry-run", false, "mostra o que seria feito, sem apagar nada")
	fake := flag.String("fake-lsblk", "", "arquivo JSON do lsblk (testes)")
	planFile := flag.String("plan", planPath, "arquivo de plano")
	textMode := flag.Bool("text", false, "força o modo texto")
	shots := flag.String("shots", "", "salva capturas das telas gráficas nesta pasta (desenvolvimento)")
	flag.Parse()

	if *shots != "" {
		if err := runShots(*shots); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	var ui Frontend
	if !*textMode {
		if g, err := NewGUI(); err == nil {
			ui = g
		} else {
			fmt.Fprintln(os.Stderr, "modo gráfico indisponível:", err)
		}
	}
	if ui == nil {
		ui = NewTextUI()
	}
	defer ui.Close()
	defer func() { // se algo explodir, devolve a tela ao modo texto antes de mostrar o erro
		if r := recover(); r != nil {
			ui.Close()
			panic(r)
		}
	}()

	plan, perr := LoadPlan(*planFile)

	// 1. Boas-vindas
	ui.Stage(StageIntro)
	if ui.Choose("Transforme este computador em um lugar seguro para os backups da sua casa", ToneNormal,
		[]string{
			"Macs (Time Machine) e PCs com Windows vão poder fazer backup aqui, automaticamente.",
			"",
			"A instalação leva uns 5 minutos. Versão " + version + ".",
		}, []string{"Começar", "Desligar"}, 0, nil) == 1 {
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
		ui.Choose("Não dá para instalar neste computador", ToneDanger,
			[]string{err.Error(), "", "O Backeddy Server precisa de um disco de pelo menos 40 GB."},
			[]string{"Desligar"}, 0, nil)
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
		lines = append(lines, "", "+Cabe com folga: o recomendado no plano é "+HumanSize(plan.RecommendedBytes)+".")
	}
	ui.Stage(StageDisks)
	if ui.Choose("Encontrei estes discos", ToneNormal, lines, []string{"Continuar", "Desligar"}, 0, nil) == 1 {
		powerOff(*dry, ui)
		return
	}

	// 3. Comparação com o plano
	switch fit {
	case FitYellow:
		if ui.Choose("Cabe, mas com pouco histórico", ToneWarn, []string{
			fmt.Sprintf("Espaço para backups: %s. Recomendado no plano: %s.", HumanSize(layout.Usable), HumanSize(plan.RecommendedBytes)),
			"",
			"Os backups vão funcionar, mas o servidor vai guardar menos tempo de histórico.",
			"Para guardar mais, use um disco de " + HumanSize(plan.RecommendedBytes) + " ou maior.",
		}, []string{"Continuar", "Desligar"}, 0, nil) == 1 {
			powerOff(*dry, ui)
			return
		}
	case FitRed:
		if ui.Choose("O espaço não é suficiente", ToneDanger, []string{
			fmt.Sprintf("Quando você criou este pendrive, o recomendado era pelo menos %s.", HumanSize(plan.MinBytes)),
			fmt.Sprintf("Este computador só tem %s para backups.", HumanSize(layout.Usable)),
			"",
			"Sugestão: instale um disco de " + HumanSize(plan.RecommendedBytes) + " ou maior e comece de novo.",
		}, []string{"Desligar", "Continuar mesmo assim"}, 0, nil) != 1 {
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
		dl = append(dl, "• "+d.Label())
	}
	warn := append([]string{"Estes discos vão ser apagados por completo:", ""}, dl...)
	warn = append(warn, "", "Windows, fotos, documentos: tudo o que estiver nesses discos será apagado para sempre.")
	ui.Stage(StageConfirm)
	if ui.Choose("Atenção: este computador vai ser apagado", ToneDanger, warn, []string{"Entendi, continuar", "Desligar"}, 1, nil) == 1 {
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
	ui.Stage(StageName)
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
		ui.Choose("As senhas não conferem", ToneWarn, []string{"Vamos tentar de novo."}, []string{"OK"}, 0, nil)
	}

	// 6. Rede (só cabo nesta versão)
	for !*dry && !hasNetwork() {
		if ui.Choose("Sem conexão de rede", ToneWarn, []string{
			"Conecte este computador ao roteador com um cabo de rede.",
			"",
			"O servidor precisa estar na mesma rede dos Macs e PCs. (Wi-Fi chega numa próxima versão.)",
		}, []string{"Tentar de novo", "Continuar sem rede"}, 0, nil) == 1 {
			break
		}
	}

	// 7. Instalação
	ui.Stage(StageInstall)
	in := &Installer{Layout: layout, Name: name, Password: pass, Plan: plan, DryRun: *dry}
	for {
		err = in.Run(func(step int) {
			ui.Progress("Instalando o Backeddy Server", Steps, step, "Não desligue o computador.")
		})
		if err == nil {
			break
		}
		if ui.Choose("Algo deu errado", ToneDanger, []string{err.Error(), "", "Detalhes em " + logPath + "."},
			[]string{"Tentar de novo", "Desligar"}, 0, nil) == 1 {
			powerOff(*dry, ui)
			return
		}
	}
	ui.Progress("Instalando o Backeddy Server", Steps, len(Steps), "")

	// 8. Pronto: Enter reinicia; tirar o pendrive também
	ui.Stage(StageDone)
	var removed <-chan struct{}
	if !*dry {
		removed = watchInstallerRemoval()
	}
	r := ui.Choose("Pronto!", ToneOK, DoneLines, []string{"Reiniciar"}, 0, removed)
	if *dry {
		ui.Close()
		fmt.Println("dry-run: comandos registrados em", logPath)
		return
	}
	if r == -1 {
		// o pendrive saiu: o sistema do instalador sumiu junto, então reinicia direto pelo kernel
		ui.Notice("Reiniciando…", ToneOK, []string{"Pendrive retirado. O computador vai reiniciar."})
		time.Sleep(2 * time.Second)
		syscall.Sync()
		_ = syscall.Reboot(syscall.LINUX_REBOOT_CMD_RESTART)
	}
	ui.Close()
	if exec.Command("systemctl", "reboot").Run() != nil {
		syscall.Sync()
		_ = syscall.Reboot(syscall.LINUX_REBOOT_CMD_RESTART)
	}
}

var DoneLines = []string{
	"*Retire o pendrive e aperte Enter para reiniciar.",
	"Se preferir, é só tirar o pendrive: o computador reinicia sozinho.",
	"",
	"!No primeiro início podem aparecer mensagens técnicas na tela. É normal: não desligue e espere aparecer \"servidor funcionando\" (uns 5 minutos).",
	"",
	"Depois disso, a tela do servidor mostra como conectar o Mac e o Windows e o endereço para acesso remoto (SSH).",
}

// watchInstallerRemoval avisa quando a partição do instalador (o pendrive) some do sistema.
func watchInstallerRemoval() <-chan struct{} {
	dev, err := filepath.EvalSymlinks("/dev/disk/by-partlabel/backeddy-inst")
	if err != nil {
		return nil
	}
	sys := "/sys/class/block/" + filepath.Base(dev)
	if _, err := os.Stat(sys); err != nil {
		return nil
	}
	ch := make(chan struct{})
	go func() {
		for {
			time.Sleep(500 * time.Millisecond)
			if _, err := os.Stat(sys); err != nil {
				close(ch)
				return
			}
		}
	}()
	return ch
}

func hasNetwork() bool {
	out, err := exec.Command("ip", "-4", "route", "show", "default").Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

func powerOff(dry bool, ui Frontend) {
	ui.Close()
	if dry {
		fmt.Println("dry-run: desligaria o computador")
		return
	}
	_ = exec.Command("systemctl", "poweroff").Run()
}
