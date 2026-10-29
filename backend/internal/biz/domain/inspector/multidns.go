package inspector

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"

	"gmssh-cli-plugin/internal/biz/domain/types"
)

type ResolverItem struct {
	Name       string   `json:"name"`
	Endpoint   string   `json:"endpoint"`
	Status     string   `json:"status"` // success, error, timeout
	DurationMs int64    `json:"durationMs"`
	IPs        []string `json:"ips"`
	TTL        uint32   `json:"ttl"`
	Error      string   `json:"error,omitempty"`
}

type MultiDNSData struct {
	Domain     string         `json:"domain"`
	Resolvers  []ResolverItem `json:"resolvers"`
	Consistent bool           `json:"consistent"`
	Summary    string         `json:"summary"`
}

// 同一组 IP 只是顺序不同时不算差异
func sameIPSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	sa := append([]string(nil), a...)
	sb := append([]string(nil), b...)
	sort.Strings(sa)
	sort.Strings(sb)
	return strings.Join(sa, ",") == strings.Join(sb, ",")
}

var DefaultResolvers = []struct {
	Name string
	Addr string
}{
	{Name: "本地/系统 DNS", Addr: ""},
	{Name: "Cloudflare (1.1.1.1)", Addr: "1.1.1.1:53"},
	{Name: "Google (8.8.8.8)", Addr: "8.8.8.8:53"},
	{Name: "AliDNS (223.5.5.5)", Addr: "223.5.5.5:53"},
}

func queryResolver(ctx context.Context, name, addr, domain string) ResolverItem {
	item := ResolverItem{
		Name:     name,
		Endpoint: addr,
		Status:   types.StatusSuccess,
		IPs:      []string{},
	}
	start := time.Now()

	// If Addr is empty, use system resolver
	if addr == "" {
		resolver := net.DefaultResolver
		ips, err := resolver.LookupIP(ctx, "ip4", domain)
		item.DurationMs = time.Since(start).Milliseconds()
		if err != nil {
			item.Status = types.StatusError
			item.Error = err.Error()
			item.Endpoint = "Local Resolver"
			return item
		}
		for _, ip := range ips {
			item.IPs = append(item.IPs, ip.String())
		}
		item.Endpoint = "System Resolver"
		return item
	}

	c := new(dns.Client)
	c.Timeout = 3 * time.Second

	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(domain), dns.TypeA)
	m.RecursionDesired = true

	r, rtt, err := c.ExchangeContext(ctx, m, addr)
	item.DurationMs = rtt.Milliseconds()
	if err != nil {
		item.Status = types.StatusError
		item.Error = err.Error()
		return item
	}

	for _, ans := range r.Answer {
		if a, ok := ans.(*dns.A); ok {
			item.IPs = append(item.IPs, a.A.String())
			if item.TTL == 0 || a.Hdr.Ttl < item.TTL {
				item.TTL = a.Hdr.Ttl
			}
		}
	}

	if len(item.IPs) == 0 {
		item.Status = types.StatusWarning
		item.Error = "no A records returned"
	}

	return item
}

func InspectMultiDNS(ctx context.Context, domain string) (*types.ModuleEnvelope, error) {
	start := time.Now()
	envelope := &types.ModuleEnvelope{
		Module:    "multidns",
		Domain:    domain,
		Status:    types.StatusSuccess,
		StartedAt: start,
		Source:    "Multi-Resolver Comparison",
	}

	// 按 DefaultResolvers 顺序预分配，保证每次输出顺序一致
	results := make([]ResolverItem, len(DefaultResolvers))
	var wg sync.WaitGroup

	for i, res := range DefaultResolvers {
		wg.Add(1)
		go func(idx int, rName, rAddr string) {
			defer wg.Done()
			results[idx] = queryResolver(ctx, rName, rAddr, domain)
		}(i, res.Name, res.Addr)
	}

	wg.Wait()

	var firstIPSet []string
	allSame := true
	successCount := 0
	failCount := 0

	for _, r := range results {
		if r.Status == types.StatusSuccess && len(r.IPs) > 0 {
			successCount++
			if firstIPSet == nil {
				firstIPSet = r.IPs
			} else if !sameIPSet(firstIPSet, r.IPs) {
				allSame = false
			}
		} else {
			failCount++
		}
	}

	consistent := allSame && successCount > 1
	summary := "所有成功响应的解析器返回一致结果"
	if successCount == 0 {
		summary = "所有解析器均未成功解析 A 记录"
		envelope.Status = types.StatusWarning
	} else if successCount == 1 {
		summary = "仅 1 个解析器成功响应，无法进行对比"
		envelope.Status = types.StatusWarning
	} else if !consistent {
		summary = "各解析器结果不同（可能为 CDN / GeoDNS 就近分流）"
	}
	if failCount > 0 && successCount > 0 {
		if consistent {
			envelope.Status = types.StatusWarning
		}
		summary += fmt.Sprintf("；%d 个解析器响应失败", failCount)
	}

	envelope.Data = &MultiDNSData{
		Domain:     domain,
		Resolvers:  results,
		Consistent: consistent,
		Summary:    summary,
	}

	envelope.FinishedAt = time.Now()
	envelope.DurationMs = time.Since(start).Milliseconds()
	return envelope, nil
}
