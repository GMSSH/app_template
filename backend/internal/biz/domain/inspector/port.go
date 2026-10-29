package inspector

import (
	"context"
	"fmt"
	"sync"
	"time"

	"gmssh-cli-plugin/internal/biz/domain/security"
	"gmssh-cli-plugin/internal/biz/domain/types"
)

type PortItem struct {
	Port       int    `json:"port"`
	Protocol   string `json:"protocol"`
	Open       bool   `json:"open"`
	LatencyMs  int64  `json:"latencyMs"`
	StatusText string `json:"statusText"` // 开放, 拒绝/超时, 错误
	Error      string `json:"error,omitempty"`
}

type PortData struct {
	Domain string     `json:"domain"`
	Ports  []PortItem `json:"ports"`
}

func checkPort(ctx context.Context, domain string, port int) PortItem {
	item := PortItem{
		Port:     port,
		Protocol: "TCP",
	}

	dialer := security.SafeDialContext(3 * time.Second)
	start := time.Now()
	conn, err := dialer(ctx, "tcp", fmt.Sprintf("%s:%d", domain, port))
	item.LatencyMs = time.Since(start).Milliseconds()

	if err != nil {
		item.Open = false
		item.StatusText = "关闭/超时"
		item.Error = err.Error()
	} else {
		_ = conn.Close()
		item.Open = true
		item.StatusText = "开放"
	}

	return item
}

func InspectPort(ctx context.Context, domain string) (*types.ModuleEnvelope, error) {
	start := time.Now()
	envelope := &types.ModuleEnvelope{
		Module:    "port",
		Domain:    domain,
		Status:    types.StatusSuccess,
		StartedAt: start,
		Source:    "TCP Safe Dial (80, 443)",
	}

	ports := []int{80, 443}
	var results []PortItem
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, p := range ports {
		wg.Add(1)
		go func(portNum int) {
			defer wg.Done()
			res := checkPort(ctx, domain, portNum)
			mu.Lock()
			results = append(results, res)
			mu.Unlock()
		}(p)
	}

	wg.Wait()

	hasAnyOpen := false
	for _, p := range results {
		if p.Open {
			hasAnyOpen = true
			break
		}
	}

	if !hasAnyOpen {
		envelope.Status = types.StatusWarning
		envelope.Error = "ports 80 and 443 are both closed or unreachable"
	}

	envelope.Data = &PortData{
		Domain: domain,
		Ports:  results,
	}

	envelope.FinishedAt = time.Now()
	envelope.DurationMs = time.Since(start).Milliseconds()
	return envelope, nil
}
