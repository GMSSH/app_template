package footprint

import (
	"testing"
	"time"
)

func TestParseSyslogLine(t *testing.T) {
	now := time.Date(2026, 10, 23, 22, 0, 0, 0, time.Local)
	loc := now.Location()

	tests := []struct {
		name       string
		line       string
		wantIP     string
		wantUser   string
		wantResult AuthResult
	}{
		{
			name:       "Failed password with invalid user",
			line:       "Oct 23 20:15:02 server sshd[12345]: Failed password for invalid user admin from 192.168.1.100 port 45212 ssh2",
			wantIP:     "192.168.1.100",
			wantUser:   "admin",
			wantResult: ResultFailed,
		},
		{
			name:       "Failed password for valid user",
			line:       "Oct 23 20:16:10 server sshd[12346]: Failed password for root from 10.0.0.5 port 55123 ssh2",
			wantIP:     "10.0.0.5",
			wantUser:   "root",
			wantResult: ResultFailed,
		},
		{
			name:       "Accepted publickey",
			line:       "Oct 23 20:18:22 server sshd[12350]: Accepted publickey for deploy from 192.168.1.50 port 51122 ssh2: RSA SHA256:abc",
			wantIP:     "192.168.1.50",
			wantUser:   "deploy",
			wantResult: ResultSuccess,
		},
		{
			name:       "Accepted password",
			line:       "Oct 23 20:20:00 server sshd[12355]: Accepted password for ubuntu from 192.168.1.55 port 51124 ssh2",
			wantIP:     "192.168.1.55",
			wantUser:   "ubuntu",
			wantResult: ResultSuccess,
		},
		{
			name:       "ISO time format",
			line:       "2026-10-23T20:22:15.123+08:00 myhost sshd[9999]: Failed password for test from 203.0.113.10 port 33333 ssh2",
			wantIP:     "203.0.113.10",
			wantUser:   "test",
			wantResult: ResultFailed,
		},
		{
			// journalctl 会把非 ASCII 字节转成八进制，必须还原成中文
			name:       "journalctl octal escaped utf8 user",
			line:       "2026-10-23T20:24:00+08:00 h sshd[1]: Failed password for \346\265\213\350\257\225 from 198.51.100.7 port 2222 ssh2",
			wantIP:     "198.51.100.7",
			wantUser:   "测试",
			wantResult: ResultFailed,
		},
		{
			name:       "control chars in user are stripped",
			line:       "Oct 23 20:26:00 s sshd[2]: Failed password for bad\x01user from 198.51.100.8 port 2223 ssh2",
			wantIP:     "198.51.100.8",
			wantUser:   "baduser",
			wantResult: ResultFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := parseLine(tt.line, now, loc)
			if ev == nil {
				t.Fatalf("expected non-nil event for %s", tt.line)
			}
			if ev.IP != tt.wantIP {
				t.Errorf("got IP %s, want %s", ev.IP, tt.wantIP)
			}
			if ev.User != tt.wantUser {
				t.Errorf("got User %s, want %s", ev.User, tt.wantUser)
			}
			if ev.Result != tt.wantResult {
				t.Errorf("got Result %s, want %s", ev.Result, tt.wantResult)
			}
		})
	}
}

// 审核方会用「逐 IP 求和 == 汇总」来核对口径，这里把它固化成断言
func TestSummaryMatchesIPList(t *testing.T) {
	now := time.Now()
	events := []*RawEvent{
		{Timestamp: now.Add(-30 * time.Minute).UnixMilli(), IP: "9.9.9.9", User: "root", Result: ResultFailed},
		{Timestamp: now.Add(-29 * time.Minute).UnixMilli(), IP: "9.9.9.9", User: "admin", Result: ResultFailed},
		{Timestamp: now.Add(-28 * time.Minute).UnixMilli(), IP: "9.9.9.9", User: "guest", Result: ResultFailed},
		{Timestamp: now.Add(-28 * time.Minute).UnixMilli(), IP: "9.9.9.9", User: "root", Result: ResultSuccess},
		{Timestamp: now.Add(-20 * time.Minute).UnixMilli(), IP: "8.8.8.8", User: "ubuntu", Result: ResultSuccess},
	}

	list := analyzeSuspiciousIPs(events)

	var failed, success int
	for _, ip := range list {
		failed += ip.FailedCount
		success += ip.SuccessCount
	}

	if failed != 3 {
		t.Errorf("sum(failed_count)=%d, want 3", failed)
	}
	if success != 2 {
		t.Errorf("sum(success_count)=%d, want 2", success)
	}
	if len(list) != 2 {
		t.Errorf("unique_ips=%d, want 2", len(list))
	}
}

