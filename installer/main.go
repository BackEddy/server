package main

// Backeddy Server — instalador do pendrive.
// "Espeta, confirma, pronto": apaga o PC (depois de avisar), instala o sistema e
// deixa o primeiro boot configurar os discos de backup. Liberdade total sobre os discos,
// com os riscos de cada escolha avisados.

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var version = "dev"

// errOff: a pessoa escolheu desligar.
var errOff = errors.New("desligar")

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

	// Mantém o programa inteiro na memória: o pendrive pode ser retirado no fim sem derrubar o instalador.
	_ = syscall.Mlockall(syscall.MCL_CURRENT)

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

	if err := run(ui, *dry, *fake, *planFile); err != nil {
		powerOff(*dry, ui)
	}
}

func run(ui Frontend, dry bool, fake, planFile string) error {
	plan, perr := LoadPlan(planFile)

	// 1. Boas-vindas
	ui.Stage(StageIntro)
	if ui.Choose("Transforme este computador em um lugar seguro para os backups da sua casa", ToneNormal,
		[]string{
			"Macs (Time Machine) e PCs com Windows vão poder fazer backup aqui, automaticamente.",
			"",
			"A instalação leva uns 5 minutos. Versão " + version + ".",
		}, []string{"Começar", "Desligar"}, 0, nil) != 0 {
		return errOff
	}

	// 2. Discos
	ui.Stage(StageDisks)
	ui.Notice("Procurando os discos…", ToneNormal, []string{"Conferindo também a saúde de cada disco."})
	var disks []Disk
	var err error
	if fake != "" {
		var data []byte
		if data, err = os.ReadFile(fake); err == nil {
			disks, err = parseLsblk(data)
		}
	} else {
		disks, err = ScanDisks()
		CheckHealth(disks)
	}
	if err == nil && len(usable(disks)) == 0 {
		err = errors.New("nenhum disco encontrado")
	}
	if err != nil {
		ui.Choose("Não encontrei discos para instalar", ToneDanger, []string{
			err.Error() + ".",
			"",
			"Se o computador tem discos, confira na BIOS/UEFI se o modo RAID ou \"Intel RST\" está ligado: troque para AHCI e comece de novo.",
		}, []string{"Desligar"}, 0, nil)
		return errOff
	}

	in := &Installer{Plan: plan, DryRun: dry, TMPercent: plan.TMPercent()}

	// 3. Backups de uma instalação anterior: recuperar ou começar do zero
	var layout *Layout
	recover := false
	for layout == nil {
		recover = false
		if HasBackeddy(disks) {
			c, b := ui.Cards("Encontrei backups do Backeddy", []string{
				"Este computador já foi um Backeddy Server. Dá para reinstalar só o sistema e manter todos os backups: os Macs e PCs continuam de onde pararam.",
			}, []Card{
				{Title: "Recuperar o servidor", Desc: "Reinstala o sistema e mantém os backups e as configurações.", Risk: RiskGreen, Recommended: true},
				{Title: "Apagar tudo e começar do zero", Desc: "Os backups antigos serão apagados para sempre.", Risk: RiskRed},
			}, 0, []string{"Continuar", "Desligar"})
			if b != 0 {
				return errOff
			}
			recover = c == 0
		}
		if recover {
			l, err := RecoverLayout(disks)
			if err != nil {
				ui.Choose("Não dá para recuperar ainda", ToneWarn, []string{err.Error() + "."}, []string{"Desligar"}, 0, nil)
				return errOff
			}
			lines := append([]string{}, l.Summary...)
			lines = append(lines, "", "Nome, senhas dos PCs e certificado voltam como eram (ficam guardados junto com os backups).")
			if ui.Choose("Recuperar o servidor", ToneNormal, lines, []string{"Recuperar", "Voltar"}, 0, nil) != 0 {
				continue
			}
			if len(l.Reds) > 0 && !redBarrier(ui, l.Reds) {
				continue
			}
			layout = l
			in.Recover = true
		} else {
			if layout, err = chooseLayout(ui, disks); err != nil {
				return err
			}
			if !checkPlan(ui, plan, perr, layout) {
				return errOff
			}
		}
	}

	// 4. Avisos de apagar
	ui.Stage(StageConfirm)
	if !recover || !layout.ReuseSystem {
		if !confirmErase(ui, layout) {
			return errOff
		}
	}

	// 5. Nome, uso e senha
	ui.Stage(StageName)
	if !recover {
		name := "Backup da Casa"
		if plan != nil && strings.TrimSpace(plan.Name) != "" {
			name = strings.TrimSpace(plan.Name)
		}
		in.Name = ui.Input("Nome do servidor", []string{"É o nome que vai aparecer no Mac e no Windows."}, "Nome:", name, false)
	}
	in.Password = askPassword(ui, recover)

	// 6. Rede (só cabo nesta versão)
	for !dry && !hasNetwork() {
		if ui.Choose("Sem conexão de rede", ToneWarn, []string{
			"Conecte este computador ao roteador com um cabo de rede.",
			"",
			"O servidor precisa estar na mesma rede dos Macs e PCs. (Wi-Fi chega numa próxima versão.)",
		}, []string{"Tentar de novo", "Continuar sem rede"}, 0, nil) == 1 {
			break
		}
	}

	// 6b. Alarme: teste de bipe e silêncio à noite
	in.BeepOK, in.QuietStart, in.QuietEnd = askAlarm(ui, dry)

	// 7. Instalação
	ui.Stage(StageInstall)
	in.Layout = layout
	for {
		err = in.Run(func(step int) {
			ui.Progress("Instalando o Backeddy Server", Steps, step, "Não desligue o computador.")
		})
		if err == nil {
			break
		}
		if ui.Choose("Algo deu errado", ToneDanger, []string{err.Error(), "", "Detalhes em " + logPath + "."},
			[]string{"Tentar de novo", "Desligar"}, 0, nil) == 1 {
			return errOff
		}
	}
	ui.Progress("Instalando o Backeddy Server", Steps, len(Steps), "")

	// 8. Pronto: Enter reinicia; tirar o pendrive também
	ui.Stage(StageDone)
	var removed <-chan struct{}
	if !dry {
		removed = watchInstallerRemoval()
	}
	lines := DoneLines
	if recover {
		lines = append([]string{}, DoneLines...)
		lines[len(lines)-1] = "Depois disso, os Macs e PCs voltam a fazer backup sozinhos. Se você trocou a senha, cada Mac vai pedir a nova."
	}
	r := ui.Choose("Pronto!", ToneOK, lines, []string{"Reiniciar"}, 0, removed)
	if dry {
		ui.Close()
		fmt.Println("dry-run: comandos registrados em", logPath)
		return nil
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
	return nil
}

// chooseLayout: tela de modos (cartões) + Manual + avisos de risco. Volta aqui até a pessoa decidir.
func chooseLayout(ui Frontend, disks []Disk) (*Layout, error) {
	opts, notes, err := Options(disks)
	sel := 0
	for {
		var layout *Layout
		manual := err != nil
		if !manual {
			cards := make([]Card, len(opts))
			for i, o := range opts {
				cards[i] = Card{Title: o.Title, Value: HumanSize(o.Usable), Desc: o.Desc, Risk: o.Risk, Recommended: i == 0}
			}
			lines := Explain(opts[0])
			for _, n := range notes {
				lines = append(lines, "", n)
			}
			c, b := ui.Cards("Como usar os discos", lines, cards, sel, []string{"Continuar", "Escolher os discos à mão", "Desligar"})
			sel = c
			switch b {
			case 2:
				return nil, errOff
			case 1:
				manual = true
			default:
				layout = opts[c]
			}
		} else if ui.Choose("Escolha os discos à mão", ToneWarn, []string{
			"Não encontrei um arranjo automático: " + err.Error() + ".",
			"Dá para escolher o papel de cada disco à mão.",
		}, []string{"Escolher os discos", "Desligar"}, 0, nil) != 0 {
			return nil, errOff
		}

		if manual {
			ds := usable(disks)
			sortDisks(ds)
			var rec *Layout
			if len(opts) > 0 {
				rec = opts[0]
			}
			roles, mirror, ok := ui.Manual("Escolher os discos à mão", []string{
				"Para cada disco: Sistema, Backup, Sistema + backup ou Não usar (fica intocado).",
			}, ds, DefaultRoles(ds, rec), rec == nil || rec.Mirror, func(r []Role, m bool) ([]string, bool) {
				l, e := ManualLayout(ds, r, m)
				if e != nil {
					return []string{"!" + strings.ToUpper(e.Error()[:1]) + e.Error()[1:] + "."}, false
				}
				out := append([]string{}, l.Summary...)
				if m && !l.Mirror {
					out = append(out, "!Espelho precisa de 2 lugares para backup: ficou sem.")
				}
				return out, true
			})
			if !ok {
				if err != nil {
					return nil, errOff
				}
				continue
			}
			layout, _ = ManualLayout(ds, roles, mirror)
		}
		if acceptRisk(ui, layout) {
			return layout, nil
		}
	}
}

// acceptRisk: amarelo = uma tela de aviso; vermelho = aviso + digitar CONTINUAR.
func acceptRisk(ui Frontend, l *Layout) bool {
	if l.Risk == RiskRed {
		return redBarrier(ui, append(append([]string{}, l.Reds...), l.Summary...))
	}
	if l.Risk == RiskYellow {
		lines := append([]string{}, l.Summary...)
		lines = append(lines, "",
			"Se um disco morrer, você perde o histórico de backups. Os arquivos nos Macs e PCs continuam, e o servidor recomeça do zero.")
		if l.Mode != ModeOneDisk {
			lines = append(lines, "", "Dá para ligar o espelho depois, sem reinstalar, se houver espaço livre: sudo backeddy mirror")
		} else {
			lines = append(lines, "", "Um segundo disco pode entrar depois e virar espelho, sem reinstalar: sudo backeddy add-disk")
		}
		return ui.Choose("Sem proteção contra disco quebrado", ToneWarn, lines, []string{"Entendi, continuar", "Voltar"}, 0, nil) == 0
	}
	return true
}

func redBarrier(ui Frontend, lines []string) bool {
	if ui.Choose("Isso não é recomendado", ToneDanger, lines, []string{"Voltar", "Continuar mesmo assim"}, 0, nil) != 1 {
		return false
	}
	return typed(ui, "Tem certeza?", []string{"Para seguir mesmo assim, digite CONTINUAR. Para voltar, digite SAIR."}, "CONTINUAR")
}

// typed pede uma palavra; SAIR desiste.
func typed(ui Frontend, title string, lines []string, word string) bool {
	for {
		v := ui.Input(title, lines, "Para confirmar, digite "+word+":", "", false)
		if v == word {
			return true
		}
		if strings.EqualFold(v, "SAIR") {
			return false
		}
	}
}

func checkPlan(ui Frontend, plan *Plan, perr error, l *Layout) bool {
	if perr != nil {
		if ui.Choose("Não consegui ler o plano do pendrive", ToneWarn, []string{perr.Error(), "", "Dá para seguir sem ele."},
			[]string{"Continuar", "Desligar"}, 0, nil) != 0 {
			return false
		}
	}
	switch plan.Check(l.Usable) {
	case FitYellow:
		return ui.Choose("Cabe, mas com pouco histórico", ToneWarn, []string{
			fmt.Sprintf("Espaço para backups: %s. Recomendado no plano: %s.", HumanSize(l.Usable), HumanSize(plan.RecommendedBytes)),
			"",
			"Os backups vão funcionar, mas o servidor vai guardar menos tempo de histórico.",
			"Para guardar mais, use um disco de " + HumanSize(plan.RecommendedBytes) + " ou maior, ou o modo Mais espaço.",
		}, []string{"Continuar", "Desligar"}, 0, nil) == 0
	case FitRed:
		if ui.Choose("O espaço não é suficiente", ToneDanger, []string{
			fmt.Sprintf("Quando você criou este pendrive, o recomendado era pelo menos %s.", HumanSize(plan.MinBytes)),
			fmt.Sprintf("Este arranjo só tem %s para backups.", HumanSize(l.Usable)),
			"",
			"Sugestão: instale um disco de " + HumanSize(plan.RecommendedBytes) + " ou maior e comece de novo.",
		}, []string{"Desligar", "Continuar mesmo assim"}, 0, nil) != 1 {
			return false
		}
		return typed(ui, "Tem certeza?", []string{
			fmt.Sprintf("Com %s, %s não vão caber juntos.", HumanSize(l.Usable), plan.ComputerNames()),
			"Quando o disco encher, algum computador vai parar de fazer backup.",
			"",
			"Para seguir mesmo assim, digite CONTINUAR. Para desistir, digite SAIR.",
		}, "CONTINUAR")
	}
	return true
}

func confirmErase(ui Frontend, l *Layout) bool {
	var dl []string
	for _, d := range l.AllDisks() {
		dl = append(dl, "• "+d.Label()+d.EraseNote())
	}
	warn := append([]string{"Estes discos vão ser apagados por completo:", ""}, dl...)
	warn = append(warn, "", "Windows, fotos, documentos: tudo o que estiver nesses discos será apagado para sempre.")
	for _, w := range l.Warns {
		warn = append(warn, "!"+w)
	}
	if ui.Choose("Atenção: estes discos vão ser apagados", ToneDanger, warn, []string{"Entendi, continuar", "Desligar"}, 1, nil) != 0 {
		return false
	}
	return typed(ui, "Confirme que pode apagar", append([]string{"Último aviso. Discos que serão apagados:", ""}, dl...), "APAGAR")
}

// askAlarm: o servidor bipa quando algo dá errado. Testa se o PC tem alto-falante e pergunta o silêncio à noite.
func askAlarm(ui Frontend, dry bool) (bool, int, int) {
	if InVM {
		// máquina virtual não tem alto-falante: quem bipa é o Windows (vigia na bandeja)
		return askQuiet(ui, "O Windows vai bipar se algo der errado no servidor. Para um alerta de madrugada não acordar a casa:")
	}
	beepOK := false
	for {
		if !dry {
			beepPattern()
		}
		c := ui.Choose("Teste do alarme", ToneNormal, []string{
			"Quando algo der errado (um disco falhando, por exemplo), o servidor bipa a cada minuto até alguém ver o aviso.",
			"",
			"*Agora ele tocou 3 bipes. Você ouviu?",
		}, []string{"Ouvi", "Tocar de novo", "Não ouvi"}, 0, nil)
		if c == 1 {
			continue
		}
		beepOK = c == 0
		break
	}
	if !beepOK {
		ui.Choose("Sem alto-falante", ToneWarn, []string{
			"Este computador não tem alto-falante interno (é comum em PCs novos).",
			"",
			"Os alertas vão aparecer na tela do servidor, no painel (http://nome-do-servidor.local) e nos apps do Backeddy.",
		}, []string{"Entendi"}, 0, nil)
		return false, 22, 8
	}
	return askQuiet(ui, "Para um alerta de madrugada não acordar a casa. Dá para mudar depois no painel.")
}

func askQuiet(ui Frontend, intro string) (bool, int, int) {
	c, _ := ui.Cards("Silêncio à noite", []string{intro}, []Card{
		{Title: "Sem bipe das 22h às 8h", Desc: "Durante a noite o aviso espera; o bipe volta de manhã.", Plain: true, Recommended: true},
		{Title: "Sem bipe das 23h às 7h", Desc: "Uma noite um pouco mais curta.", Plain: true},
		{Title: "Escolher o horário", Desc: "Você digita a hora de começar e de terminar o silêncio.", Plain: true},
		{Title: "Bipar a qualquer hora", Desc: "Para quem prefere saber na hora, mesmo de madrugada.", Plain: true},
	}, 0, []string{"Continuar"})
	switch c {
	case 1:
		return true, 23, 7
	case 2:
		s := askHour(ui, "Começo do silêncio", "A partir de que hora o servidor fica quieto? (0 a 23)", "22")
		e := askHour(ui, "Fim do silêncio", "A que horas o bipe pode voltar? (0 a 23)", "8")
		if s == e {
			return true, -1, -1
		}
		return true, s, e
	case 3:
		return true, -1, -1
	}
	return true, 22, 8
}

// askHour pede uma hora cheia de 0 a 23.
func askHour(ui Frontend, title, text, def string) int {
	lines := []string{text}
	for {
		v := strings.TrimSpace(strings.TrimSuffix(strings.ToLower(ui.Input(title, lines, "Hora:", def, false)), "h"))
		if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 23 {
			return n
		}
		lines = []string{text, "", "!Digite só o número da hora, de 0 a 23 (ex.: 22)."}
	}
}

func askPassword(ui Frontend, recover bool) string {
	intro := []string{
		"Macs e PCs vão usar esta senha para fazer backup.",
		"Ela também serve para administrar o servidor (usuário: admin).",
		"Mínimo de 6 caracteres.",
	}
	if recover {
		intro = []string{
			"Use a mesma senha de antes: assim os Macs continuam sem pedir nada.",
			"Se usar uma nova, cada Mac vai pedir a senha nova uma vez.",
			"Mínimo de 6 caracteres.",
		}
	}
	for {
		p1 := ui.Input("Senha do servidor", intro, "Senha:", "", true)
		if len([]rune(p1)) < 6 {
			continue
		}
		p2 := ui.Input("Senha do servidor", []string{"Digite a senha de novo, para conferir."}, "Senha:", "", true)
		if p1 == p2 {
			return p1
		}
		ui.Choose("As senhas não conferem", ToneWarn, []string{"Vamos tentar de novo."}, []string{"OK"}, 0, nil)
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
