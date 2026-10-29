package inspector

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"gmssh-cli-plugin/internal/biz/domain/security"
	"gmssh-cli-plugin/internal/biz/domain/types"
)

// Go 的 resp.Status 已是 "200 OK"，避免拼成 "200 200 OK"
func statusLine(code int, text string) string {
	if text == "" {
		return fmt.Sprintf("%d", code)
	}
	if strings.HasPrefix(text, fmt.Sprintf("%d", code)) {
		return text
	}
	return fmt.Sprintf("%d %s", code, text)
}

// RunOverview inspects all 9 modules concurrently with a total budget timeout
func RunOverview(ctx context.Context, rawDomain string) (*types.GlobalOverview, error) {
	domain, err := security.NormalizeDomain(rawDomain)
	if err != nil {
		return nil, err
	}

	overview := &types.GlobalOverview{
		Domain:       domain,
		Timestamp:    time.Now(),
		ServerRegion: "Local Host / GMSSH",
		Modules:      make(map[string]*types.ModuleEnvelope),
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	modules := []struct {
		name string
		fn   func(ctx context.Context, domain string) (*types.ModuleEnvelope, error)
	}{
		{"rdap", InspectRDAP},
		{"dns", InspectDNS},
		{"multidns", InspectMultiDNS},
		{"web", InspectWeb},
		{"ssl", InspectSSL},
		{"port", InspectPort},
		{"redirect", InspectRedirect},
		{"mail", InspectMail},
		{"dnssec", InspectDNSSEC},
	}

	// 12s overall timeout for all concurrent inspectors
	ctxTimeout, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	for _, mod := range modules {
		wg.Add(1)
		go func(mName string, inspectFn func(context.Context, string) (*types.ModuleEnvelope, error)) {
			defer wg.Done()
			env, err := inspectFn(ctxTimeout, domain)
			if err != nil && env == nil {
				env = &types.ModuleEnvelope{
					Module:     mName,
					Domain:     domain,
					Status:     types.StatusError,
					StartedAt:  time.Now(),
					FinishedAt: time.Now(),
					Error:      err.Error(),
				}
			}
			mu.Lock()
			overview.Modules[mName] = env
			mu.Unlock()
		}(mod.name, mod.fn)
	}

	wg.Wait()

	// Fill overview shortcuts from modules
	if webEnv, ok := overview.Modules["web"]; ok && webEnv != nil && webEnv.Data != nil {
		if wd, ok := webEnv.Data.(*WebData); ok {
			overview.HttpStatus = wd.StatusCode
			overview.HttpStatusText = statusLine(wd.StatusCode, wd.StatusText)
			if wd.ResolvedIP != "" {
				overview.PrimaryIP = wd.ResolvedIP
			}
		}
	}

	if overview.PrimaryIP == "" {
		if dnsEnv, ok := overview.Modules["dns"]; ok && dnsEnv != nil && dnsEnv.Data != nil {
			if dd, ok := dnsEnv.Data.(*DNSData); ok && dd.PrimaryIP != "" {
				overview.PrimaryIP = dd.PrimaryIP
			}
		}
	}

	// 到期时间优先取域名注册到期；证书到期是另一回事，取不到才退而用证书
	if rdapEnv, ok := overview.Modules["rdap"]; ok && rdapEnv != nil {
		if rd, ok := rdapEnv.Data.(*RDAPData); ok && rd.RemainingDays != nil {
			overview.ExpiryDays = rd.RemainingDays
			overview.ExpiryDate = rd.ExpirationDate
		}
	}
	if overview.ExpiryDays == nil {
		if sslEnv, ok := overview.Modules["ssl"]; ok && sslEnv != nil {
			if sd, ok := sslEnv.Data.(*SSLData); ok {
				overview.ExpiryDays = &sd.RemainingDays
				overview.ExpiryDate = sd.ValidTo
			}
		}
	}

	return overview, nil
}
