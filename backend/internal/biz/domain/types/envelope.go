package types

import "time"

// Status constant definitions
const (
	StatusSuccess       = "success"
	StatusWarning       = "warning"
	StatusError         = "error"
	StatusUnavailable   = "unavailable"
	StatusNotConfigured = "not_configured"
)

// ModuleEnvelope is the standard wrapper for each module's result
type ModuleEnvelope struct {
	Module     string    `json:"module"`
	Domain     string    `json:"domain"`
	Status     string    `json:"status"`
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`
	DurationMs int64     `json:"durationMs"`
	Source     string    `json:"source"`
	Data       any       `json:"data"`
	Error      string    `json:"error,omitempty"`
}

// GlobalOverview holds the summary and all module results
type GlobalOverview struct {
	Domain         string                     `json:"domain"`
	Timestamp      time.Time                  `json:"timestamp"`
	ServerRegion   string                     `json:"serverRegion"`
	HttpStatus     int                        `json:"httpStatus"`
	HttpStatusText string                     `json:"httpStatusText"`
	PrimaryIP      string                     `json:"primaryIP"`
	ExpiryDays     *int                       `json:"expiryDays"`
	ExpiryDate     string                     `json:"expiryDate"`
	Modules        map[string]*ModuleEnvelope `json:"modules"`
}
