package footprint

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"gmssh-cli-plugin/internal/input"
	"gmssh-cli-plugin/internal/logging"
	"gmssh-cli-plugin/internal/middleware"
)

// Run is the command handler for ssh_footprint
func Run(ctx *middleware.Context, in input.Input) (any, error) {
	action := in.Str("action")
	if action == "" {
		action = "probe"
	}

	logging.Business().Infof("ssh_footprint action=%s range=%s", action, in.Str("range"))

	switch action {
	case "probe":
		return ProbeLogs()
	case "scan":
		rangeStr := in.Str("range")
		if rangeStr == "" {
			rangeStr = "7d"
		}
		return ScanLogs(rangeStr)
	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}
}

// candidateLogFiles 定义可能存在 SSH 认证日志的常见系统路径
var candidateLogFiles = []string{
	"/var/log/auth.log",
	"/var/log/auth.log.1",
	"/var/log/secure",
	"/var/log/secure-1",
	"/var/log/messages",
}

// ProbeLogs 探测日志源与读取权限
func ProbeLogs() (*ProbeResult, error) {
	res := &ProbeResult{
		Sources:       make([]string, 0),
		ReadableFiles: make([]string, 0),
	}

	// 获取本地时区信息
	now := time.Now()
	zName, zOffset := now.Zone()
	res.Timezone = zName
	hours := zOffset / 3600
	minutes := (zOffset % 3600) / 60
	res.TimezoneDesc = fmt.Sprintf("%s (UTC%+03d:%02d)", zName, hours, minutes)

	// 1. 检查 journalctl 是否可用且可读
	if _, err := exec.LookPath("journalctl"); err == nil {
		testCmd := exec.Command("journalctl", "-u", "ssh", "-u", "sshd", "-n", "1", "--no-pager")
		if err := testCmd.Run(); err == nil {
			res.HasJournal = true
			res.Sources = append(res.Sources, "systemd-journal")
		}
	}

	// 2. 检查静态日志文件
	for _, fpath := range candidateLogFiles {
		if fi, err := os.Stat(fpath); err == nil && !fi.IsDir() {
			f, oErr := os.Open(fpath)
			if oErr == nil {
				_ = f.Close()
				res.ReadableFiles = append(res.ReadableFiles, fpath)
				res.Sources = append(res.Sources, fpath)
			}
		}
	}

	// 判断总体可用性
	if res.HasJournal || len(res.ReadableFiles) > 0 {
		res.Available = true
		res.Permission = true
		res.Message = "日志源就绪"
	} else {
		res.Available = false
		res.Permission = false
		res.Message = "未发现可读的 SSH 认证日志，可能受到系统权限限制或未开启 SSH 审计"
	}

	return res, nil
}

