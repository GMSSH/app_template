package radar

import (
	"testing"

	"gmssh-cli-plugin/internal/input"
	"gmssh-cli-plugin/internal/middleware"
)

func TestRadarRun(t *testing.T) {
	ctx := &middleware.Context{}
	in, err := input.Parse("radar", []input.Arg{
		{Name: "action", Type: input.TypeString},
	}, []string{"--action", "server"})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	res, err := Run(ctx, in)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	report, ok := res.(ServerReport)
	if !ok {
		t.Fatalf("expected ServerReport, got %T", res)
	}
	if len(report.Events) == 0 {
		t.Errorf("expected non-empty events")
	}
	if report.Location.Name == "" {
		t.Errorf("expected server location name to be populated")
	}
}
