package toolbox

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// DNSRecord 是一条解析记录。
type DNSRecord struct {
	Type     string `json:"type"`
	Value    string `json:"value"`
	TTL      int    `json:"ttl"`
	Priority int    `json:"priority,omitempty"`
}

// DNSResult 是一次 DNS 查询的结果。
type DNSResult struct {
	Host      string      `json:"host"`
	Type      string      `json:"type"`
	Server    string      `json:"server"`
	Tool      string      `json:"tool"`
	Records   []DNSRecord `json:"records"`
	ElapsedMS int64       `json:"elapsed_ms"`
	Supported bool        `json:"supported"`
	ErrorKind string      `json:"error_kind"`
	ErrorText string      `json:"error_text"`
	Raw       string      `json:"raw"`
	// Limited 为 true 表示当前工具只能做地址解析（getent），
	// 它无法查询 MX / TXT / NS / CNAME，界面必须如实提示而不是假装成功。
	Limited bool `json:"limited"`
}

// validDNSType 限定允许的记录类型，避免用户输入进入命令行参数以外的地方。
var validDNSType = map[string]bool{
	"A": true, "AAAA": true, "CNAME": true, "MX": true, "TXT": true, "NS": true, "SOA": true,
}

// QueryDNS 按可用工具依次尝试：dig → nslookup → getent。
func QueryDNS(host, recordType, server string) (DNSResult, error) {
	recordType = strings.ToUpper(strings.TrimSpace(recordType))
	if recordType == "" {
		recordType = "A"
	}
	out := DNSResult{Host: host, Type: recordType, Server: server, Supported: true, Records: []DNSRecord{}}
	if !validDNSType[recordType] {
		out.ErrorKind = "bad_type"
		out.ErrorText = "不支持的记录类型：" + recordType
		return out, nil
	}

	start := time.Now()
	defer func() { out.ElapsedMS = time.Since(start).Milliseconds() }()

	switch {
	case Has("dig"):
		return queryWithDig(host, recordType, server, start)
	case Has("nslookup"):
		return queryWithNslookup(host, recordType, server, start)
	case Has("getent") || Has("host"):
		return queryWithGetent(host, recordType, start)
	default:
		out.Supported = false
		out.ErrorKind = "missing_tool"
		out.ErrorText = "当前服务器缺少 dig / nslookup / getent，无法进行 DNS 查询"
		return out, nil
	}
}

func queryWithDig(host, recordType, server string, start time.Time) (DNSResult, error) {
	out := DNSResult{Host: host, Type: recordType, Server: server, Tool: "dig", Supported: true, Records: []DNSRecord{}}
	// 必须带 +comments：否则拿不到 "status: NXDOMAIN"，会把不存在的域名报成空结果
	args := []string{"+time=5", "+tries=2", "+noall", "+comments", "+answer"}
	if server != "" {
		args = append(args, "@"+server)
	}
	args = append(args, host, recordType)

	res, _ := RunCmd(12*time.Second, "dig", args...)
	out.ElapsedMS = time.Since(start).Milliseconds()
	out.Raw = res.Stdout

	if res.TimedOut {
		out.ErrorKind = "timeout"
		out.ErrorText = "DNS 查询超时（服务器无响应或网络不可达）"
		return out, nil
	}

	text := res.Stdout
	if strings.Contains(text, "status: NXDOMAIN") {
		out.ErrorKind = "nxdomain"
		out.ErrorText = "域名不存在（NXDOMAIN）"
		return out, nil
	}
	if strings.Contains(text, "status: SERVFAIL") {
		out.ErrorKind = "servfail"
		out.ErrorText = "DNS 服务器返回 SERVFAIL（解析失败）"
		return out, nil
	}
	if strings.Contains(text, "status: REFUSED") {
		out.ErrorKind = "refused"
		out.ErrorText = "DNS 服务器拒绝查询（REFUSED）"
		return out, nil
	}

	out.Records = parseDigAnswer(text, recordType)
	if len(out.Records) == 0 {
		// 域名存在但没有该类型的记录（如没有 MX），与「域名不存在」是两件事
		out.ErrorKind = "empty"
		out.ErrorText = fmt.Sprintf("域名存在，但没有 %s 记录", recordType)
	}
	return out, nil
}

// parseDigAnswer 解析 dig +noall +answer 的表格行。
// 格式：name TTL IN TYPE value...
func parseDigAnswer(text, recordType string) []DNSRecord {
	var recs []DNSRecord
	for _, line := range OutLines(text) {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		ttlIdx := -1
		for i, f := range fields {
			if f == "IN" && i >= 1 {
				ttlIdx = i - 1
				break
			}
		}
		if ttlIdx < 1 || ttlIdx+2 >= len(fields) {
			continue
		}
		ttl := 0
		fmt.Sscanf(fields[ttlIdx], "%d", &ttl)
		rtype := fields[ttlIdx+2]
		// 只收查询类型的记录：NXDOMAIN 时 authority 段会带 SOA，
		// 不过滤会把「域名不存在」误判成「查到了 SOA 记录」
		if !strings.EqualFold(rtype, recordType) && !(recordType == "A" && rtype == "CNAME") {
			continue
		}
		value := strings.Join(fields[ttlIdx+3:], " ")
		rec := DNSRecord{Type: rtype, Value: strings.Trim(value, `"`), TTL: ttl}
		if rtype == "MX" {
			parts := strings.Fields(value)
			if len(parts) == 2 {
				fmt.Sscanf(parts[0], "%d", &rec.Priority)
				rec.Value = parts[1]
			}
		}
		if rec.Value == "" || rec.Value == "." {
			continue
		}
		recs = append(recs, rec)
	}
	return recs
}

