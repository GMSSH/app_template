package middleware

import (
	"testing"

	"gmssh-cli-plugin/internal/input"
)

func TestChainOrder(t *testing.T) {
	old := globalMW
	globalMW = nil
	t.Cleanup(func() { globalMW = old })

	var order []string
	Use(func(next Handler) Handler {
		return func(ctx *Context, in input.Input) (any, error) {
			order = append(order, "global")
			return next(ctx, in)
		}
	})
	extra := []Middleware{func(next Handler) Handler {
		return func(ctx *Context, in input.Input) (any, error) {
			order = append(order, "cmd")
			return next(ctx, in)
		}
	}}
	run := Chain(func(ctx *Context, in input.Input) (any, error) {
		order = append(order, "run")
		return nil, nil
	}, extra)
	if _, err := run(&Context{}, input.Input{}); err != nil {
		t.Fatal(err)
	}
	want := "global,cmd,run"
	got := ""
	for i, item := range order {
		if i > 0 {
			got += ","
		}
		got += item
	}
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}
