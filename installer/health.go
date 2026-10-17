package main

// Saúde dos discos pelo SMART (smartctl em JSON). Sem SMART (máquina virtual, algumas caixas USB)
// o disco fica "desconhecido" e é usado normalmente.

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

type smartJSON struct {
	SmartStatus *struct {
		Passed bool `json:"passed"`
	} `json:"smart_status"`
	PowerOnTime struct {
		Hours int `json:"hours"`
	} `json:"power_on_time"`
	ATA struct {
		Table []struct {
			ID  int `json:"id"`
			Raw struct {
				Value int64 `json:"value"`
			} `json:"raw"`
		} `json:"table"`
	} `json:"ata_smart_attributes"`
	NVMe *struct {
		CriticalWarning int   `json:"critical_warning"`
		MediaErrors     int64 `json:"media_errors"`
		PercentageUsed  int   `json:"percentage_used"`
	} `json:"nvme_smart_health_information_log"`
}

// judgeSmart transforma a saída do smartctl numa avaliação.
func judgeSmart(data []byte) (Health, string) {
	var s smartJSON
	if json.Unmarshal(data, &s) != nil || s.SmartStatus == nil {
		return HealthUnknown, ""
	}
	if !s.SmartStatus.Passed {
		return HealthBad, T("o próprio disco diz que vai falhar")
	}
	var realloc, pending int64
	for _, a := range s.ATA.Table {
		switch a.ID {
		case 5:
			realloc = a.Raw.Value
		case 197, 198:
			pending += a.Raw.Value
		}
	}
	if pending > 0 {
		return HealthBad, fmt.Sprintf(T("%d setores ilegíveis"), pending)
	}
	if s.NVMe != nil {
		if s.NVMe.CriticalWarning != 0 || s.NVMe.MediaErrors > 0 {
			return HealthBad, T("o SSD registrou erros graves")
		}
		if s.NVMe.PercentageUsed >= 100 {
			return HealthOld, T("SSD no fim da vida útil")
		}
	}
	if realloc > 0 {
		return HealthOld, fmt.Sprintf(T("já trocou %d setores com defeito; prefira o modo Protegido"), realloc)
	}
	if h := s.PowerOnTime.Hours; h >= 50000 {
		return HealthOld, fmt.Sprintf(T("%d mil horas ligado (uns %d anos); prefira o modo Protegido"), h/1000, h/8760)
	}
	return HealthOK, ""
}

// CheckHealth preenche a saúde de cada disco (alguns segundos no total).
func CheckHealth(disks []Disk) {
	for i := range disks {
		if disks[i].Installer {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		out, _ := exec.CommandContext(ctx, "smartctl", "-j", "-H", "-A", disks[i].Path).Output()
		cancel()
		disks[i].Health, disks[i].HealthNote = judgeSmart(out)
	}
}
