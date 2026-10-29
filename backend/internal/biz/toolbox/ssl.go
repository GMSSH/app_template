package toolbox

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// CertInfo 是证书的关键字段。
type CertInfo struct {
	Subject           string   `json:"subject"`
	Issuer            string   `json:"issuer"`
	NotBefore         string   `json:"not_before"`
	NotAfter          string   `json:"not_after"`
	DaysLeft          int      `json:"days_left"`
	SerialNumber      string   `json:"serial_number"`
	SANs              []string `json:"sans"`
	SignatureAlg      string   `json:"signature_alg"`
	PublicKeyAlg      string   `json:"public_key_alg"`
	PublicKeyBits     int      `json:"public_key_bits"`
	FingerprintSHA256 string   `json:"fingerprint_sha256"`
	ChainSubjects     []string `json:"chain_subjects"`
}

// SSLResult 是一次证书检测的完整结果。
type SSLResult struct {
	Host         string    `json:"host"`
	Port         int       `json:"port"`
	Cert         *CertInfo `json:"cert"`
	VerifyOK     bool      `json:"verify_ok"`
	VerifyCode   int       `json:"verify_code"`
	VerifyText   string    `json:"verify_text"`
	DomainMatch  bool      `json:"domain_match"`
	MatchedBy    string    `json:"matched_by"`
	SelfSigned   bool      `json:"self_signed"`
	Expired      bool      `json:"expired"`
	ExpiringSoon bool      `json:"expiring_soon"`
	Protocol     string    `json:"protocol"`
	CipherSuite  string    `json:"cipher_suite"`
	ChainCount   int       `json:"chain_count"`
	ErrorKind    string    `json:"error_kind"`
	ErrorText    string    `json:"error_text"`
	Supported    bool      `json:"supported"`
	ElapsedMS    int64     `json:"elapsed_ms"`
}

// CheckSSL 用 openssl s_client 拉取证书链，再交给 openssl x509 解析。
// 不加 -servername 会让虚拟主机返回默认站点证书，所以必须带 SNI。
func CheckSSL(host string, port int, timeout time.Duration) (SSLResult, error) {
	out := SSLResult{Host: host, Port: port, Supported: true}
	if !Has("openssl") {
		out.Supported = false
		out.ErrorKind = "missing_tool"
		out.ErrorText = "当前服务器缺少 openssl，无法进行证书检测"
		return out, nil
	}

	start := time.Now()
	res, _ := RunCmd(timeout+5*time.Second, "openssl",
		"s_client",
		"-connect", fmt.Sprintf("%s:%d", host, port),
		"-servername", host,
		"-showcerts",
	)
	out.ElapsedMS = time.Since(start).Milliseconds()

	text := res.Stdout + "\n" + res.Stderr

	if res.TimedOut {
		out.ErrorKind = "timeout"
		out.ErrorText = fmt.Sprintf("连接 %s:%d 超时", host, port)
		return out, nil
	}
	if !strings.Contains(text, "BEGIN CERTIFICATE") {
		if msg := connectErrorText(text); msg != "" {
			out.ErrorKind = "connect_failed"
			out.ErrorText = msg
		} else {
			out.ErrorKind = "no_cert"
			out.ErrorText = "未能获取证书（目标端口可能不是 TLS 服务）"
		}
		return out, nil
	}

	out.Protocol = regexMatch(text, `Protocol\s*:\s*(\S+)`)
	out.CipherSuite = regexMatch(text, `Cipher\s*:\s*(\S+)`)

	if m := regexp.MustCompile(`Verify return code:\s*(\d+)\s*\(([^)]*)\)`).FindStringSubmatch(text); m != nil {
		out.VerifyCode, _ = strconv.Atoi(m[1])
		out.VerifyOK = out.VerifyCode == 0
		out.VerifyText = strings.TrimSpace(m[2])
	} else {
		out.VerifyOK = !strings.Contains(text, "verify error")
		out.VerifyText = "未返回验证码"
	}

	pems := extractPEMs(res.Stdout)
	out.ChainCount = len(pems)
	if len(pems) == 0 {
		out.ErrorKind = "no_cert"
		out.ErrorText = "未从响应中提取到 PEM 证书"
		return out, nil
	}

	dump, err := describeCert(pems[0], timeout)
	if err != nil || dump == "" {
		out.ErrorKind = "parse_failed"
		out.ErrorText = "无法解析证书内容"
		return out, nil
	}
	cert := parseCertDump(dump)
	if cert == nil {
		out.ErrorKind = "parse_failed"
		out.ErrorText = "无法从证书中读取主体信息"
		return out, nil
	}
	out.Cert = cert
	cert.FingerprintSHA256 = certFingerprint(pems[0], timeout)

	for _, p := range pems[1:] {
		d, err := describeCert(p, timeout)
		if err != nil {
			continue
		}
		if s := regexMatch(d, `(?m)^\s*Subject:\s*(.+)$`); s != "" {
			cert.ChainSubjects = append(cert.ChainSubjects, cnFromDN(s))
		}
	}

	out.SelfSigned = cert.Subject != "" && cert.Subject == cert.Issuer
	out.Expired = cert.DaysLeft < 0
	out.ExpiringSoon = cert.DaysLeft >= 0 && cert.DaysLeft <= 30
	out.DomainMatch, out.MatchedBy = matchDomain(host, cert)

	return out, nil
}