func queryWithNslookup(host, recordType, server string, start time.Time) (DNSResult, error) {
	out := DNSResult{Host: host, Type: recordType, Server: server, Tool: "nslookup", Supported: true, Records: []DNSRecord{}}
	args := []string{"-type=" + recordType}
	if server != "" {
		args = append(args, host, server)
	} else {
		args = append(args, host)
	}
	res, _ := RunCmd(12*time.Second, "nslookup", args...)
	out.ElapsedMS = time.Since(start).Milliseconds()
	out.Raw = res.Stdout + "\n" + res.Stderr

	text := res.Stdout + "\n" + res.Stderr
	if res.TimedOut {
		out.ErrorKind = "timeout"
		out.ErrorText = "DNS 查询超时"
		return out, nil
	}
	if strings.Contains(text, "NXDOMAIN") || strings.Contains(text, "can't find") {
		out.ErrorKind = "nxdomain"
		out.ErrorText = "域名不存在（NXDOMAIN）"
		return out, nil
	}
	if strings.Contains(text, "SERVFAIL") {
		out.ErrorKind = "servfail"
		out.ErrorText = "DNS 服务器返回 SERVFAIL"
		return out, nil
	}

	// nslookup 的输出块里，Address: 的值需要区分「服务器地址」和「解析结果」
	var recs []DNSRecord
	var inAnswer bool
	for _, line := range OutLines(text) {
		low := strings.ToLower(line)
		if strings.Contains(low, "name:") {
			inAnswer = true
			continue
		}
		if !inAnswer {
			continue
		}
		if idx := strings.Index(low, "address:"); idx >= 0 {
			v := strings.TrimSpace(line[idx+len("address:"):])
			v = strings.TrimSuffix(v, ".")
			if v == "" {
				continue
			}
			rt := recordType
			if strings.Contains(v, ":") {
				rt = "AAAA"
				v = strings.Split(v, "#")[0]
			}
			recs = append(recs, DNSRecord{Type: rt, Value: v})
		}
		if idx := strings.Index(low, "mail exchanger ="); idx >= 0 {
			v := strings.TrimSpace(line[idx+len("mail exchanger ="):])
			recs = append(recs, DNSRecord{Type: "MX", Value: strings.TrimSuffix(v, ".")})
		}
	}
	out.Records = recs
	if len(recs) == 0 {
		out.ErrorKind = "empty"
		out.ErrorText = fmt.Sprintf("nslookup 未返回 %s 记录", recordType)
	}
	return out, nil
}

// queryWithGetent 只能做地址解析。它无法查 MX/TXT/NS/CNAME，
// 因此这些类型必须明确报「能力不足」而不是返回空结果假装成功。
func queryWithGetent(host, recordType string, start time.Time) (DNSResult, error) {
	out := DNSResult{Host: host, Type: recordType, Tool: "getent", Supported: true, Records: []DNSRecord{}}
	if recordType != "A" && recordType != "AAAA" {
		out.Limited = true
		out.Supported = false
		out.ErrorKind = "limited_tool"
		out.ErrorText = fmt.Sprintf("当前服务器只有 getent，它只能做地址解析，无法查询 %s 记录", recordType)
		return out, nil
	}

	cmdName := "getent"
	args := []string{"ahosts", host}
	if Has("host") && !Has("getent") {
		cmdName = "host"
		args = []string{"-t", recordType, host}
	}
	res, _ := RunCmd(10*time.Second, cmdName, args...)
	out.ElapsedMS = time.Since(start).Milliseconds()
	out.Raw = res.Stdout

	seen := map[string]bool{}
	for _, line := range OutLines(res.Stdout) {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		ipStr := fields[0]
		isV6 := strings.Contains(ipStr, ":")
		if (recordType == "A" && isV6) || (recordType == "AAAA" && !isV6) {
			continue
		}
		if seen[ipStr] {
			continue
		}
		seen[ipStr] = true
		out.Records = append(out.Records, DNSRecord{Type: recordType, Value: ipStr})
	}
	out.Limited = true
	if len(out.Records) == 0 {
		out.ErrorKind = "nxdomain"
		out.ErrorText = "系统解析器无法解析该主机名"
	}
	return out, nil
}

// RandomResolver 返回一个公共 DNS 服务器，仅用于「对比查询」的可选功能。
// 之所以不默认启用：不同服务器侧的递归结果本来就可能不同，
// 不能仅凭差异断定存在 DNS 污染。
func RandomResolver() string {
	pool := []string{"1.1.1.1", "8.8.8.8", "9.9.9.9", "223.5.5.5"}
	return pool[rand.Intn(len(pool))]
}
