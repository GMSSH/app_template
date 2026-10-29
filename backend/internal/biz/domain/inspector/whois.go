package inspector

import (
	"context"
	"io"
	"net"
	"strings"
	"time"

	"gmssh-cli-plugin/internal/biz/domain/security"
)

// .cn 等 TLD 没有公开 RDAP 服务，只能退回 WHOIS(43)；取到的仍是注册局的真实记录
var whoisServers = map[string]string{
	"cn":   "whois.cnnic.cn",
	"com":  "whois.verisign-grs.com",
	"net":  "whois.verisign-grs.com",
	"org":  "whois.pir.org",
	"io":   "whois.nic.io",
	"cc":   "ccwhois.verisign-grs.com",
	"me":   "whois.nic.me",
	"info": "whois.afilias.net",
	"top":  "whois.nic.top",
}

func whoisQuery(ctx context.Context, server, query string) (string, error) {
	dialer := security.SafeDialContext(4 * time.Second)
	conn, err := dialer(ctx, "tcp", net.JoinHostPort(server, "43"))
	if err != nil {
		return "", err
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(8 * time.Second))
	if _, err := conn.Write([]byte(query + "\r\n")); err != nil {
		return "", err
	}
	buf, err := io.ReadAll(io.LimitReader(conn, 256*1024))
	return string(buf), err
}

// 未知 TLD 先问 IANA 拿到该注册局的 WHOIS 服务器
func whoisReferral(ctx context.Context, tld string) string {
	raw, err := whoisQuery(ctx, "whois.iana.org", tld)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 6 && strings.EqualFold(trimmed[:6], "refer:") {
			return strings.TrimSpace(trimmed[6:])
		}
	}
	return ""
}

// 返回 nil 表示未能取得注册信息（调用方仍按原错误上报）
func inspectWhois(ctx context.Context, regDomain string) *RDAPData {
	parts := strings.Split(regDomain, ".")
	if len(parts) < 2 {
		return nil
	}
	tld := parts[len(parts)-1]

	server, ok := whoisServers[tld]
	if !ok {
		if server = whoisReferral(ctx, tld); server == "" {
			return nil
		}
	}

	raw, err := whoisQuery(ctx, server, regDomain)
	if err != nil {
		return nil
	}

	data := parseWhois(raw, regDomain, server)
	if data.Registrar == "" && data.ExpirationDate == "" && len(data.Nameservers) == 0 {
		return nil
	}
	return data
}

func parseWhois(raw, regDomain, server string) *RDAPData {
	res := &RDAPData{
		DomainName:       regDomain,
		RegisteredDomain: regDomain,
		Source:           "WHOIS " + server,
		Status:           []string{},
		Nameservers:      []string{},
	}

	seen := map[string]bool{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "%") || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line[:idx]))
		val := strings.TrimSpace(line[idx+1:])
		if val == "" {
			continue
		}

		// 不同注册局的字段名不同，这里做别名归一
		switch key {
		case "registrar", "sponsoring registrar":
			if res.Registrar == "" {
				res.Registrar = val
			}
		case "registrant", "registrant name", "registrant organization", "registrant organisation":
			if res.Registrant == "" {
				res.Registrant = val
			}
		case "registration time", "creation date", "registered on", "created", "domain registration date":
			if res.RegistrationDate == "" {
				res.RegistrationDate = normalizeWhoisDate(val)
			}
		case "expiration time", "registry expiry date", "expiry date", "expiration date", "paid-till":
			if res.ExpirationDate == "" {
				res.ExpirationDate = normalizeWhoisDate(val)
			}
		case "updated date", "last updated", "last updated on", "changed":
			if res.UpdatedDate == "" {
				res.UpdatedDate = normalizeWhoisDate(val)
			}
		case "name server", "nserver", "nameserver":
			ns := strings.ToLower(strings.TrimSuffix(strings.Fields(val)[0], "."))
			if !seen["ns:"+ns] {
				seen["ns:"+ns] = true
				res.Nameservers = append(res.Nameservers, ns)
			}
		case "domain status", "status":
			st := strings.Fields(val)[0]
			if !seen["st:"+st] {
				seen["st:"+st] = true
				res.Status = append(res.Status, st)
			}
		}
	}

	if res.ExpirationDate != "" {
		if t, err := time.Parse("2006-01-02 15:04:05", res.ExpirationDate); err == nil {
			days := int(time.Until(t).Hours() / 24)
			res.RemainingDays = &days
		}
	}
	return res
}

func normalizeWhoisDate(v string) string {
	v = strings.TrimSpace(v)
	for _, layout := range []string{
		"2006-01-02 15:04:05", "2006-01-02T15:04:05Z", "2006-01-02T15:04:05-07:00",
		"2006-01-02", "02-Jan-2006", "02.01.2006",
	} {
		if t, err := time.Parse(layout, v); err == nil {
			return t.Format("2006-01-02 15:04:05")
		}
	}
	return v
}