// describeCert 把一张 PEM 交给 openssl x509 输出可解析的文本。
func describeCert(pem string, timeout time.Duration) (string, error) {
	res, err := RunCmdIn(timeout+3*time.Second, pem, "openssl", "x509", "-noout", "-text")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(res.Stdout) == "" {
		return "", fmt.Errorf("empty x509 output")
	}
	return res.Stdout, nil
}

// certFingerprint 取证书的 SHA-256 指纹（openssl 输出的冒号十六进制形式）。
func certFingerprint(pem string, timeout time.Duration) string {
	res, err := RunCmdIn(timeout+3*time.Second, pem, "openssl", "x509", "-noout", "-fingerprint", "-sha256")
	if err != nil {
		return ""
	}
	text := strings.TrimSpace(res.Stdout)
	if i := strings.Index(text, "="); i >= 0 {
		return strings.TrimSpace(text[i+1:])
	}
	return text
}

func extractPEMs(s string) []string {
	const begin = "-----BEGIN CERTIFICATE-----"
	const end = "-----END CERTIFICATE-----"
	var out []string
	rest := s
	for {
		i := strings.Index(rest, begin)
		if i < 0 {
			break
		}
		j := strings.Index(rest[i:], end)
		if j < 0 {
			break
		}
		out = append(out, rest[i:i+j+len(end)]+"\n")
		rest = rest[i+j+len(end):]
	}
	return out
}

var (
	reXSubject = regexp.MustCompile(`(?m)^\s*Subject:\s*(.+)$`)
	reXIssuer  = regexp.MustCompile(`(?m)^\s*Issuer:\s*(.+)$`)
	reXBefore  = regexp.MustCompile(`(?m)^\s*Not Before:\s*(.+)$`)
	reXAfter   = regexp.MustCompile(`(?m)^\s*Not After\s*:\s*(.+)$`)
	reXSAN     = regexp.MustCompile(`(?s)X509v3 Subject Alternative Name:.*?\n\s*([^\n]+)`)
	reXSigAlg  = regexp.MustCompile(`(?m)^\s*Signature Algorithm:\s*(\S+)`)
	reXPubAlg  = regexp.MustCompile(`(?m)^\s*Public Key Algorithm:\s*(.+)$`)
	reXPubBits = regexp.MustCompile(`(?m)^\s*Public-Key:\s*\((\d+) bit\)`)
	reXSerial  = regexp.MustCompile(`(?m)^\s*Serial Number:\s*\n?\s*([0-9a-fA-F:]+)`)
)

