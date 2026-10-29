package inspector

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"

	"gmssh-cli-plugin/internal/biz/domain/types"
)

type MXItem struct {
	Preference uint16 `json:"preference"`
	Host       string `json:"host"`
}

type MailData struct {
	Domain      string   `json:"domain"`
	MXRecords   []MXItem `json:"mxRecords"`
	HasMX       bool     `json:"hasMX"`
	SPFRecord   string   `json:"spfRecord,omitempty"`
	HasSPF      bool     `json:"hasSPF"`
	DMARCRecord string   `json:"dmarcRecord,omitempty"`
	DMARCPolicy string   `json:"dmarcPolicy,omitempty"`
	HasDMARC    bool     `json:"hasDMARC"`
	DKIMStatus  string   `json:"dkimStatus"` // 未提供选择器 (Selector)
	Summary     string   `json:"summary"`
}

func InspectMail(ctx context.Context, domain string) (*types.ModuleEnvelope, error) {
	start := time.Now()
	envelope := &types.ModuleEnvelope{
		Module:    "mail",
		Domain:    domain,
		Status:    types.StatusSuccess,
		StartedAt: start,
		Source:    "DNS MX / SPF / DMARC Inspection",
	}

	data := &MailData{
		Domain:     domain,
		MXRecords:  []MXItem{},
		DKIMStatus: "未指定选择器 (需提供 selector 才能精确定位)",
	}

	dnsServer := "223.5.5.5:53"
	var wg sync.WaitGroup

	// 1. Query MX
	wg.Add(1)
	go func() {
		defer wg.Done()
		c := new(dns.Client)
		c.Timeout = 3 * time.Second
		m := new(dns.Msg)
		m.SetQuestion(dns.Fqdn(domain), dns.TypeMX)
		r, _, err := c.ExchangeContext(ctx, m, dnsServer)
		if err == nil && r != nil {
			for _, ans := range r.Answer {
				if mx, ok := ans.(*dns.MX); ok {
					data.MXRecords = append(data.MXRecords, MXItem{
						Preference: mx.Preference,
						Host:       strings.TrimSuffix(mx.Mx, "."),
					})
				}
			}
			if len(data.MXRecords) > 0 {
				data.HasMX = true
			}
		}
	}()

	// 2. Query SPF (TXT records on domain)
	wg.Add(1)
	go func() {
		defer wg.Done()
		c := new(dns.Client)
		c.Timeout = 3 * time.Second
		m := new(dns.Msg)
		m.SetQuestion(dns.Fqdn(domain), dns.TypeTXT)
		r, _, err := c.ExchangeContext(ctx, m, dnsServer)
		if err == nil && r != nil {
			for _, ans := range r.Answer {
				if txt, ok := ans.(*dns.TXT); ok {
					fullTxt := strings.Join(txt.Txt, "")
					if strings.HasPrefix(strings.ToLower(fullTxt), "v=spf1") {
						data.SPFRecord = fullTxt
						data.HasSPF = true
						break
					}
				}
			}
		}
	}()

	// 3. Query DMARC (TXT record on _dmarc.domain)
	wg.Add(1)
	go func() {
		defer wg.Done()
		c := new(dns.Client)
		c.Timeout = 3 * time.Second
		m := new(dns.Msg)
		m.SetQuestion(dns.Fqdn("_dmarc."+domain), dns.TypeTXT)
		r, _, err := c.ExchangeContext(ctx, m, dnsServer)
		if err == nil && r != nil {
			for _, ans := range r.Answer {
				if txt, ok := ans.(*dns.TXT); ok {
					fullTxt := strings.Join(txt.Txt, "")
					if strings.Contains(strings.ToLower(fullTxt), "v=dmarc1") {
						data.DMARCRecord = fullTxt
						data.HasDMARC = true
						// parse policy
						parts := strings.Split(fullTxt, ";")
						for _, p := range parts {
							kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
							if len(kv) == 2 && strings.ToLower(kv[0]) == "p" {
								data.DMARCPolicy = strings.ToLower(kv[1])
							}
						}
						break
					}
				}
			}
		}
	}()

	wg.Wait()

	if !data.HasMX && !data.HasSPF && !data.HasDMARC {
		envelope.Status = types.StatusWarning
		data.Summary = "未检测到邮件相关配置（该域名可能未用于企业邮箱）"
	} else {
		var parts []string
		if data.HasMX {
			parts = append(parts, fmt.Sprintf("%d 条 MX 记录", len(data.MXRecords)))
		}
		if data.HasSPF {
			parts = append(parts, "SPF 已配置")
		}
		if data.HasDMARC {
			parts = append(parts, fmt.Sprintf("DMARC(%s)", data.DMARCPolicy))
		}
		data.Summary = strings.Join(parts, " · ")
	}

	envelope.Data = data
	envelope.FinishedAt = time.Now()
	envelope.DurationMs = time.Since(start).Milliseconds()
	return envelope, nil
}
