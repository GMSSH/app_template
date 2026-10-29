package inspector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/publicsuffix"

	"gmssh-cli-plugin/internal/biz/domain/types"
)

type RDAPData struct {
	DomainName       string   `json:"domainName"`
	RegisteredDomain string   `json:"registeredDomain"`
	Registrar        string   `json:"registrar"`
	RegistrationDate string   `json:"registrationDate"`
	UpdatedDate      string   `json:"updatedDate"`
	Registrant       string   `json:"registrant,omitempty"`
	ExpirationDate   string   `json:"expirationDate"`
	RemainingDays    *int     `json:"remainingDays"`
	Status           []string `json:"status"`
	Nameservers      []string `json:"nameservers"`
	Source           string   `json:"source"`
	RawRDAP          string   `json:"rawRDAP,omitempty"`
}

var (
	bootstrapOnce sync.Once
	bootstrapMap  map[string]string
)

func initBootstrap() {
	bootstrapMap = map[string]string{
		"com":  "https://rdap.verisign.com/com/v1/",
		"net":  "https://rdap.verisign.com/net/v1/",
		"org":  "https://rdap.publicinterestregistry.org/rdap/",
		"io":   "https://rdap.identitydigital.services/rdap/",
		"cc":   "https://rdap.verisign.com/cc/v1/",
		"me":   "https://rdap.identitydigital.services/rdap/",
		"info": "https://rdap.identitydigital.services/rdap/",
		"xyz":  "https://rdap.centralnic.com/xyz/",
		"top":  "https://rdap.nic.top/",
		"dev":  "https://rdap.google.com/",
		"app":  "https://rdap.google.com/",
	}
}

// GetRDAPEndpoint returns the bootstrap endpoint or queries IANA
func GetRDAPEndpoint(tld string) string {
	bootstrapOnce.Do(initBootstrap)
	if url, ok := bootstrapMap[tld]; ok {
		return url
	}
	return "https://rdap.org/" // Generic fallback service
}

func InspectRDAP(ctx context.Context, domain string) (*types.ModuleEnvelope, error) {
	start := time.Now()
	envelope := &types.ModuleEnvelope{
		Module:    "rdap",
		Domain:    domain,
		Status:    types.StatusSuccess,
		StartedAt: start,
		Source:    "IANA/RDAP Bootstrap",
	}

	regDomain, err := publicsuffix.EffectiveTLDPlusOne(domain)
	if err != nil {
		regDomain = domain
	}

	parts := strings.Split(regDomain, ".")
	tld := parts[len(parts)-1]
	rdapBase := GetRDAPEndpoint(tld)
	queryURL := strings.TrimSuffix(rdapBase, "/") + "/domain/" + regDomain

	// .cn 等 TLD 没有公开 RDAP 服务：直接走 WHOIS，省一次注定失败的请求
	if _, hasRDAP := bootstrapMap[tld]; !hasRDAP {
		if data := inspectWhois(ctx, regDomain); data != nil {
			envelope.Data = data
			envelope.Source = data.Source
			envelope.FinishedAt = time.Now()
			envelope.DurationMs = time.Since(start).Milliseconds()
			return envelope, nil
		}
	}

	// RDAP 失败的兜底：部分注册局 RDAP 不稳定，WHOIS 通常仍可用
	fallback := func(status, reason string) (*types.ModuleEnvelope, error) {
		if data := inspectWhois(ctx, regDomain); data != nil {
			envelope.Data = data
			envelope.Source = data.Source
			envelope.Error = ""
			envelope.FinishedAt = time.Now()
			envelope.DurationMs = time.Since(start).Milliseconds()
			return envelope, nil
		}
		envelope.Status = status
		envelope.Error = reason
		envelope.FinishedAt = time.Now()
		envelope.DurationMs = time.Since(start).Milliseconds()
		envelope.Data = &RDAPData{
			DomainName:       domain,
			RegisteredDomain: regDomain,
			Source:           queryURL,
		}
		return envelope, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, queryURL, nil)
	if err != nil {
		envelope.Status = types.StatusError
		envelope.Error = err.Error()
		envelope.FinishedAt = time.Now()
		envelope.DurationMs = time.Since(start).Milliseconds()
		return envelope, nil
	}
	req.Header.Set("Accept", "application/rdap+json, application/json")
	req.Header.Set("User-Agent", "GMSSH-DomainToolbox/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fallback(types.StatusUnavailable, fmt.Sprintf("RDAP query failed: %v", err))
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		envelope.Status = types.StatusError
		envelope.Error = err.Error()
		envelope.FinishedAt = time.Now()
		envelope.DurationMs = time.Since(start).Milliseconds()
		return envelope, nil
	}

	if resp.StatusCode == http.StatusNotFound {
		return fallback(types.StatusNotConfigured, "domain not found in registry (NXDOMAIN or unregistered)")
	}

	if resp.StatusCode != http.StatusOK {
		return fallback(types.StatusUnavailable, fmt.Sprintf("RDAP registry returned HTTP %d", resp.StatusCode))
	}

	var parsed map[string]any
	_ = json.Unmarshal(bodyBytes, &parsed)

	data := parseRDAPJSON(parsed, domain, regDomain, queryURL)
	envelope.Data = data
	envelope.FinishedAt = time.Now()
	envelope.DurationMs = time.Since(start).Milliseconds()
	return envelope, nil
}

