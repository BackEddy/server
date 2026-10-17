package main

// Wi-Fi no instalador (0.7): só aparece quando não há cabo. Lista as redes (iw), conecta com a senha (iwd)
// e o perfil salvo em /var/lib/iwd vai junto para o servidor instalado.

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type WifiNet struct {
	SSID    string
	Signal  int // 0–100
	Secured bool
}

const iwdDir = "/var/lib/iwd"

// Trocáveis nos testes
var (
	wifiIface   = findWifiIface
	wifiScan    = scanWifi
	wifiConnect = connectWifi
	netReady    = func(dry bool) bool { return dry || hasNetwork() }
)

// findWifiIface devolve a primeira placa Wi-Fi que o Linux reconheceu ("" se nenhuma).
func findWifiIface() string {
	ms, _ := filepath.Glob("/sys/class/net/*/wireless")
	sort.Strings(ms)
	for _, m := range ms {
		return filepath.Base(filepath.Dir(m))
	}
	return ""
}

// dBm → 0–100 (−100 dBm = 0, −50 dBm ou mais = 100)
func signalPercent(dbm float64) int {
	p := int(2 * (dbm + 100))
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return p
}

// parseIwScan lê a saída de "iw dev X scan": uma rede por SSID (a de sinal mais forte), sem redes
// escondidas nem corporativas (802.1X), da mais forte para a mais fraca.
func parseIwScan(out string) []WifiNet {
	best := map[string]WifiNet{}
	var cur *WifiNet
	enterprise := false
	flush := func() {
		if cur != nil && cur.SSID != "" && !enterprise {
			if b, ok := best[cur.SSID]; !ok || cur.Signal > b.Signal {
				best[cur.SSID] = *cur
			}
		}
	}
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := sc.Text()
		t := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "BSS "):
			flush()
			cur, enterprise = &WifiNet{}, false
		case cur == nil:
		case strings.HasPrefix(t, "SSID: "):
			s := strings.TrimPrefix(t, "SSID: ")
			if !strings.Contains(s, `\x00`) {
				cur.SSID = s
			}
		case strings.HasPrefix(t, "signal: "):
			f := strings.Fields(strings.TrimPrefix(t, "signal: "))
			if len(f) > 0 {
				if v, err := strconv.ParseFloat(f[0], 64); err == nil {
					cur.Signal = signalPercent(v)
				}
			}
		case strings.HasPrefix(t, "RSN:"), strings.HasPrefix(t, "WPA:"), strings.HasPrefix(t, "capability:") && strings.Contains(t, "Privacy"):
			cur.Secured = true
		case strings.Contains(t, "Authentication suites:") && strings.Contains(t, "802.1X") && !strings.Contains(t, "PSK") && !strings.Contains(t, "SAE"):
			enterprise = true
		}
	}
	flush()
	nets := make([]WifiNet, 0, len(best))
	for _, n := range best {
		nets = append(nets, n)
	}
	sort.Slice(nets, func(i, j int) bool {
		if nets[i].Signal != nets[j].Signal {
			return nets[i].Signal > nets[j].Signal
		}
		return nets[i].SSID < nets[j].SSID
	})
	return nets
}

func scanWifi(iface string) ([]WifiNet, error) {
	_ = exec.Command("ip", "link", "set", iface, "up").Run()
	var lastErr error
	for try := 0; try < 4; try++ { // o iwd também faz varreduras: "Device or resource busy" é normal
		out, err := exec.Command("iw", "dev", iface, "scan").CombinedOutput()
		if err == nil {
			return parseIwScan(string(out)), nil
		}
		lastErr = fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
		time.Sleep(2 * time.Second)
	}
	return nil, lastErr
}

var errWifiAuth = errors.New("senha")

