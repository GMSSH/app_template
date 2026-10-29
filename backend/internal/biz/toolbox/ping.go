package toolbox

import (
	"fmt"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// PingSample 是一次探测的结果。Loss=true 表示该次没有收到响应。
// TTL / Bytes 来自回包原文，用来让界面显示真实报文信息。
type PingSample struct {
	Seq   int    `json:"seq"`
	MS    int64  `json:"ms"`
	TTL   int    `json:"ttl,omitempty"`
	Bytes int    `json:"bytes,omitempty"`
	Loss  bool   `json:"loss"`
	Note  string `json:"note,omitempty"`
}

// PingResult 是一轮 PING 的结果。
type PingResult struct {
	Host      string       `json:"host"`
	Count     int          `json:"count"`
	Sent      int          `json:"sent"`
	Recv      int          `json:"recv"`
	LossPct   float64      `json:"loss_pct"`
	MinMS     int64        `json:"min_ms"`
	AvgMS     int64        `json:"avg_ms"`
	MaxMS     int64        `json:"max_ms"`
	Samples   []PingSample `json:"samples"`
	TimedOut  bool         `json:"timed_out"`
	Stopped   bool         `json:"stopped"`
	Supported bool         `json:"supported"`
	ErrorKind string       `json:"error_kind"`
	ErrorText string       `json:"error_text"`
	Raw       string       `json:"raw"`
	IsIPv6    bool         `json:"is_ipv6"`
}

var (
	reTimeMS  = regexp.MustCompile(`time[=<]([\d.]+)\s*ms`)
	reICMPSeq = regexp.MustCompile(`icmp_seq=(\d+)`)
	reSummary = regexp.MustCompile(`(\d+)\s+(?:packets\s+)?transmitted.*?(\d+)\s+(?:packets\s+)?received`)
	reRTT     = regexp.MustCompile(`min/avg/max(?:/mdev)?\s*=\s*([\d.]+)/([\d.]+)/([\d.]+)`)
)

// RunPing 执行固定次数的 ping。count=0 表示持续检测，此时用 timeout 作为总时长上限。
func RunPing(host string, count int, interval time.Duration, timeout time.Duration) (PingResult, error) {
	out := PingResult{Host: host, Count: count, Samples: []PingSample{}, Supported: true, IsIPv6: isV6(host)}

	if count <= 0 {
		out.Supported = false
		out.ErrorKind = "stream_unsupported"
		out.ErrorText = "当前宿主接口不支持流式输出，持续检测请在界面上分次发起"
		return out, nil
	}

	bin := "ping"
	args := []string{"-c", strconv.Itoa(count)}
	// -W 的语义按平台不同：Linux 是「单个回包的等待秒数」，BSD/macOS 是「毫秒」。
	// 用错单位会导致要么 2ms 超时全部丢包，要么把 -t 当成单包超时而提前退出。
	if runtime.GOOS == "darwin" {
		args = append(args, "-W", "2000")
	} else {
		args = append(args, "-W", "2")
	}
	if interval > 0 {
		args = append(args, "-i", fmt.Sprintf("%.1f", interval.Seconds()))
	}
	if isV6(host) && Has("ping6") {
		bin = "ping6"
	} else if isV6(host) {
		args = append([]string{"-6"}, args...)
	}
	args = append(args, host)

	if !Has(bin) {
		out.Supported = false
		out.ErrorKind = "missing_tool"
		out.ErrorText = "当前服务器缺少 ping 命令"
		return out, nil
	}

	budget := timeout
	if budget <= 0 {
		budget = time.Duration(count+2) * (time.Second + interval)
	}
	res, _ := RunCmd(budget, bin, args...)
	out.Raw = res.Stdout
	out.TimedOut = res.TimedOut

	// 解析出的样本只包含「收到响应」的探测；丢包位置按序号空缺补成 Loss 样本，
	// 这样图表上的断点位置全部来自真实序号，不做任何平滑或编造。
	got := parsePingSamples(res.Stdout)
	loss := map[int]bool{}
	for i := 0; i < count; i++ {
		loss[i] = true
	}
	for _, s := range got {
		delete(loss, s.Seq)
	}

	out.Samples = got
	for seq := 0; seq < count; seq++ {
		if loss[seq] {
			out.Samples = append(out.Samples, PingSample{Seq: seq, Loss: true})
		}
	}
	sort.Slice(out.Samples, func(i, j int) bool { return out.Samples[i].Seq < out.Samples[j].Seq })

	out.Sent = count
	out.Recv = len(got)
	var sum int64
	for _, s := range got {
		if out.MinMS == 0 || s.MS < out.MinMS {
			out.MinMS = s.MS
		}
		if s.MS > out.MaxMS {
			out.MaxMS = s.MS
		}
		sum += s.MS
	}
	if out.Recv > 0 {
		out.AvgMS = sum / int64(out.Recv)
	}
	if out.Sent > 0 {
		out.LossPct = float64(out.Sent-out.Recv) / float64(out.Sent) * 100
	}

	// 用系统自己的汇总行校正（不同发行版的 ping 输出格式有差异）
	if m := reSummary.FindStringSubmatch(res.Stdout); m != nil {
		if v, err := strconv.Atoi(m[1]); err == nil && v > 0 {
			out.Sent = v
		}
		if v, err := strconv.Atoi(m[2]); err == nil {
			out.Recv = v
			if out.Sent > 0 {
				out.LossPct = float64(out.Sent-v) / float64(out.Sent) * 100
			}
		}
	}
	if m := reRTT.FindStringSubmatch(res.Stdout); m != nil {
		out.MinMS = msFromFloat(m[1])
		out.AvgMS = msFromFloat(m[2])
		out.MaxMS = msFromFloat(m[3])
	}

	if out.Sent > 0 && out.Recv == 0 {
		out.ErrorKind = "no_reply"
		out.ErrorText = "所有探测均无响应。注意：目标可能只是禁用了 ICMP，并不代表网站无法访问"
	}
	return out, nil
}

// ttlOf / bytesOf 只从回包原文里取值，取不到就返回 0（界面隐藏）。
func ttlOf(line string) int {
	if m := reTTL.FindStringSubmatch(line); m != nil {
		v, _ := strconv.Atoi(m[1])
		return v
	}
	return 0
}

func bytesOf(line string) int {
	if m := reBytes.FindStringSubmatch(line); m != nil {
		v, _ := strconv.Atoi(m[1])
		return v
	}
	return 0
}

func msFromFloat(s string) int64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return int64(v + 0.5)
}