func parseCertDump(d string) *CertInfo {
	c := &CertInfo{}
	if m := reXSubject.FindStringSubmatch(d); m != nil {
		c.Subject = cnFromDN(m[1])
	}
	if c.Subject == "" {
		return nil
	}
	if m := reXIssuer.FindStringSubmatch(d); m != nil {
		c.Issuer = cnFromDN(m[1])
	}
	// 时间统一规范成 2006-01-02 15:04:05，界面上不必再兼容各版本 openssl 的文本格式
	if m := reXBefore.FindStringSubmatch(d); m != nil {
		c.NotBefore = normalizeCertTime(strings.TrimSpace(m[1]))
	}
	if m := reXAfter.FindStringSubmatch(d); m != nil {
		raw := strings.TrimSpace(m[1])
		c.NotAfter = normalizeCertTime(raw)
		if t, err := parseOpenSSLTime(raw); err == nil {
			c.DaysLeft = int(time.Until(t).Hours() / 24)
		}
	}
	if m := reXSerial.FindStringSubmatch(d); m != nil {
		c.SerialNumber = strings.ReplaceAll(strings.TrimSpace(m[1]), ":", "")
	}
	if m := reXSigAlg.FindStringSubmatch(d); m != nil {
		c.SignatureAlg = m[1]
	}
	if m := reXPubAlg.FindStringSubmatch(d); m != nil {
		c.PublicKeyAlg = strings.TrimSpace(m[1])
	}
	if m := reXPubBits.FindStringSubmatch(d); m != nil {
		c.PublicKeyBits, _ = strconv.Atoi(m[1])
	}
	if m := reXSAN.FindStringSubmatch(d); m != nil {
		for _, item := range strings.Split(m[1], ",") {
			item = strings.TrimSpace(item)
			item = strings.TrimPrefix(item, "DNS:")
			item = strings.TrimPrefix(item, "IP Address:")
			if item != "" {
				c.SANs = append(c.SANs, item)
			}
		}
	}
	return c
}

// cnFromDN 从 X509 的 DN 文本里取 CN；取不到时返回第一段。
func cnFromDN(dn string) string {
	dn = strings.TrimSpace(dn)
	for _, part := range strings.Split(dn, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "CN=") || strings.HasPrefix(part, "CN =") {
			return strings.TrimSpace(strings.SplitN(part, "=", 2)[1])
		}
	}
	for _, part := range strings.Split(dn, "/") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "CN=") {
			return strings.TrimSpace(strings.SplitN(part, "=", 2)[1])
		}
	}
	return dn
}

func regexMatch(s, pattern string) string {
	m := regexp.MustCompile(pattern).FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[len(m)-1])
}

// connectErrorText 从 openssl 输出里挑出能解释失败原因的那一行。
func connectErrorText(text string) string {
	for _, line := range OutLines(text) {
		l := strings.ToLower(line)
		switch {
		case strings.Contains(l, "connection refused"):
			return "连接被拒绝：目标端口没有 TLS 服务在监听"
		case strings.Contains(l, "no route to host"):
			return "无法路由到该主机"
		case strings.Contains(l, "name or service not known"):
			return "域名解析失败"
		case strings.Contains(l, "timeout"), strings.Contains(l, "timed out"):
			return "连接超时"
		}
	}
	return ""
}

func normalizeCertTime(s string) string {
	if t, err := parseOpenSSLTime(s); err == nil {
		return t.UTC().Format("2006-01-02 15:04:05")
	}
	return s
}

func parseOpenSSLTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{
		"Jan  2 15:04:05 2006 MST",
		"Jan 2 15:04:05 2006 MST",
		"Jan  2 15:04:05 2006 GMT",
		"Jan 2 15:04:05 2006 GMT",
		time.RFC3339,
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("无法解析证书时间: %s", s)
}

// matchDomain 判断证书是否覆盖目标域名。精确匹配优先，其次通配符。
func matchDomain(host string, c *CertInfo) (bool, string) {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	candidates := append([]string{}, c.SANs...)
	if len(candidates) == 0 && c.Subject != "" {
		candidates = append(candidates, c.Subject)
	}
	for _, name := range candidates {
		n := strings.ToLower(strings.TrimSpace(name))
		if n == "" {
			continue
		}
		if n == host {
			return true, "精确匹配 " + n
		}
		if strings.HasPrefix(n, "*.") {
			suffix := n[1:]
			if strings.HasSuffix(host, suffix) {
				rest := strings.TrimSuffix(host, suffix)
				if rest != "" && !strings.Contains(rest, ".") {
					return true, "通配符匹配 " + n
				}
			}
		}
	}
	if len(candidates) == 0 {
		return false, "证书未提供任何域名信息"
	}
	return false, "证书域名列表不含 " + host
}

func firstLine(s string) string {
	for _, l := range OutLines(s) {
		return l
	}
	return ""
}
