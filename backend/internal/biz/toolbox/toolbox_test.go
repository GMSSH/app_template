package toolbox

import (
	"strings"
	"testing"
	"time"
)

// 这些用例只验证「解析真实输出」的部分，全部离线，不依赖网络。

func TestParseWhoisFieldsVerisign(t *testing.T) {
	raw := `   Domain Name: EXAMPLE.COM
   Registry Domain ID: 2336799_DOMAIN_COM-VRSN
   Registrar WHOIS Server: whois.iana.org
   Registrar URL: http://res-dom.iana.org
   Updated Date: 2025-08-14T07:01:34Z
   Creation Date: 1995-08-14T04:00:00Z
   Registry Expiry Date: 2026-08-13T04:00:00Z
   Registrar: RESERVED-Internet Assigned Numbers Authority
   Domain Status: clientDeleteProhibited https://icann.org/epp#clientDeleteProhibited
   Domain Status: clientTransferProhibited https://icann.org/epp#clientTransferProhibited
   Name Server: A.IANA-SERVERS.NET
   Name Server: B.IANA-SERVERS.NET
   DNSSEC: signedDelegation
>>> Last update of whois database: 2025-10-29T12:00:00Z <<<`

	fields := parseWhoisFields(raw)
	values := map[string][]string{}
	for _, f := range fields {
		values[f.Name] = append(values[f.Name], f.Value)
	}

	if len(values["域名"]) != 1 || values["域名"][0] != "EXAMPLE.COM" {
		t.Fatalf("域名 = %v", values["域名"])
	}
	if len(values["注册商"]) != 1 || values["注册商"][0] != "RESERVED-Internet Assigned Numbers Authority" {
		t.Fatalf("注册商 = %v", values["注册商"])
	}
	if len(values["注册时间"]) != 1 || values["注册时间"][0] != "1995-08-14T04:00:00Z" {
		t.Fatalf("注册时间 = %v", values["注册时间"])
	}
	if len(values["到期时间"]) != 1 || values["到期时间"][0] != "2026-08-13T04:00:00Z" {
		t.Fatalf("到期时间 = %v", values["到期时间"])
	}
	if len(values["域名状态"]) != 2 {
		t.Fatalf("域名状态应去重后保留 2 条，得到 %v", values["域名状态"])
	}
	if strings.Contains(values["域名状态"][0], "http") {
		t.Fatalf("域名状态应剥掉协议链接：%q", values["域名状态"][0])
	}
	if len(values["域名服务器"]) != 2 {
		t.Fatalf("域名服务器 = %v", values["域名服务器"])
	}
	if len(values["DNSSEC"]) != 1 || values["DNSSEC"][0] != "signedDelegation" {
		t.Fatalf("DNSSEC = %v", values["DNSSEC"])
	}
}

func TestParseWhoisFieldsCNNIC(t *testing.T) {
	raw := `Domain Name: EXAMPLE.CN
ROID: 20030311s10000000cn
Domain Status: clientDeleteProhibited
Registrant: 中国互联网络信息中心
Registrant Organization: 中国互联网络信息中心
Registration Time: 2003-03-17 12:13:14
Expiration Time: 2026-03-17 12:13:14
DNSSEC: unsigned
DNS Servers: ns1.example.cn
DNS Servers: ns2.example.cn`

	fields := parseWhoisFields(raw)
	got := map[string]string{}
	for _, f := range fields {
		if _, ok := got[f.Name]; !ok {
			got[f.Name] = f.Value
		}
	}
	if got["域名"] != "EXAMPLE.CN" {
		t.Fatalf("域名 = %q", got["域名"])
	}
	if got["注册时间"] != "2003-03-17 12:13:14" {
		t.Fatalf("注册时间 = %q", got["注册时间"])
	}
	if got["到期时间"] != "2026-03-17 12:13:14" {
		t.Fatalf("到期时间 = %q", got["到期时间"])
	}
	if got["注册者"] != "中国互联网络信息中心" {
		t.Fatalf("注册者 = %q", got["注册者"])
	}
	if got["域名服务器"] != "ns1.example.cn" {
		t.Fatalf("域名服务器 = %q", got["域名服务器"])
	}
}

func TestWhoisMissingAndReferral(t *testing.T) {
	if !reWhoisMissing.MatchString("No match for \"NOT-A-REAL-DOMAIN-42.COM\".") {
		t.Fatal("应识别出未注册域名")
	}
	if reWhoisMissing.MatchString("Domain Name: example.com") {
		t.Fatal("正常响应不应判定为未注册")
	}
	if got := whoisReferralText("% IANA WHOIS server\nrefer:        whois.verisign-grs.com\n"); got != "whois.verisign-grs.com" {
		t.Fatalf("referral = %q", got)
	}
	if got := whoisReferralText("Domain Name: EXAMPLE.COM"); got != "" {
		t.Fatalf("无 refer 时应返回空串，得到 %q", got)
	}
}