var (
	reEchoReply = regexp.MustCompile(`bytes from .*icmp_seq=(\d+).*time=([\d.]+)`)
	reTTL       = regexp.MustCompile(`ttl=(\d+)`)
	reBytes     = regexp.MustCompile(`(\d+) bytes from`)
)

// parsePingSamples 逐行提取「收到响应」的记录。丢包位置由序号空缺推断，
// 不凭空生成数据点。
func parsePingSamples(s string) []PingSample {
	var out []PingSample
	seen := map[int]bool{}
	for _, line := range OutLines(s) {
		m := reEchoReply.FindStringSubmatch(line)
		if m == nil {
			// BSD/macOS 格式：64 bytes from 1.1.1.1: icmp_seq=0 ttl=55 time=12.3 ms
			m2 := reICMPSeq.FindStringSubmatch(line)
			m3 := reTimeMS.FindStringSubmatch(line)
			if m2 != nil && m3 != nil && strings.Contains(line, "bytes from") {
				seq, _ := strconv.Atoi(m2[1])
				if !seen[seq] {
					seen[seq] = true
					out = append(out, PingSample{Seq: seq, MS: msFromFloat(m3[1]), TTL: ttlOf(line), Bytes: bytesOf(line)})
				}
			}
			continue
		}
		seq, _ := strconv.Atoi(m[1])
		if seen[seq] {
			continue
		}
		seen[seq] = true
		out = append(out, PingSample{Seq: seq, MS: msFromFloat(m[2]), TTL: ttlOf(line), Bytes: bytesOf(line)})
	}
	return out
}
