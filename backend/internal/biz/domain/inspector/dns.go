package inspector

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"

	"gmssh-cli-plugin/internal/biz/domain/types"
)

type DNSRecordItem struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	Value  string `json:"value"`
	TTL    uint32 `json:"ttl"`
	Source string `json:"source"`
}

type DNSData struct {
	Domain     string          `json:"domain"`
	Nameserver string          `json:"nameserver"`
	Records    []DNSRecordItem `json:"records"`
	TotalCount int             `json:"totalCount"`
	HasA       bool            `json:"hasA"`
	HasAAAA    bool            `json:"hasAAAA"`
	HasCNAME   bool            `json:"hasCNAME"`
	PrimaryIP  string          `json:"primaryIP,omitempty"`
	// 查询超时/失败的记录类型：这些类型是"未知"，不是"没有记录"
	FailedTypes []string `json:"failedTypes,omitempty"`
}

// 优先使用本机 resolv.conf 里的 nameserver，读不到再退回 AliDNS
func systemResolver() string {
	if data, err := os.ReadFile("/etc/resolv.conf"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			f := strings.Fields(line)
			if len(f) >= 2 && f[0] == "nameserver" {
				addr := f[1]
				if !strings.Contains(addr, ":") {
					addr += ":53"
				}
				return addr
			}
		}
	}
	return "223.5.5.5:53"
}

func querySingleType(ctx context.Context, server, domain string, qType uint16) ([]DNSRecordItem, error) {
	c := new(dns.Client)
	c.Timeout = 3 * time.Second

	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(domain), qType)
	m.RecursionDesired = true

	r, _, err := c.ExchangeContext(ctx, m, server)
	if err != nil {
		// Fallback to TCP if truncated
		if r != nil && r.Truncated {
			c.Net = "tcp"
			r, _, err = c.ExchangeContext(ctx, m, server)
		}
	}
	if err != nil {
		return nil, err
	}

	var results []DNSRecordItem
	typeName := dns.TypeToString[qType]

	for _, ans := range r.Answer {
		val := ""
		switch v := ans.(type) {
		case *dns.A:
			val = v.A.String()
		case *dns.AAAA:
			val = v.AAAA.String()
		case *dns.CNAME:
			val = strings.TrimSuffix(v.Target, ".")
		case *dns.MX:
			val = fmt.Sprintf("%d %s", v.Preference, strings.TrimSuffix(v.Mx, "."))
		case *dns.TXT:
			val = strings.Join(v.Txt, " ")
		case *dns.NS:
			val = strings.TrimSuffix(v.Ns, ".")
		case *dns.CAA:
			val = fmt.Sprintf("%d %s %q", v.Flag, v.Tag, v.Value)
		case *dns.SOA:
			val = fmt.Sprintf("%s %s %d %d %d %d %d",
				strings.TrimSuffix(v.Ns, "."), strings.TrimSuffix(v.Mbox, "."),
				v.Serial, v.Refresh, v.Retry, v.Expire, v.Minttl)
		default:
			val = ans.String()
		}

		results = append(results, DNSRecordItem{
			Type:   typeName,
			Name:   strings.TrimSuffix(ans.Header().Name, "."),
			Value:  val,
			TTL:    ans.Header().Ttl,
			Source: server,
		})
	}

	return results, nil
}

func InspectDNS(ctx context.Context, domain string) (*types.ModuleEnvelope, error) {
	start := time.Now()
	envelope := &types.ModuleEnvelope{
		Module:    "dns",
		Domain:    domain,
		Status:    types.StatusSuccess,
		StartedAt: start,
		Source:    "Local / Recursive DNS",
	}

	dnsServer := systemResolver()
	typesToQuery := []uint16{
		dns.TypeA,
		dns.TypeAAAA,
		dns.TypeCNAME,
		dns.TypeMX,
		dns.TypeTXT,
		dns.TypeNS,
		dns.TypeCAA,
		dns.TypeSOA,
	}

	var allRecords []DNSRecordItem
	var failedTypes []string
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, qt := range typesToQuery {
		wg.Add(1)
		go func(q uint16) {
			defer wg.Done()
			records, err := querySingleType(ctx, dnsServer, domain, q)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failedTypes = append(failedTypes, dns.TypeToString[q])
				return
			}
			allRecords = append(allRecords, records...)
		}(qt)
	}

	wg.Wait()

	// 并发收集，排序后输出才稳定
	sort.Slice(allRecords, func(i, j int) bool {
		if allRecords[i].Type != allRecords[j].Type {
			return allRecords[i].Type < allRecords[j].Type
		}
		return allRecords[i].Value < allRecords[j].Value
	})
	sort.Strings(failedTypes)

	data := &DNSData{
		Domain:      domain,
		Nameserver:  dnsServer,
		Records:     allRecords,
		TotalCount:  len(allRecords),
		FailedTypes: failedTypes,
	}

	for _, r := range allRecords {
		if r.Type == "A" {
			data.HasA = true
			if data.PrimaryIP == "" {
				data.PrimaryIP = r.Value
			}
		} else if r.Type == "AAAA" {
			data.HasAAAA = true
		} else if r.Type == "CNAME" {
			data.HasCNAME = true
		}
	}

	if len(allRecords) == 0 {
		envelope.Status = types.StatusWarning
		envelope.Error = "no DNS records found"
	}

	envelope.Data = data
	envelope.FinishedAt = time.Now()
	envelope.DurationMs = time.Since(start).Milliseconds()
	return envelope, nil
}