func TestAnalyzeSuspiciousIPs(t *testing.T) {
	now := time.Now()
	events := []*RawEvent{
		// IP 1: 猜账号里的真实账号成功（Invalid user 探测后登入）→ 高危
		{Timestamp: now.Add(-10 * time.Minute).UnixMilli(), IP: "1.1.1.1", User: "admin", Result: ResultFailed, IsInvalid: true, TimeStr: "10:00:00", Raw: "line1"},
		{Timestamp: now.Add(-9 * time.Minute).UnixMilli(), IP: "1.1.1.1", User: "root", Result: ResultFailed, IsInvalid: true, TimeStr: "10:01:00", Raw: "line2"},
		{Timestamp: now.Add(-8 * time.Minute).UnixMilli(), IP: "1.1.1.1", User: "root", Result: ResultSuccess, TimeStr: "10:02:00", Raw: "line3"},
		// IP 2: 多账号尝试
		{Timestamp: now.Add(-5 * time.Minute).UnixMilli(), IP: "2.2.2.2", User: "root", Result: ResultFailed, TimeStr: "10:05:00", Raw: "line4"},
		{Timestamp: now.Add(-4 * time.Minute).UnixMilli(), IP: "2.2.2.2", User: "admin", Result: ResultFailed, TimeStr: "10:06:00", Raw: "line5"},
		{Timestamp: now.Add(-3 * time.Minute).UnixMilli(), IP: "2.2.2.2", User: "guest", Result: ResultFailed, TimeStr: "10:07:00", Raw: "line6"},
		// IP 3: 本人同一账号输错两次再登入 → 低危，不冤枉自己
		{Timestamp: now.Add(-2 * time.Minute).UnixMilli(), IP: "3.3.3.3", User: "ubuntu", Result: ResultFailed, TimeStr: "10:08:00", Raw: "line7"},
		{Timestamp: now.Add(-1 * time.Minute).UnixMilli(), IP: "3.3.3.3", User: "ubuntu", Result: ResultFailed, TimeStr: "10:09:00", Raw: "line8"},
		{Timestamp: now.UnixMilli(), IP: "3.3.3.3", User: "ubuntu", Result: ResultSuccess, TimeStr: "10:10:00", Raw: "line9"},
	}

	list := analyzeSuspiciousIPs(events)
	if len(list) != 3 {
		t.Fatalf("expected 3 IPs, got %d", len(list))
	}

	// 检查 IP 1 是否触发"值得核对的成功登录"
	var ip1 *SuspiciousIP
	var ip2 *SuspiciousIP
	for i := range list {
		if list[i].IP == "1.1.1.1" {
			ip1 = &list[i]
		}
		if list[i].IP == "2.2.2.2" {
			ip2 = &list[i]
		}
	}

	if ip1 == nil || ip1.RiskLevel != "high" {
		t.Errorf("expected 1.1.1.1 to be high risk, got %+v", ip1)
	}
	if ip2 == nil || ip2.RiskLevel != "medium" {
		t.Errorf("expected 2.2.2.2 to be medium risk, got %+v", ip2)
	}

	// 同一账号重试后退出的 IP 不应被标成"值得核对的成功登录"
	for _, item := range list {
		if item.IP != "3.3.3.3" {
			continue
		}
		if item.RiskLevel != "low" {
			t.Errorf("expected 3.3.3.3 to be low risk, got %s", item.RiskLevel)
		}
		for _, tag := range item.RiskTags {
			if tag == "值得核对的成功登录" {
				t.Errorf("3.3.3.3 是本人重试，不应被标记为值得核对")
			}
		}
	}
}
