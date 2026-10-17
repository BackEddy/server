package main

// Arquivo de plano gravado no pendrive pelo app Backeddy (partição BACKEDDY).

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const planPath = "/run/backeddy-plan/plan.json"

type PlanComputer struct {
	Name  string `json:"name"`
	Kind  string `json:"kind"` // "mac" ou "windows"
	Bytes int64  `json:"bytes"`
}

type Plan struct {
	Version          int            `json:"version"`
	Name             string         `json:"name"`
	MinBytes         int64          `json:"minBytes"`
	RecommendedBytes int64          `json:"recommendedBytes"`
	Computers        []PlanComputer `json:"computers"`
	Timezone         string         `json:"timezone"`
	Language         string         `json:"language"`
}

// LoadPlan lê o plano; sem arquivo devolve nil (instalação sem plano é permitida).
func LoadPlan(path string) (*Plan, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var p Plan
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("plano inválido: %w", err)
	}
	return &p, nil
}

type Fit int

const (
	FitUnknown Fit = iota
	FitGreen
	FitYellow
	FitRed
)

func (p *Plan) Check(usable int64) Fit {
	if p == nil || p.MinBytes <= 0 {
		return FitUnknown
	}
	switch {
	case p.RecommendedBytes > 0 && usable >= p.RecommendedBytes:
		return FitGreen
	case usable >= p.MinBytes:
		return FitYellow
	default:
		return FitRed
	}
}

func (p *Plan) HasWindows() bool {
	if p == nil {
		return false
	}
	for _, c := range p.Computers {
		if strings.EqualFold(c.Kind, "windows") {
			return true
		}
	}
	return false
}

func (p *Plan) ComputerNames() string {
	if p == nil || len(p.Computers) == 0 {
		return "os computadores planejados"
	}
	var names []string
	for _, c := range p.Computers {
		names = append(names, "\""+c.Name+"\"")
	}
	if len(names) == 1 {
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " e " + names[len(names)-1]
}

// TMPercent: quanto do espaço vai para o Time Machine (o resto fica para os PCs).
func (p *Plan) TMPercent() int {
	if p == nil {
		return 60
	}
	if !p.HasWindows() {
		return 85
	}
	macs := 0
	for _, c := range p.Computers {
		if strings.EqualFold(c.Kind, "mac") {
			macs++
		}
	}
	if macs == 0 {
		return 10
	}
	return 60
}
