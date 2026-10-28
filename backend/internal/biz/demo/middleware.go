package demo

import (
	"time"

	"gmssh-cli-plugin/internal/input"
	"gmssh-cli-plugin/internal/logging"
	"gmssh-cli-plugin/internal/middleware"
)

// Timing is a command-local middleware example: measure handler duration.
// Attach with Builder.Use(Timing). Global middleware (Locale / Respond / …)
// still runs; this wraps only the selected command's Handle.
func Timing(next middleware.Handler) middleware.Handler {
	return func(ctx *middleware.Context, in input.Input) (any, error) {
		start := time.Now()
		out, err := next(ctx, in)
		name := ""
		if ctx != nil {
			name = ctx.Name
		}
		logging.Business().Infof("demo timing command=%s elapsed=%s err=%v", name, time.Since(start), err != nil)
		return out, err
	}
}
