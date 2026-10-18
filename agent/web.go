package main

// Interface web e API do servidor (rede local, porta 80), protegidas pela senha do servidor.
// O Backeddy for Mac/Windows usam a mesma API (Authorization: Basic backeddy:SENHA).

import (
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed index.html
var indexHTML []byte

var (
	sessMu   sync.Mutex
	sessions = map[string]time.Time{}
	failMu   sync.Mutex
	fails    = map[string][]time.Time{}
)

func serveWeb(addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(indexHTML)
	})
	mux.HandleFunc("/api/login", apiLogin)
	mux.HandleFunc("/api/beep", apiBeep)   // sem senha: só diz se é hora de bipar (vigia do Windows)
	mux.HandleFunc("/api/hello", apiHello) // sem senha: nome do servidor (o mesmo que o Bonjour já anuncia)
	mux.HandleFunc("/api/status", auth(apiStatus))
	mux.HandleFunc("/api/ack", auth(apiAck))
	mux.HandleFunc("/api/quiet", auth(apiQuiet))
	mux.HandleFunc("/api/pair", apiPair) // a senha vai no corpo (Backeddy for Windows)
	mux.HandleFunc("/api/pcs", auth(apiPCs))
	mux.HandleFunc("/api/update", auth(apiUpdate))
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Printf("interface web em %s", addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Printf("interface web: %v", err)
	}
}

// passwordOK: checkPassword, trocável nos testes
var passwordOK = checkPassword

// checkPassword compara com o hash NT do usuário backeddy no Samba (a senha do servidor).
func checkPassword(pw string) bool {
	out, err := exec.Command("pdbedit", "-L", "-w", "-u", "backeddy").Output()
	if err != nil {
		return false
	}
	f := strings.Split(strings.TrimSpace(string(out)), ":")
	if len(f) < 4 {
		return false
	}
	want, err := hex.DecodeString(f[3])
	if err != nil || len(want) != 16 {
		return false
	}
	got := ntHash(pw)
	return subtle.ConstantTimeCompare(got[:], want) == 1
}

// tooMany: no máximo 10 senhas erradas em 10 minutos por endereço.
func tooMany(ip string) bool {
	failMu.Lock()
	defer failMu.Unlock()
	var keep []time.Time
	for _, t := range fails[ip] {
		if time.Since(t) < 10*time.Minute {
			keep = append(keep, t)
		}
	}
	fails[ip] = keep
	return len(keep) >= 10
}

func failed(ip string) {
	failMu.Lock()
	fails[ip] = append(fails[ip], time.Now())
	failMu.Unlock()
}

func clientIP(r *http.Request) string {
	ip := r.RemoteAddr
	if i := strings.LastIndex(ip, ":"); i > 0 {
		ip = ip[:i]
	}
	return ip
}

func auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("backeddy"); err == nil {
			sessMu.Lock()
			exp, ok := sessions[c.Value]
			sessMu.Unlock()
			if ok && time.Now().Before(exp) {
				next(w, r)
				return
			}
		}
		if u, p, ok := r.BasicAuth(); ok && u == "backeddy" {
			ip := clientIP(r)
			if !tooMany(ip) && passwordOK(p) {
				next(w, r)
				return
			}
			failed(ip)
		}
		http.Error(w, "senha necessária", http.StatusUnauthorized)
	}
}

func apiLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct{ Password string }
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req)
	ip := clientIP(r)
	if tooMany(ip) {
		http.Error(w, "muitas tentativas; espere 10 minutos", http.StatusTooManyRequests)
		return
	}
	if !passwordOK(req.Password) {
		failed(ip)
		http.Error(w, "senha errada", http.StatusUnauthorized)
		return
	}
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	tok := hex.EncodeToString(b)
	sessMu.Lock()
	sessions[tok] = time.Now().Add(30 * 24 * time.Hour)
	sessMu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "backeddy", Value: tok, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteStrictMode, MaxAge: 30 * 24 * 3600})
	w.WriteHeader(http.StatusNoContent)
}

type statusResp struct {
	Name       string  `json:"name"`
	Alerts     []Alert `json:"alerts"`
	Status     string  `json:"status"`
	QuietStart int     `json:"quietStart"`
	QuietEnd   int     `json:"quietEnd"`
}

func apiStatus(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	p := append([]Alert{}, pending...)
	mu.Unlock()
	cmd := exec.Command(backeddy, "status")
	cmd.Env = append(os.Environ(), "TERM=dumb")
	out, _ := cmd.Output()
	qs, qe := quietHours()
	name := ""
	if data, err := os.ReadFile("/etc/backeddy/server.conf"); err == nil {
		for _, l := range strings.Split(string(data), "\n") {
			if v, ok := strings.CutPrefix(l, "NAME="); ok {
				name = strings.Trim(v, `"`)
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(statusResp{Name: name, Alerts: p, Status: string(out), QuietStart: qs, QuietEnd: qe})
}

// apiHello: o instalador do Windows descobre o nome do servidor recém-instalado.
func apiHello(w http.ResponseWriter, r *http.Request) {
	conf := map[string]string{}
	if data, err := os.ReadFile("/etc/backeddy/server.conf"); err == nil {
		for _, l := range strings.Split(string(data), "\n") {
			if k, v, ok := strings.Cut(l, "="); ok {
				conf[k] = strings.Trim(v, `"`)
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"name": conf["NAME"], "slug": conf["SLUG"], "version": conf["VERSION"]})
}

// apiBeep: o vigia do Windows (VM sem alto-falante) pergunta a cada minuto se deve bipar.
// Não exige senha e não revela detalhes: só "sim/não" e quantos alertas.
func apiBeep(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	n := len(pending)
	mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"beep": n > 0 && !quietNow(time.Now()), "alerts": n})
}

func apiAck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct{ Days int }
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req)
	if req.Days < 1 || req.Days > 30 {
		req.Days = 1
	}
	if err := exec.Command(backeddy, "ack", strconv.Itoa(req.Days)).Run(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	check()
	w.WriteHeader(http.StatusNoContent)
}

func apiQuiet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Off        bool
		Start, End int
	}
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req)
	args := []string{"quiet", "off"}
	if !req.Off {
		if req.Start < 0 || req.Start > 23 || req.End < 0 || req.End > 23 {
			http.Error(w, "horas de 0 a 23", http.StatusBadRequest)
			return
		}
		args = []string{"quiet", strconv.Itoa(req.Start), strconv.Itoa(req.End)}
	}
	if err := exec.Command(backeddy, args...).Run(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// "Procurar atualização agora" no painel: roda o backeddy update numa unidade própria do systemd,
// para continuar mesmo quando a atualização reinicia este vigia.
func apiUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	cmd := exec.Command("systemd-run", "--no-block", "--collect", "--unit=backeddy-update-now", backeddy, "update", "now")
	if out, err := cmd.CombinedOutput(); err != nil {
		http.Error(w, strings.TrimSpace(string(out)), http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
