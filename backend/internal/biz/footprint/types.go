package footprint

// ProbeResult 预检探针返回结果
type ProbeResult struct {
	Available     bool     `json:"available"`
	Sources       []string `json:"sources"`
	HasJournal    bool     `json:"has_journal"`
	ReadableFiles []string `json:"readable_files"`
	Permission    bool     `json:"permission"`
	Timezone      string   `json:"timezone"`
	TimezoneDesc  string   `json:"timezone_desc"`
	Message       string   `json:"message"`
}

// TimeBucket 柱状图时间桶数据
type TimeBucket struct {
	Label        string `json:"label"`
	StartTime    int64  `json:"start_time"`
	EndTime      int64  `json:"end_time"`
	FailedCount  int    `json:"failed_count"`
	SuccessCount int    `json:"success_count"`
}

// SuspiciousIP 可疑来源 IP 统计项
type SuspiciousIP struct {
	IP           string   `json:"ip"`
	FailedCount  int      `json:"failed_count"`
	SuccessCount int      `json:"success_count"`
	Users        []string `json:"users"`
	FirstSeen    string   `json:"first_seen"`
	LastSeen     string   `json:"last_seen"`
	FirstSeenTS  int64    `json:"first_seen_ts"`
	LastSeenTS   int64    `json:"last_seen_ts"`
	RiskLevel    string   `json:"risk_level"`
	RiskTags     []string `json:"risk_tags"`
	RiskReasons  []string `json:"risk_reasons"`
	Logs         []string `json:"logs"`
}

// SummaryData 数据概览卡片
type SummaryData struct {
	TotalFailed  int `json:"total_failed"`
	TotalSuccess int `json:"total_success"`
	UniqueIPs    int `json:"unique_ips"`
}

// ScanResult 完整扫描分析结果
type ScanResult struct {
	Range         string         `json:"range"`
	ScanTime      string         `json:"scan_time"`
	Timezone      string         `json:"timezone"`
	WindowStart   string         `json:"window_start"`
	WindowEnd     string         `json:"window_end"`
	WindowStartTS int64          `json:"window_start_ts"`
	WindowEndTS   int64          `json:"window_end_ts"`
	CountMethod   string         `json:"count_method"`
	Summary       SummaryData    `json:"summary"`
	Buckets       []TimeBucket   `json:"buckets"`
	IPList        []SuspiciousIP `json:"ip_list"`
	ActiveSource  string         `json:"active_source"`
	TotalLogs     int            `json:"total_logs"`
	EmptyReason   string         `json:"empty_reason"`
}

// CountMethod 统计口径说明，随结果一起返回，避免使用者把事件数当成日志行数
const CountMethod = "按认证事件计数：一次认证尝试只算 1 次。" +
	"失败 = Failed password 或 Invalid user 行；" +
	"成功 = Accepted password 或 Accepted publickey 行；" +
	"Connection closed / Received disconnect / banner exchange 等连接层日志不计入。" +
	"未解析出时间戳的行不计入。"
