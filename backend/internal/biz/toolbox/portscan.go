package toolbox

import (
	"fmt"
	"net"
	"sort"
	"sync"
	"time"
)

// 检测的是站长最常关注的端口，与界面表格一一对应。
var defaultPorts = []int{21, 22, 80, 443, 3306, 6379, 8080}

var portService = map[int]string{
	21: "FTP", 22: "SSH", 80: "HTTP", 443: "HTTPS",
	3306: "MySQL", 6379: "Redis", 8080: "HTTP-Alt",
}

// PortCheck 是单个端口的探测结果。
// State 只有三种可确证的取值：
//
//	open     — 三次握手成功
//	closed   — 目标明确返回 RST（端口没有服务在监听）
//	filtered — 超时无响应（可能被防火墙丢包，也可能主机不可达）
//
// 除此之外一律是 error，并带上原始原因，不猜。
type PortCheck struct {
	Port      int    `json:"port"`
	Service   string `json:"service"`
	State     string `json:"state"`
	ElapsedMS int64  `json:"elapsed_ms"`
	ErrorKind string `json:"error_kind"`
	ErrorText string `json:"error_text"`
}

// PortScanResult 是一轮端口扫描的结果。
type PortScanResult struct {
	Host      string      `json:"host"`
	Checks    []PortCheck `json:"checks"`
	Open      int         `json:"open"`
	Closed    int         `json:"closed"`
	Filtered  int         `json:"filtered"`
	Errors    int         `json:"errors"`
	ElapsedMS int64       `json:"elapsed_ms"`
	TimeoutMS int64       `json:"timeout_ms"`
}

// ScanCommonPorts 并发探测固定端口列表，总耗时为最慢的那个端口。
func ScanCommonPorts(host string, timeout time.Duration) PortScanResult {
	start := time.Now()
	out := PortScanResult{Host: host, Checks: make([]PortCheck, 0, len(defaultPorts)), TimeoutMS: timeout.Milliseconds()}

	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, port := range defaultPorts {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			check := PortCheck{Port: p, Service: portService[p]}
			if check.Service == "" {
				check.Service = "TCP"
			}

			dialStart := time.Now()
			conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, fmt.Sprintf("%d", p)), timeout)
			check.ElapsedMS = time.Since(dialStart).Milliseconds()
			if err != nil {
				out2 := TCPResult{}
				classifyDialError(err, &out2)
				check.ErrorKind = out2.ErrorKind
				check.ErrorText = out2.ErrorText
				switch out2.ErrorKind {
				case "refused":
					check.State = "closed"
				case "timeout":
					check.State = "filtered"
				default:
					check.State = "error"
				}
			} else {
				conn.Close()
				check.State = "open"
			}

			mu.Lock()
			out.Checks = append(out.Checks, check)
			mu.Unlock()
		}(port)
	}
	wg.Wait()

	sort.Slice(out.Checks, func(i, j int) bool { return out.Checks[i].Port < out.Checks[j].Port })
	for _, c := range out.Checks {
		switch c.State {
		case "open":
			out.Open++
		case "closed":
			out.Closed++
		case "filtered":
			out.Filtered++
		default:
			out.Errors++
		}
	}
	out.ElapsedMS = time.Since(start).Milliseconds()
	return out
}