// ScanLogs 扫描指定时间范围内的日志并分析
func ScanLogs(rangeStr string) (*ScanResult, error) {
	now := time.Now()
	loc := now.Location()
	zName, _ := now.Zone()

	var duration time.Duration
	switch rangeStr {
	case "24h":
		duration = 24 * time.Hour
	case "30d":
		duration = 30 * 24 * time.Hour
	case "7d":
		fallthrough
	default:
		rangeStr = "7d"
		duration = 7 * 24 * time.Hour
	}

	cutoffTime := now.Add(-duration)
	cutoffTS := cutoffTime.UnixMilli()

	// 用绝对时间而不是 "24 hours ago"：让 journalctl 的下界与本地过滤下界完全一致，
	// 否则两边取 now 的时刻不同，窗口边界附近的事件会只出现在一侧。
	journalSince := cutoffTime.Format("2006-01-02 15:04:05")

	res := &ScanResult{
		Range:         rangeStr,
		ScanTime:      now.Format("2006-01-02 15:04:05"),
		Timezone:      zName,
		WindowStart:   journalSince,
		WindowEnd:     now.Format("2006-01-02 15:04:05"),
		WindowStartTS: cutoffTS,
		WindowEndTS:   now.UnixMilli(),
		CountMethod:   CountMethod,
		IPList:        make([]SuspiciousIP, 0),
		Buckets:       make([]TimeBucket, 0),
	}

	// 1. 读取原始日志行
	var rawLines []string
	activeSrc := ""

	// 优先 Journalctl
	if lines, err := readJournalLogs(journalSince, 20000); err == nil && len(lines) > 0 {
		rawLines = lines
		activeSrc = "systemd-journal"
	} else {
		// 遍历文件读取
		for _, fpath := range candidateLogFiles {
			if fLines, fErr := readFileLogs(fpath, 15000); fErr == nil && len(fLines) > 0 {
				rawLines = append(rawLines, fLines...)
				if activeSrc == "" {
					activeSrc = fpath
				} else {
					activeSrc += ", " + fpath
				}
			}
		}
	}

	res.ActiveSource = activeSrc
	res.TotalLogs = len(rawLines)

	if len(rawLines) == 0 {
		res.EmptyReason = "在可读取的日志范围内，暂未发现 SSH 认证记录。"
		res.Buckets = buildEmptyBuckets(rangeStr, cutoffTime, now)
		return res, nil
	}

	// 2. 解析事件并精确去重
	// 约束：Invalid user 与 Failed password 可能描述同一次认证过程，不能重复计数；
	// 同一 IP、同一秒、同一用户名的失败只保留一次。
	events := make([]*RawEvent, 0, len(rawLines))
	seenFailed := make(map[string]bool)
	seenSuccess := make(map[string]bool)

	for _, line := range rawLines {
		ev := parseLine(line, now, loc)
		if ev == nil {
			continue
		}
		if ev.Timestamp < cutoffTS {
			continue
		}

		// 去重指纹：秒级时间戳 + IP + 用户名
		sec := ev.Timestamp / 1000
		key := fmt.Sprintf("%d|%s|%s", sec, ev.IP, ev.User)

		if ev.Result == ResultFailed {
			if seenFailed[key] {
				continue
			}
			seenFailed[key] = true
		} else if ev.Result == ResultSuccess {
			if seenSuccess[key] {
				continue
			}
			seenSuccess[key] = true
		}

		events = append(events, ev)
	}

	if len(events) == 0 {
		res.EmptyReason = "在可读取的日志范围内，暂未发现该时间范围内的 SSH 认证记录。"
		res.Buckets = buildEmptyBuckets(rangeStr, cutoffTime, now)
		return res, nil
	}

	// 3. 构建时间桶柱状图数据
	res.Buckets = aggregateBuckets(rangeStr, cutoffTime, now, events)

	// 4. 按 IP 聚合统计并匹配异常行为规则
	res.IPList = analyzeSuspiciousIPs(events)

	// 5. 汇总统计指标
	totalFailed := 0
	totalSuccess := 0
	for _, ev := range events {
		if ev.Result == ResultFailed {
			totalFailed++
		} else if ev.Result == ResultSuccess {
			totalSuccess++
		}
	}
	res.Summary.TotalFailed = totalFailed
	res.Summary.TotalSuccess = totalSuccess
	res.Summary.UniqueIPs = len(res.IPList)

	return res, nil
}

// head 取前 n 个用于文案展示，避免一次爆破几十个用户名把界面撑爆
func head(items []string, n int) []string {
	if len(items) <= n {
		return items
	}
	out := append([]string{}, items[:n]...)
	out = append(out, "…")
	return out
}

// buildEmptyBuckets 当没有事件时生成空柱子结构
func buildEmptyBuckets(rangeStr string, start, end time.Time) []TimeBucket {
	return aggregateBuckets(rangeStr, start, end, nil)
}

