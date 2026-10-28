package demo

import (
	"fmt"

	"gmssh-cli-plugin/internal/command"
	"gmssh-cli-plugin/internal/input"
	"gmssh-cli-plugin/internal/logging"
	"gmssh-cli-plugin/internal/middleware"
)

// DivResult is the structured payload for demo-div (success path).
type DivResult struct {
	A        int     `json:"a"`
	B        int     `json:"b"`
	Quotient float64 `json:"quotient"`
}

// Div demonstrates Timing middleware + Fail / FailCause (exit 1).
//
// Customize user text in i18n/*.ini under key cmd.demo_div.div_zero —
// the JSON message comes from i18n, not from the underlying Go error.
func Div(_ *middleware.Context, in input.Input) (any, error) {
	a, b := in.Int("a"), in.Int("b")
	if b == 0 {
		// Simulate a low-level error; clients only see the i18n message.
		cause := fmt.Errorf("arithmetic: divide by zero (a=%d b=%d)", a, b)
		logging.Business().Errorf("demo-div %v", cause)
		return nil, command.FailCause("cmd.demo_div.div_zero", cause, a)
	}
	q := float64(a) / float64(b)
	logging.Business().Debugf("demo-div a=%d b=%d q=%v", a, b, q)
	return DivResult{A: a, B: b, Quotient: q}, nil
}