func parseRDAPJSON(raw map[string]any, fullDomain, regDomain, queryURL string) *RDAPData {
	res := &RDAPData{
		DomainName:       fullDomain,
		RegisteredDomain: regDomain,
		Source:           queryURL,
		Registrar:        "Unknown",
		Status:           []string{},
		Nameservers:      []string{},
	}

	if events, ok := raw["events"].([]any); ok {
		for _, e := range events {
			ev, ok := e.(map[string]any)
			if !ok {
				continue
			}
			action, _ := ev["eventAction"].(string)
			dateStr, _ := ev["eventDate"].(string)
			switch action {
			case "registration":
				res.RegistrationDate = dateStr
			case "last changed", "last update":
				res.UpdatedDate = dateStr
			case "expiration":
				res.ExpirationDate = dateStr
				if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
					days := int(time.Until(t).Hours() / 24)
					res.RemainingDays = &days
				}
			}
		}
	}

	if entities, ok := raw["entities"].([]any); ok {
		for _, ent := range entities {
			entityMap, ok := ent.(map[string]any)
			if !ok {
				continue
			}
			isRegistrar, isRegistrant := false, false
			if roles, ok := entityMap["roles"].([]any); ok {
				for _, r := range roles {
					if rStr, ok := r.(string); ok {
						switch rStr {
						case "registrar", "sponsor":
							isRegistrar = true
						case "registrant":
							isRegistrant = true
						}
					}
				}
			}
			if !isRegistrar && !isRegistrant {
				continue
			}

			// vcardArray: ["vcard", [["fn", {}, "text", "值"], ["org", {}, "text", "值"], ...]]
			fn, org := "", ""
			if vcard, ok := entityMap["vcardArray"].([]any); ok && len(vcard) > 1 {
				if items, ok := vcard[1].([]any); ok {
					for _, it := range items {
						row, ok := it.([]any)
						if !ok || len(row) < 4 {
							continue
						}
						switch row[0] {
						case "fn":
							fn = fmt.Sprintf("%v", row[3])
						case "org":
							org = fmt.Sprintf("%v", row[3])
						}
					}
				}
			}

			if isRegistrar {
				if fn != "" {
					res.Registrar = fn
				} else if handle, ok := entityMap["handle"].(string); ok && res.Registrar == "Unknown" {
					res.Registrar = handle
				}
			}
			if isRegistrant && res.Registrant == "" {
				if org != "" {
					res.Registrant = org
				} else {
					res.Registrant = fn
				}
			}
		}
	}

	if statuses, ok := raw["status"].([]any); ok {
		for _, s := range statuses {
			if sStr, ok := s.(string); ok {
				res.Status = append(res.Status, sStr)
			}
		}
	}

	if ns, ok := raw["nameservers"].([]any); ok {
		for _, n := range ns {
			if nMap, ok := n.(map[string]any); ok {
				if ldh, ok := nMap["ldhName"].(string); ok {
					res.Nameservers = append(res.Nameservers, ldh)
				}
			}
		}
	}

	return res
}