func TestParseResponseHeadersTakesLastBlock(t *testing.T) {
	stdout := "HTTP/1.1 301 Moved Permanently\r\nLocation: https://example.com/\r\nServer: nginx\r\n\r\n" +
		"HTTP/2 200\r\nserver: cloudflare\r\ncontent-type: text/html; charset=UTF-8\r\n\r\n" +
		"\n__WM__200|93.184.216.34||1|0.01|0.02|0.05|0.08|0.10|1256|text/html; charset=UTF-8|2"

	headers := parseResponseHeaders(stdout)
	if len(headers) != 2 {
		t.Fatalf("应只取最终响应的 2 个响应头，得到 %+v", headers)
	}
	if headers[0].Name != "server" || headers[0].Value != "cloudflare" {
		t.Fatalf("第一个响应头 = %+v", headers[0])
	}

	raw, ok := parseCurlWriteOut(stdout)
	if !ok {
		t.Fatal("计时行解析失败")
	}
	if raw.httpCode != 200 || raw.protocol != "HTTP/2" || raw.sizeDl != 1256 {
		t.Fatalf("计时字段 = %+v", raw)
	}
	if stage := computeStage(raw); stage.DNSMS != 10 || stage.TCPMS != 10 || stage.TLSMS != 30 {
		t.Fatalf("分阶段耗时 = %+v", stage)
	}
}

func TestParseDigAnswerOnlyKeepsQueriedType(t *testing.T) {
	text := `example.com.		3600	IN	MX	10 mail.example.com.
example.com.		3600	IN	A	93.184.216.34
`
	recs := parseDigAnswer(text, "MX")
	if len(recs) != 1 || recs[0].Value != "mail.example.com." || recs[0].Priority != 10 {
		t.Fatalf("MX 记录 = %+v", recs)
	}
	if recs := parseDigAnswer(text, "A"); len(recs) != 1 || recs[0].Value != "93.184.216.34" {
		t.Fatalf("A 记录 = %+v", recs)
	}
}

func TestParsePingSamplesKeepsTTLAndBytes(t *testing.T) {
	out := `PING example.com (93.184.216.34) 56(84) bytes of data.
64 bytes from 93.184.216.34: icmp_seq=1 ttl=55 time=11.2 ms
64 bytes from 93.184.216.34: icmp_seq=2 ttl=55 time=12.8 ms

--- example.com ping statistics ---
2 packets transmitted, 2 received, 0% packet loss, time 1001ms
rtt min/avg/max/mdev = 11.200/12.000/12.800/0.800 ms`

	samples := parsePingSamples(out)
	if len(samples) != 2 {
		t.Fatalf("样本数 = %d", len(samples))
	}
	if samples[0].TTL != 55 || samples[0].Bytes != 64 || samples[0].MS != 11 {
		t.Fatalf("样本 = %+v", samples[0])
	}
}

func TestNormalizeCertTime(t *testing.T) {
	if got := normalizeCertTime("Aug  2 00:00:00 2026 GMT"); got != "2026-08-02 00:00:00" {
		t.Fatalf("证书时间 = %q", got)
	}
	if got := normalizeCertTime("unknown"); got != "unknown" {
		t.Fatalf("无法解析时应原样返回，得到 %q", got)
	}
}

// TestScanCommonPortsLocal 用回环地址验证扫描结构：端口必然全关，
// 但每个端口都要有可确证的状态，不能出现空状态或重复端口。
func TestScanCommonPortsLocal(t *testing.T) {
	res := ScanCommonPorts("127.0.0.1", 500*time.Millisecond)
	if len(res.Checks) != len(defaultPorts) {
		t.Fatalf("端口数 = %d，期望 %d", len(res.Checks), len(defaultPorts))
	}
	for i, c := range res.Checks {
		if i > 0 && c.Port <= res.Checks[i-1].Port {
			t.Fatalf("端口未按升序返回：%+v", res.Checks)
		}
		switch c.State {
		case "open", "closed", "filtered", "error":
		default:
			t.Fatalf("未知状态 %q（端口 %d）", c.State, c.Port)
		}
		if c.Service == "" {
			t.Fatalf("端口 %d 缺少服务名", c.Port)
		}
	}
	if res.Open+res.Closed+res.Filtered+res.Errors != len(defaultPorts) {
		t.Fatalf("统计与实际不符：%+v", res)
	}
}

func TestQueryDNSAllShape(t *testing.T) {
	// 只验证结构：即使当前机器没有任何解析工具，也必须返回 6 组且带上失败原因。
	res := QueryDNSAll("example.com", "")
	if len(res.Groups) != len(allDNSTypes) {
		t.Fatalf("分组数 = %d", len(res.Groups))
	}
	for i, g := range res.Groups {
		if g.Type != allDNSTypes[i] {
			t.Fatalf("分组顺序错误：%v", res.Groups)
		}
		if len(g.Records) == 0 && g.ErrorText == "" && g.ErrorKind == "" {
			t.Fatalf("空结果必须带失败原因：%+v", g)
		}
	}
}

func TestParseHostOnlyRejectsJunk(t *testing.T) {
	bad := []string{"", "  ", "http://", "exa mple.com", "-bad-.", "a..b"}
	for _, s := range bad {
		if _, err := ParseHostOnly(s); err == nil {
			t.Fatalf("应拒绝 %q", s)
		}
	}
	got, err := ParseHostOnly("https://example.com/path?id=1")
	if err != nil || got != "example.com" {
		t.Fatalf("ParseHostOnly = %q, %v", got, err)
	}
}