// connectWifi conecta pelo iwd e espera o endereço (DHCP) por até 30 s.
func connectWifi(iface, ssid, pass string) error {
	args := []string{}
	if pass != "" {
		args = append(args, "--passphrase", pass)
	}
	args = append(args, "station", iface, "connect", ssid)
	out, err := exec.Command("iwctl", args...).CombinedOutput()
	if err != nil {
		_ = exec.Command("iwctl", "known-networks", ssid, "forget").Run()
		msg := strings.ToLower(string(out))
		if strings.Contains(msg, "passphrase") || strings.Contains(msg, "operation failed") || strings.Contains(msg, "invalid") {
			return errWifiAuth
		}
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	for i := 0; i < 30; i++ {
		if hasNetwork() {
			return nil
		}
		time.Sleep(time.Second)
	}
	return errors.New("conectou no Wi-Fi, mas o roteador não deu endereço")
}

func bars(p int) string {
	switch {
	case p >= 70:
		return "Sinal forte"
	case p >= 45:
		return "Sinal bom"
	case p >= 25:
		return "Sinal fraco"
	}
	return "Sinal muito fraco"
}

// askNetwork: sem cabo, oferece o Wi-Fi (se houver placa); devolve o nome da rede Wi-Fi usada ("" = cabo ou sem rede).
func askNetwork(ui Frontend, dry bool) string {
	for !netReady(dry) {
		iface := wifiIface()
		if iface == "" {
			if ui.Choose("Sem conexão de rede", ToneWarn, []string{
				"Conecte este computador ao roteador com um cabo de rede.",
				"",
				"Não achei placa de Wi-Fi que funcione neste computador.",
				"O servidor precisa estar na mesma rede dos Macs e PCs.",
			}, []string{"Tentar de novo", "Continuar sem rede"}, 0, nil) == 1 {
				return ""
			}
			continue
		}
		ssid, ok := askWifi(ui, iface)
		if ok {
			return ssid
		}
		if ssid == "-" { // escolheu continuar sem rede
			return ""
		}
	}
	return ""
}

// askWifi devolve (rede, true) quando conectou; ("", false) para voltar a conferir o cabo;
// ("-", false) para seguir sem rede.
func askWifi(ui Frontend, iface string) (string, bool) {
	for {
		ui.Notice("Procurando redes Wi-Fi", ToneNormal, []string{"Sem cabo de rede: procurando as redes Wi-Fi por perto…"})
		nets, err := wifiScan(iface)
		if netReady(false) { // ligaram o cabo enquanto isso
			return "", false
		}
		if len(nets) > 4 {
			nets = nets[:4]
		}
		lines := []string{
			"Não encontrei cabo de rede. Escolha a sua rede Wi-Fi.",
			"Cabo é melhor: o primeiro backup pelo Wi-Fi é bem mais lento. Se ligar o cabo depois, o servidor passa a usá-lo sozinho.",
		}
		if err != nil || len(nets) == 0 {
			c := ui.Choose("Nenhuma rede Wi-Fi encontrada", ToneWarn, []string{
				"Não encontrei redes Wi-Fi. Confira se o roteador está ligado e perto,",
				"ou conecte um cabo de rede.",
			}, []string{"Procurar de novo", "Digitar o nome da rede", "Já liguei o cabo", "Continuar sem rede"}, 0, nil)
			switch c {
			case 1:
				if ssid, ok := tryConnect(ui, iface, "", true); ok {
					return ssid, true
				}
			case 2:
				return "", false
			case 3:
				return "-", false
			}
			continue
		}
		cards := make([]Card, len(nets))
		for i, n := range nets {
			lock := "Aberta (sem senha)"
			if n.Secured {
				lock = "Com senha"
			}
			cards[i] = Card{Title: n.SSID, Value: fmt.Sprintf("%d%%", n.Signal), Desc: bars(n.Signal) + " · " + lock, Plain: true}
		}
		card, btn := ui.Cards("Rede Wi-Fi", lines, cards, 0, []string{"Conectar", "Outra rede", "Procurar de novo", "Já liguei o cabo"})
		switch btn {
		case 0:
			n := nets[card]
			if ssid, ok := tryConnect(ui, iface, n.SSID, n.Secured); ok {
				return ssid, true
			}
		case 1:
			if ssid, ok := tryConnect(ui, iface, "", true); ok {
				return ssid, true
			}
		case 3:
			return "", false
		}
	}
}

// tryConnect pede a senha (e o nome, se ssid == "") e conecta. false = voltar para a lista.
func tryConnect(ui Frontend, iface, ssid string, secured bool) (string, bool) {
	if ssid == "" {
		ssid = strings.TrimSpace(ui.Input("Outra rede", []string{"Digite o nome da rede Wi-Fi exatamente como aparece no celular (maiúsculas contam)."}, "Nome da rede:", "", false))
		if ssid == "" {
			return "", false
		}
	}
	lines := []string{"Rede: *" + ssid}
	for {
		pass := ""
		if secured {
			pass = ui.Input("Senha do Wi-Fi", lines, "Senha:", "", true)
			if pass == "" {
				return "", false
			}
		}
		ui.Notice("Conectando", ToneNormal, []string{"Conectando em " + ssid + "…"})
		err := wifiConnect(iface, ssid, pass)
		if err == nil {
			ui.Choose("Conectado pelo Wi-Fi", ToneOK, []string{
				"+Conectado em " + ssid + ".",
				"",
				"O servidor vai usar esta rede sempre, e reconecta sozinho se ela cair.",
				"!O primeiro backup pelo Wi-Fi é bem mais lento que pelo cabo. Se puder, ligue um cabo depois: o servidor passa a usá-lo sozinho.",
				"",
				"Para trocar de rede no futuro: com teclado e monitor no servidor, entre como admin e rode sudo backeddy wifi.",
			}, []string{"Continuar"}, 0, nil)
			return ssid, true
		}
		if !secured {
			ui.Choose("Não conectou", ToneWarn, []string{"Não consegui conectar em " + ssid + ".", "", err.Error()}, []string{"Voltar"}, 0, nil)
			return "", false
		}
		if errors.Is(err, errWifiAuth) {
			lines = []string{"Rede: *" + ssid, "", "!Senha errada (ou o nome da rede não confere). Tente de novo."}
		} else {
			lines = []string{"Rede: *" + ssid, "", "!Não conectou: " + err.Error()}
		}
	}
}

// copyWifiProfiles leva as redes Wi-Fi conhecidas para o servidor instalado.
func (in *Installer) copyWifiProfiles() error {
	if in.Wifi == "" {
		return nil
	}
	if in.DryRun {
		in.logf("$ cp %s/* %s%s/", iwdDir, target, iwdDir)
		return nil
	}
	if err := os.MkdirAll(target+iwdDir, 0700); err != nil {
		return err
	}
	files, _ := filepath.Glob(iwdDir + "/*")
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if err := os.WriteFile(filepath.Join(target+iwdDir, filepath.Base(f)), b, 0600); err != nil {
			return err
		}
	}
	in.logf("Wi-Fi: %d perfil(is) copiado(s) (rede %q)", len(files), in.Wifi)
	return nil
}
