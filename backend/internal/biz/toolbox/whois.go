package toolbox

import (
	"fmt"
	"io"
	"net"
	"regexp"
	"strings"
	"time"
)

// WhoisField 是一条已识别的注册信息。Name 用中文标签，界面直接渲染。
type WhoisField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// WhoisResult 是一次 WHOIS 查询的结果。
type WhoisResult struct {
	Domain    string       `json:"domain"`
	Servers   []string     `json:"servers"`
	Fields    []WhoisField `json:"fields"`
	Found     bool         `json:"found"`
	Raw       string       `json:"raw"`
	ErrorKind string       `json:"error_kind"`
	ErrorText string       `json:"error_text"`
	ElapsedMS int64        `json:"elapsed_ms"`
}

// whoisMaxRaw 是保留的原始响应上限，避免把几十 KB 的注册局文本塞进 JSON 信封。
const whoisMaxRaw = 4096

// 各注册局的字段名差异很大，全部归一到这里；顺序即界面展示顺序。
var whoisAliases = map[string]string{
	"domain name":                            "域名",
	"domain":                                 "域名",
	"registrar":                              "注册商",
	"registrar name":                         "注册商",
	"sponsoring registrar":                   "注册商",
	"creation date":                          "注册时间",
	"created on":                             "注册时间",
	"created":                                "注册时间",
	"registration time":                      "注册时间",
	"registered on":                          "注册时间",
	"registry expiry date":                   "到期时间",
	"registrar registration expiration date": "到期时间",
	"expiration date":                        "到期时间",
	"expiration time":                        "到期时间",
	"expiry date":                            "到期时间",
	"updated date":                           "更新时间",
	"last updated on":                        "更新时间",
	"updated":                                "更新时间",
	"domain status":                          "域名状态",
	"status":                                 "域名状态",
	"name server":                            "域名服务器",
	"name servers":                           "域名服务器",
	"dns servers":                            "域名服务器",
	"nserver":                                "域名服务器",
	"dnssec":                                 "DNSSEC",
	"registrant organization":                "注册者",
	"registrant country":                     "注册国家",
}

var whoisFieldOrder = []string{"域名", "注册商", "注册时间", "到期时间", "更新时间", "域名状态", "域名服务器", "DNSSEC", "注册者", "注册国家"}

var (
	reWhoisRefer   = regexp.MustCompile("(?mi)^[ \t]*(?:refer|whois|registrar whois server)[ \t]*:[ \t]*(\\S+)[ \t]*$")
	reWhoisMissing = regexp.MustCompile(`(?i)no match for|not found|no entries found|domain not found|no data found|not registered`)
	reWhoisLimited = regexp.MustCompile(`(?i)rate limit|too many requests|quota exceeded|exceeded the query limit`)
)

// QueryWhois 走真实的 WHOIS 协议（TCP 43）：
// 先问 IANA 拿到该 TLD 的注册局服务器，再问注册局拿到注册信息。
// 全程只读，不依赖系统 whois 命令。
// 命名返回值让 defer 能把真实耗时写回结果（否则返回的是 defer 之前的副本）。
func QueryWhois(domain string, timeout time.Duration) (out WhoisResult) {
	start := time.Now()
	out = WhoisResult{Domain: domain, Fields: []WhoisField{}, Servers: []string{}}
	defer func() { out.ElapsedMS = time.Since(start).Milliseconds() }()

	label := firstLabel(domain)
	if label == "" || label == domain {
		out.ErrorKind = "bad_target"
		out.ErrorText = "WHOIS 查询需要完整的域名（例如 example.com）"
		return out
	}

	server := "whois.iana.org"
	query := label
	if s := whoisReferral(server, query, timeout); s != "" {
		server = s
	}
	query = domain

	for hop := 0; hop < 3; hop++ {
		out.Servers = append(out.Servers, server)
		text, err := whoisQuery(server, query, timeout)
		if err != nil {
			out.ErrorKind, out.ErrorText = whoisNetError(err, server)
			return out
		}
		out.Raw = truncate(strings.TrimSpace(text), whoisMaxRaw)

		if reWhoisLimited.MatchString(text) {
			out.ErrorKind = "rate_limited"
			out.ErrorText = fmt.Sprintf("注册局 %s 限流，请稍后再查", server)
			return out
		}

		fields := parseWhoisFields(text)
		if reWhoisMissing.MatchString(text) && len(fields) == 0 {
			out.ErrorKind = "not_found"
			out.ErrorText = "该域名未注册，或注册局无对应记录"
			return out
		}
		if len(fields) > 0 {
			out.Fields = fields
			out.Found = true
			return out
		}

		// 精简注册局只返回一个转向地址，继续问下一台
		next := whoisReferralText(text)
		if next == "" || next == server {
			break
		}
		server = next
	}

	out.ErrorKind = "no_fields"
	out.ErrorText = "已拿到注册局响应，但未识别出注册信息字段（该注册局格式不常见）"
	return out
}

