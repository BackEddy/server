package main

import (
	"fmt"
	"testing"
)

func TestInQuiet(t *testing.T) {
	cases := []struct {
		h, s, e int
		want    bool
	}{
		{23, 22, 8, true}, {3, 22, 8, true}, {8, 22, 8, false}, {12, 22, 8, false},
		{14, 13, 15, true}, {15, 13, 15, false}, {3, -1, -1, false}, {5, 5, 5, false},
	}
	for _, c := range cases {
		if got := inQuiet(c.h, c.s, c.e); got != c.want {
			t.Errorf("inQuiet(%d,%d,%d)=%v", c.h, c.s, c.e, got)
		}
	}
}

func TestMD4(t *testing.T) {
	// vetores do RFC 1320 e um hash NT conhecido
	if fmt.Sprintf("%x", md4([]byte("abc"))) != "a448017aaf21d8525fc10ae87aa6729d" {
		t.Fatal("md4 abc")
	}
	if fmt.Sprintf("%x", md4(nil)) != "31d6cfe0d16ae931b73c59d7e0c089c0" {
		t.Fatal("md4 vazio")
	}
	if fmt.Sprintf("%X", ntHash("password")) != "8846F7EAEE8FB117AD06BDD830B7586C" {
		t.Fatal("nt hash")
	}
}
