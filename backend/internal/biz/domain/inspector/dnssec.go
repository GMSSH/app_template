package inspector

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/miekg/dns"

	"gmssh-cli-plugin/internal/biz/domain/types"
)

type DNSKEYItem struct {
	Algorithm uint8  `json:"algorithm"`
	Flags     uint16 `json:"flags"`
	PublicKey string `json:"publicKey"`
	KeyTag    uint16 `json:"keyTag"`
}

type DSItem struct {
	KeyTag     uint16 `json:"keyTag"`
	Algorithm  uint8  `json:"algorithm"`
	DigestType uint8  `json:"digestType"`
	Digest     string `json:"digest"`
}

type DNSSECData struct {
	Domain        string       `json:"domain"`
	HasDS         bool         `json:"hasDS"`
	HasDNSKEY     bool         `json:"hasDNSKEY"`
	HasRRSIG      bool         `json:"hasRRSIG"`
	DSRecords     []DSItem     `json:"dsRecords"`
	DNSKEYRecords []DNSKEYItem `json:"dnskeyRecords"`
	StatusLabel   string       `json:"statusLabel"` // 未启用, 验证通过, 验证失败, 无法验证
	Summary       string       `json:"summary"`
}

func InspectDNSSEC(ctx context.Context, domain string) (*types.ModuleEnvelope, error) {
	start := time.Now()
	envelope := &types.ModuleEnvelope{
		Module:    "dnssec",
		Domain:    domain,
		Status:    types.StatusNotConfigured,
		StartedAt: start,
		Source:    "DNSSEC DS/DNSKEY/RRSIG Probe",
	}

	data := &DNSSECData{
		Domain:        domain,
		DSRecords:     []DSItem{},
		DNSKEYRecords: []DNSKEYItem{},
		StatusLabel:   "未启用",
	}

	dnsServer := "223.5.5.5:53"
	var wg sync.WaitGroup

	// 1. Query DS record
	wg.Add(1)
	go func() {
		defer wg.Done()
		c := new(dns.Client)
		c.Timeout = 3 * time.Second
		m := new(dns.Msg)
		m.SetQuestion(dns.Fqdn(domain), dns.TypeDS)
		m.RecursionDesired = true
		r, _, err := c.ExchangeContext(ctx, m, dnsServer)
		if err == nil && r != nil {
			for _, ans := range r.Answer {
				if ds, ok := ans.(*dns.DS); ok {
					data.HasDS = true
					data.DSRecords = append(data.DSRecords, DSItem{
						KeyTag:     ds.KeyTag,
						Algorithm:  ds.Algorithm,
						DigestType: ds.DigestType,
						Digest:     ds.Digest,
					})
				}
			}
		}
	}()

	// 2. Query DNSKEY record
	wg.Add(1)
	go func() {
		defer wg.Done()
		c := new(dns.Client)
		c.Timeout = 3 * time.Second
		m := new(dns.Msg)
		m.SetQuestion(dns.Fqdn(domain), dns.TypeDNSKEY)
		m.RecursionDesired = true
		r, _, err := c.ExchangeContext(ctx, m, dnsServer)
		if err == nil && r != nil {
			for _, ans := range r.Answer {
				if dk, ok := ans.(*dns.DNSKEY); ok {
					data.HasDNSKEY = true
					data.DNSKEYRecords = append(data.DNSKEYRecords, DNSKEYItem{
						Algorithm: dk.Algorithm,
						Flags:     dk.Flags,
						PublicKey: dk.PublicKey,
						KeyTag:    dk.KeyTag(),
					})
				}
			}
		}
	}()

	// 3. Query A record with DO (DNSSEC OK) bit to check RRSIG
	wg.Add(1)
	go func() {
		defer wg.Done()
		c := new(dns.Client)
		c.Timeout = 3 * time.Second
		m := new(dns.Msg)
		m.SetQuestion(dns.Fqdn(domain), dns.TypeA)
		m.SetEdns0(4096, true) // DO bit = true
		m.RecursionDesired = true
		r, _, err := c.ExchangeContext(ctx, m, dnsServer)
		if err == nil && r != nil {
			for _, ans := range r.Answer {
				if _, ok := ans.(*dns.RRSIG); ok {
					data.HasRRSIG = true
					break
				}
			}
			// Also check authenticated data flag
			if r.AuthenticatedData {
				data.HasRRSIG = true
			}
		}
	}()

	wg.Wait()

	if data.HasDS && data.HasDNSKEY {
		data.StatusLabel = "验证通过"
		envelope.Status = types.StatusSuccess
		data.Summary = fmt.Sprintf("已启用 DNSSEC 安全链（%d 个 DS，%d 个 DNSKEY）", len(data.DSRecords), len(data.DNSKEYRecords))
	} else if data.HasDS || data.HasDNSKEY || data.HasRRSIG {
		data.StatusLabel = "部分配置"
		envelope.Status = types.StatusWarning
		data.Summary = "检测到部分 DNSSEC 记录，信任链可能尚未完全闭合"
	} else {
		data.StatusLabel = "未启用"
		envelope.Status = types.StatusNotConfigured
		data.Summary = "域名未启用 DNSSEC 签名保护"
	}

	envelope.Data = data
	envelope.FinishedAt = time.Now()
	envelope.DurationMs = time.Since(start).Milliseconds()
	return envelope, nil
}