// whoisQuery 是裸 TCP 43 查询，读写都设超时，响应最多读 128KB。
func whoisQuery(server, query string, timeout time.Duration) (string, error) {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(server, "43"), timeout)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return "", err
	}
	if _, err := fmt.Fprintf(conn, "%s\r\n", query); err != nil {
		return "", err
	}
	buf, err := io.ReadAll(io.LimitReader(conn, 128*1024))
	if err != nil && len(buf) == 0 {
		return "", err
	}
	return strings.ReplaceAll(string(buf), "\r\n", "\n"), nil
}

// whoisReferral 查 IANA 得到 TLD 的注册局服务器；失败时返回空串，由调用方决定是否继续。
func whoisReferral(server, tld string, timeout time.Duration) string {
	text, err := whoisQuery(server, tld, timeout)
	if err != nil {
		return ""
	}
	return whoisReferralText(text)
}

func whoisReferralText(text string) string {
	m := reWhoisRefer.FindStringSubmatch(text)
	if m == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(m[1]))
}

// parseWhoisFields 按「键: 值」抽取已识别的字段，多值字段（状态、NS）逐条保留。
func parseWhoisFields(text string) []WhoisField {
	type bucket struct{ values []string }
	got := map[string]*bucket{}
	seen := map[string]bool{}

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "%") ||
			strings.HasPrefix(line, ">") || strings.HasPrefix(line, "NOTICE") {
			continue
		}
		i := strings.Index(line, ":")
		if i <= 0 {
			continue
		}
		name, ok := whoisAliases[normalizeKey(line[:i])]
		if !ok {
			continue
		}
		value := strings.TrimSpace(line[i+1:])
		if j := strings.Index(value, " http"); j > 0 {
			value = strings.TrimSpace(value[:j])
		}
		value = truncate(strings.Join(strings.Fields(value), " "), 200)
		if value == "" || seen[name+"|"+value] {
			continue
		}
		seen[name+"|"+value] = true
		if got[name] == nil {
			got[name] = &bucket{}
		}
		got[name].values = append(got[name].values, value)
	}

	out := make([]WhoisField, 0, len(got))
	for _, name := range whoisFieldOrder {
		b := got[name]
		if b == nil {
			continue
		}
		for _, v := range b.values {
			out = append(out, WhoisField{Name: name, Value: v})
			if len(out) >= 40 {
				return out
			}
		}
	}
	return out
}

func normalizeKey(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
}

func firstLabel(domain string) string {
	domain = strings.TrimPrefix(strings.ToLower(domain), "www.")
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return ""
	}
	return parts[len(parts)-1]
}

func whoisNetError(err error, server string) (string, string) {
	low := strings.ToLower(err.Error())
	switch {
	case strings.Contains(low, "i/o timeout"), strings.Contains(low, "timed out"),
		strings.Contains(low, "deadline exceeded"):
		return "timeout", fmt.Sprintf("连接 %s:43 超时（部分网络会封锁 WHOIS 端口）", server)
	case strings.Contains(low, "connection refused"):
		return "refused", fmt.Sprintf("注册局 %s 拒绝连接", server)
	case strings.Contains(low, "no such host"):
		return "dns", fmt.Sprintf("无法解析 WHOIS 服务器 %s", server)
	default:
		return "failed", fmt.Sprintf("WHOIS 连接失败：%s", err.Error())
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
