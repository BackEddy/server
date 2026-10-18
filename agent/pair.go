package main

// Pareamento dos PCs (Backeddy for Windows): o app acha o servidor na rede, a pessoa digita a senha do
// servidor e o próprio servidor cria o acesso daquele PC no Kopia (o mesmo que "backeddy add-pc").
// A resposta traz o endereço, o usuário, a senha do PC e a impressão digital do certificado: o app só
// aceita um servidor com esse certificado.

import (
	"encoding/json"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"unicode"
)

// pcName: o nome que o Windows dá ao PC vira um nome seguro (letras, números e hífen), como o add-pc faz.
func pcName(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			b.WriteRune(r)
			dash = false
		} else if b.Len() > 0 && !dash {
			b.WriteByte('-')
			dash = true
		}
	}
	out := strings.TrimRight(b.String(), "-")
	if len(out) > 40 {
		out = strings.TrimRight(out[:40], "-")
	}
	return out
}

// localIP: o endereço do servidor que o PC usou para chegar aqui (serve quando o nome .local não resolve).
func localIP(r *http.Request) string {
	a, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
	if !ok {
		return ""
	}
	host, _, err := net.SplitHostPort(a.String())
	if err != nil {
		return ""
	}
	if ip := net.ParseIP(host); ip != nil && ip.To4() != nil && !ip.IsLoopback() {
		return ip.String()
	}
	return ""
}

func apiPair(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct{ Password, Name string }
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req)
	ip := clientIP(r)
	if tooMany(ip) {
		http.Error(w, "too many attempts; wait 10 minutes", http.StatusTooManyRequests)
		return
	}
	if !passwordOK(req.Password) {
		failed(ip)
		http.Error(w, "wrong password", http.StatusUnauthorized)
		return
	}
	name := pcName(req.Name)
	if name == "" {
		http.Error(w, "missing PC name", http.StatusBadRequest)
		return
	}
	out, err := exec.Command(backeddy, "add-pc", name, "--json").Output()
	var resp map[string]any
	if err != nil || json.Unmarshal(out, &resp) != nil {
		http.Error(w, "could not create the PC access", http.StatusInternalServerError)
		return
	}
	if lip := localIP(r); lip != "" {
		resp["ip"] = lip
		resp["urlIP"] = "https://" + lip + ":51515"
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// apiPCs: PCs pareados (nomes), para o painel e os apps mostrarem.
func apiPCs(w http.ResponseWriter, r *http.Request) {
	pcs := []string{}
	if out, err := exec.Command(backeddy, "pcs").Output(); err == nil {
		for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if l = strings.TrimSpace(l); l != "" {
				pcs = append(pcs, l)
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"pcs": pcs})
}
