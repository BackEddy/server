package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPCName(t *testing.T) {
	for in, want := range map[string]string{
		"DESKTOP-AB12CD": "desktop-ab12cd", "PC da Sala": "pc-da-sala", "  Notebook_Ção!! ": "notebook-o",
		"---": "", strings.Repeat("a", 60): strings.Repeat("a", 40),
	} {
		if got := pcName(in); got != want {
			t.Errorf("pcName(%q) = %q, quero %q", in, got, want)
		}
	}
}

func TestAPIPair(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "backeddy")
	// finge o "backeddy add-pc NOME --json"
	script := "#!/bin/sh\n[ \"$1\" = add-pc ] && [ \"$3\" = --json ] || exit 1\n" +
		"printf '{\"pc\":\"%s\",\"username\":\"backeddy\",\"hostname\":\"%s\",\"password\":\"x1\",\"url\":\"https://casa.local:51515\",\"fingerprint\":\"ab\"}' \"$2\" \"$2\"\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	oldB, oldP := backeddy, passwordOK
	defer func() { backeddy, passwordOK = oldB, oldP }()
	backeddy = fake
	passwordOK = func(pw string) bool { return pw == "certa" }

	call := func(body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/pair", strings.NewReader(body))
		req.RemoteAddr = "192.0.2.50:5000"
		apiPair(rec, req)
		return rec
	}
	if rec := call(`{"password":"errada","name":"PC da Sala"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("senha errada: %d", rec.Code)
	}
	rec := call(`{"password":"certa","name":"PC da Sala"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("senha certa: %d %s", rec.Code, rec.Body)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil || resp["hostname"] != "pc-da-sala" || resp["fingerprint"] != "ab" {
		t.Fatalf("resposta: %v %s", err, rec.Body)
	}
	if rec := call(`{"password":"certa","name":"!!!"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("nome vazio: %d", rec.Code)
	}
	// 10 erros seguidos bloqueiam o endereço
	for i := 0; i < 10; i++ {
		call(`{"password":"errada","name":"x"}`)
	}
	if rec := call(`{"password":"certa","name":"x"}`); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("limite de tentativas: %d", rec.Code)
	}
}
