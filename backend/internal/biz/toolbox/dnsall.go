package toolbox

import "time"

// DNSGroup 是单一记录类型的查询结果。type=ALL 时前端按组渲染，
// 每组都带自己的失败原因，某一类查不到不影响其他类型。
type DNSGroup struct {
	Type      string      `json:"type"`
	Records   []DNSRecord `json:"records"`
	ErrorKind string      `json:"error_kind"`
	ErrorText string      `json:"error_text"`
}

// DNSAllResult 是「一次查全部记录类型」的结果。
type DNSAllResult struct {
	Host      string     `json:"host"`
	Server    string     `json:"server"`
	Tool      string     `json:"tool"`
	Groups    []DNSGroup `json:"groups"`
	ElapsedMS int64      `json:"elapsed_ms"`
	Supported bool       `json:"supported"`
	ErrorKind string     `json:"error_kind"`
	ErrorText string     `json:"error_text"`
}

// allDNSTypes 是界面上「ALL」实际展开的类型集合。
var allDNSTypes = []string{"A", "AAAA", "CNAME", "MX", "TXT", "NS"}

// QueryDNSAll 按类型逐个查询后合并。查的是同一个域名、同一台服务器，
// 所以这里的耗时是真实累计值，不做任何估算。
func QueryDNSAll(host, server string) DNSAllResult {
	start := time.Now()
	out := DNSAllResult{Host: host, Server: server, Groups: make([]DNSGroup, 0, len(allDNSTypes)), Supported: true}

	for _, t := range allDNSTypes {
		res, err := QueryDNS(host, t, server)
		if err != nil {
			out.Groups = append(out.Groups, DNSGroup{Type: t, Records: []DNSRecord{}, ErrorKind: "failed", ErrorText: err.Error()})
			continue
		}
		if res.Tool != "" && out.Tool == "" {
			out.Tool = res.Tool
		}
		recs := res.Records
		if recs == nil {
			recs = []DNSRecord{}
		}
		out.Groups = append(out.Groups, DNSGroup{Type: t, Records: recs, ErrorKind: res.ErrorKind, ErrorText: res.ErrorText})
		if !res.Supported && res.ErrorKind == "missing_tool" {
			out.Supported = false
			out.ErrorKind = res.ErrorKind
			out.ErrorText = res.ErrorText
		}
	}

	// 域名不存在时每个类型都会报 NXDOMAIN，界面只需要一句结论
	for _, g := range out.Groups {
		if g.ErrorKind == "nxdomain" {
			out.ErrorKind = "nxdomain"
			out.ErrorText = "域名不存在（NXDOMAIN）"
			break
		}
	}
	out.ElapsedMS = time.Since(start).Milliseconds()
	return out
}