// aggregateBuckets 聚合柱状图数据
// 24小时：每小时1组，共24组
// 7天：每6小时1组，共28组
// 30天：每天1组，共30组
func aggregateBuckets(rangeStr string, start, end time.Time, events []*RawEvent) []TimeBucket {
	var count int
	var step time.Duration

	switch rangeStr {
	case "24h":
		count = 24
		step = time.Hour
	case "30d":
		count = 30
		step = 24 * time.Hour
	case "7d":
		fallthrough
	default:
		count = 28
		step = 6 * time.Hour
	}

	buckets := make([]TimeBucket, count)
	curStart := end.Add(-time.Duration(count) * step)

	for i := 0; i < count; i++ {
		bStart := curStart.Add(time.Duration(i) * step)
		bEnd := bStart.Add(step)

		label := ""
		switch rangeStr {
		case "24h":
			label = bStart.Format("15:00")
		case "30d":
			label = bStart.Format("01-02")
		case "7d":
			label = bStart.Format("01-02 15h")
		}

		buckets[i] = TimeBucket{
			Label:     label,
			StartTime: bStart.UnixMilli(),
			EndTime:   bEnd.UnixMilli(),
		}
	}

	// 分配事件到桶
	for _, ev := range events {
		for i := 0; i < count; i++ {
			if ev.Timestamp >= buckets[i].StartTime && ev.Timestamp < buckets[i].EndTime {
				if ev.Result == ResultFailed {
					buckets[i].FailedCount++
				} else if ev.Result == ResultSuccess {
					buckets[i].SuccessCount++
				}
				break
			}
		}
	}

	return buckets
}

