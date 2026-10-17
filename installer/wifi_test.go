package main

import (
	"testing"
)

const iwSample = `BSS aa:bb:cc:dd:ee:01(on wlan0)
	freq: 2437
	capability: ESS Privacy ShortSlotTime (0x0411)
	signal: -48.00 dBm
	SSID: Casa
	RSN:	 * Version: 1
		 * Authentication suites: PSK
BSS aa:bb:cc:dd:ee:02(on wlan0)
	signal: -70.00 dBm
	SSID: Casa
	RSN:	 * Version: 1
		 * Authentication suites: PSK
BSS aa:bb:cc:dd:ee:03(on wlan0)
	capability: ESS (0x0401)
	signal: -80.00 dBm
	SSID: Padaria Free
BSS aa:bb:cc:dd:ee:04(on wlan0)
	capability: ESS Privacy (0x0411)
	signal: -60.00 dBm
	SSID: Empresa
	RSN:	 * Version: 1
		 * Authentication suites: IEEE 802.1X
BSS aa:bb:cc:dd:ee:05(on wlan0)
	signal: -40.00 dBm
	SSID: \x00\x00\x00
BSS aa:bb:cc:dd:ee:06(on wlan0)
	capability: ESS Privacy (0x0411)
	signal: -65.00 dBm
	SSID: Vizinho 5G
	RSN:	 * Version: 1
		 * Authentication suites: SAE
`

func TestParseIwScan(t *testing.T) {
	nets := parseIwScan(iwSample)
	if len(nets) != 3 {
		t.Fatalf("esperava 3 redes (Casa, Vizinho 5G, Padaria Free), veio %+v", nets)
	}
	if nets[0].SSID != "Casa" || nets[0].Signal != 100 || !nets[0].Secured {
		t.Fatalf("primeira deveria ser Casa 100%% com senha: %+v", nets[0])
	}
	if nets[1].SSID != "Vizinho 5G" || !nets[1].Secured || nets[1].Signal != 70 {
		t.Fatalf("segunda: %+v", nets[1])
	}
	if nets[2].SSID != "Padaria Free" || nets[2].Secured {
		t.Fatalf("terceira deveria ser aberta: %+v", nets[2])
	}
}

// Sem cabo: lista, senha errada, senha certa, aviso de Wi-Fi e segue o roteiro normal.
func TestFlowWifi(t *testing.T) {
	connected := false
	tries := 0
	oldReady, oldIface, oldScan, oldConn := netReady, wifiIface, wifiScan, wifiConnect
	defer func() { netReady, wifiIface, wifiScan, wifiConnect = oldReady, oldIface, oldScan, oldConn }()
	netReady = func(bool) bool { return connected }
	wifiIface = func() string { return "wlan0" }
	wifiScan = func(string) ([]WifiNet, error) { return parseIwScan(iwSample), nil }
	wifiConnect = func(_, ssid, pass string) error {
		tries++
		if ssid != "Casa" {
			t.Fatalf("conectou na rede errada: %q", ssid)
		}
		if pass != "certa123" {
			return errWifiAuth
		}
		connected = true
		return nil
	}
	f := runFlow(t, hyperV, []answer{{title: "Transforme"},
		{title: "Como usar os discos", card: 0, btn: 0},
		{title: "Atenção"}, {title: "Confirme", input: "APAGAR"},
		{title: "Nome"},
		{title: "Senha", input: "segredo1"}, {title: "Senha", input: "segredo1"},
		{title: "Rede Wi-Fi", card: 0, btn: 0},
		{title: "Senha do Wi-Fi", input: "errada"},
		{title: "Senha do Wi-Fi", input: "certa123"},
		{title: "Conectado pelo Wi-Fi"},
		{title: "Teste do alarme", choose: 0}, {title: "Silêncio", card: 0},
		{title: "Pronto"},
	})
	if tries != 2 {
		t.Fatalf("esperava 2 tentativas de conexão, foram %d (%v)", tries, f.seen)
	}
}

// Sem cabo e sem placa de Wi-Fi: pede o cabo; "Continuar sem rede" segue.
func TestFlowNoWifiCard(t *testing.T) {
	oldReady, oldIface := netReady, wifiIface
	defer func() { netReady, wifiIface = oldReady, oldIface }()
	netReady = func(bool) bool { return false }
	wifiIface = func() string { return "" }
	runFlow(t, hyperV, []answer{{title: "Transforme"},
		{title: "Como usar os discos", card: 0, btn: 0},
		{title: "Atenção"}, {title: "Confirme", input: "APAGAR"},
		{title: "Nome"},
		{title: "Senha", input: "segredo1"}, {title: "Senha", input: "segredo1"},
		{title: "Sem conexão de rede", choose: 1},
		{title: "Teste do alarme", choose: 0}, {title: "Silêncio", card: 0},
		{title: "Pronto"},
	})
}