// analyzeSuspiciousIPs 按 IP 聚合统计并根据轻量规则标注异常
func analyzeSuspiciousIPs(events []*RawEvent) []SuspiciousIP {
	type ipTracker struct {
		ip           string
		failedCount  int
		successCount int
		usersMap     map[string]bool
		firstTS      int64
		lastTS       int64
		firstStr     string
		lastStr      string
		events       []*RawEvent
	}

	trackers := make(map[string]*ipTracker)

	for _, ev := range events {
		tr, ok := trackers[ev.IP]
		if !ok {
			tr = &ipTracker{
				ip:       ev.IP,
				usersMap: make(map[string]bool),
				firstTS:  ev.Timestamp,
				lastTS:   ev.Timestamp,
				firstStr: ev.TimeStr,
				lastStr:  ev.TimeStr,
				events:   make([]*RawEvent, 0),
			}
			trackers[ev.IP] = tr
		}

		if ev.User != "" {
			tr.usersMap[ev.User] = true
		}
		if ev.Timestamp < tr.firstTS {
			tr.firstTS = ev.Timestamp
			tr.firstStr = ev.TimeStr
		}
		if ev.Timestamp > tr.lastTS {
			tr.lastTS = ev.Timestamp
			tr.lastStr = ev.TimeStr
		}

		if ev.Result == ResultFailed {
			tr.failedCount++
		} else if ev.Result == ResultSuccess {
			tr.successCount++
		}

		tr.events = append(tr.events, ev)
	}

	list := make([]SuspiciousIP, 0, len(trackers))

	for _, tr := range trackers {
		// 用户列表转数组
		users := make([]string, 0, len(tr.usersMap))
		for u := range tr.usersMap {
			users = append(users, u)
		}
		sort.Strings(users)

		// 按时间排序内部事件
		sort.Slice(tr.events, func(i, j int) bool {
			return tr.events[i].Timestamp < tr.events[j].Timestamp
		})

		// 提取原始日志预览（最多 30 条）
		logSamples := make([]string, 0, 30)
		for i, e := range tr.events {
			if i >= 30 {
				break
			}
			logSamples = append(logSamples, e.Raw)
		}

		// 规则分析
		tags := make([]string, 0)
		reasons := make([]string, 0)
		riskLevel := "info"

		// 规则 1：高频认证失败（5 分钟内失败达到 20 次，或总体失败超 50 次）
		hasBurst := false
		failedEvs := make([]*RawEvent, 0)
		for _, e := range tr.events {
			if e.Result == ResultFailed {
				failedEvs = append(failedEvs, e)
			}
		}
		if len(failedEvs) >= 20 {
			for i := 0; i < len(failedEvs)-19; i++ {
				if failedEvs[i+19].Timestamp-failedEvs[i].Timestamp <= 5*60*1000 {
					hasBurst = true
					break
				}
			}
		}
		if hasBurst {
			tags = append(tags, "高频登录尝试")
			reasons = append(reasons, "5分钟内连续认证失败超过20次")
			riskLevel = "high"
		} else if tr.failedCount >= 50 {
			tags = append(tags, "反复认证失败")
			reasons = append(reasons, fmt.Sprintf("累计认证失败达 %d 次", tr.failedCount))
			if riskLevel != "high" {
				riskLevel = "medium"
			}
		}

		// 规则 2：尝试多个账号（尝试不同账号 >= 3 个）
		if len(users) >= 3 {
			tags = append(tags, "多账号尝试")
			reasons = append(reasons, fmt.Sprintf("尝试了 %d 个不同用户名 (%s)", len(users), strings.Join(head(users, 5), ", ")))
			if riskLevel != "high" {
				riskLevel = "medium"
			}
		}

		// 规则 3：失败后成功（连续失败至少 2 次后出现成功认证）
		// 本人输错几次密码再登进去也是这个形态，所以要看失败的"性质"：
		// 只有试过不存在的账号（Invalid user）或换过 3 个以上账号再登进去，
		// 才是猜口令成功的线索；同一账号反复重试只算正常重试。
		failedBeforeSuccess := false
		normalRetry := false
		currentFails := 0
		runInvalid := false
		runUsers := make(map[string]bool)
		for _, e := range tr.events {
			switch e.Result {
			case ResultFailed:
				currentFails++
				if e.IsInvalid {
					runInvalid = true
				}
				if e.User != "" {
					runUsers[e.User] = true
				}
			case ResultSuccess:
				if currentFails >= 2 {
					if runInvalid || len(runUsers) >= 3 {
						failedBeforeSuccess = true
					} else {
						normalRetry = true
					}
					break
				}
				currentFails = 0
				runInvalid = false
				runUsers = make(map[string]bool)
			}
		}
		if failedBeforeSuccess {
			tags = append(tags, "值得核对的成功登录")
			reasons = append(reasons, "在该 IP 连续认证失败（含不存在的账号或多次更换账号）后，检测到了成功的 SSH 登录")
			riskLevel = "high"
		}
		if normalRetry {
			tags = append(tags, "重试后登录成功")
			reasons = append(reasons, fmt.Sprintf("同一账号连续输错 %d 次后登录成功，符合本人重试特征", currentFails))
			if riskLevel != "high" {
				riskLevel = "low"
			}
		}

		// 默认风险分级
		if len(tags) == 0 {
			if tr.failedCount > 0 && tr.successCount == 0 {
				tags = append(tags, "认证失败")
				reasons = append(reasons, fmt.Sprintf("存在 %d 次偶发认证失败", tr.failedCount))
				riskLevel = "low"
			} else if tr.successCount > 0 && tr.failedCount == 0 {
				tags = append(tags, "正常登录")
				reasons = append(reasons, fmt.Sprintf("成功认证 %d 次，无失败记录", tr.successCount))
				riskLevel = "info"
			} else {
				tags = append(tags, "偶发失败")
				reasons = append(reasons, fmt.Sprintf("成功 %d 次，失败 %d 次", tr.successCount, tr.failedCount))
				riskLevel = "low"
			}
		}

		item := SuspiciousIP{
			IP:           tr.ip,
			FailedCount:  tr.failedCount,
			SuccessCount: tr.successCount,
			Users:        users,
			FirstSeen:    tr.firstStr,
			LastSeen:     tr.lastStr,
			FirstSeenTS:  tr.firstTS,
			LastSeenTS:   tr.lastTS,
			RiskLevel:    riskLevel,
			RiskTags:     tags,
			RiskReasons:  reasons,
			Logs:         logSamples,
		}
		list = append(list, item)
	}

	// 排序规则：
	// 1. high > medium > low > info
	// 2. 失败次数降序
	// 3. 末次活动时间降序
	rankScore := map[string]int{
		"high":   4,
		"medium": 3,
		"low":    2,
		"info":   1,
	}

	sort.Slice(list, func(i, j int) bool {
		si := rankScore[list[i].RiskLevel]
		sj := rankScore[list[j].RiskLevel]
		if si != sj {
			return si > sj
		}
		if list[i].FailedCount != list[j].FailedCount {
			return list[i].FailedCount > list[j].FailedCount
		}
		return list[i].LastSeenTS > list[j].LastSeenTS
	})

	return list
}
