// Package validate provides composable argument validators for commands.
//
// Attach rules via the fluent builder: command.Define(...).Rules(...).
// Business packages can add Func / FuncRule validators without modifying the
// framework (Open/Closed).
//
// Built-in rules:
//   - Required(name)              — flag must be set and non-blank
//   - LengthBetween(name, min, max) — trimmed rune length (0 skips a bound)
//   - OneOf(name, values...)      — enum check when the flag is set
//   - FuncRule(key, pred, args...) — custom predicate → UsageFail(key, args...)
//
// Example (register.go):
//
//	command.Define("hello").
//	    Rules(
//	        validate.Required("text"),
//	        validate.LengthBetween("text", 1, 64),
//	        validate.OneOf("mode", "fast", "safe"),
//	    ).
//	    Handle(Run).
//	    Register()
//
// Example (ad-hoc in a handler or test):
//
//	err := validate.RunAll(in,
//	    validate.Required("id"),
//	    validate.LengthBetween("id", 1, 32),
//	)
package validate

import (
	"strings"
	"unicode/utf8"

	"gmssh-cli-plugin/internal/input"
)

// Func is a single validation Strategy. Return nil on success, or a
// *input.UsageError (via input.UsageFail) so the engine exits with code 2.
type Func func(in input.Input) error

// RunAll executes rules in order and returns the first error.
// Nil rules are skipped. An empty rule list always succeeds.
func RunAll(in input.Input, rules ...Func) error {
	for _, rule := range rules {
		if rule == nil {
			continue
		}
		if err := rule(in); err != nil {
			return err
		}
	}
	return nil
}

// Required fails when the named flag is missing or blank after TrimSpace.
//
//	validate.Required("text")
func Required(name string) Func {
	return func(in input.Input) error {
		if !in.IsSet(name) || strings.TrimSpace(in.Str(name)) == "" {
			return input.UsageFail("cli.missing_required", name)
		}
		return nil
	}
}

// LengthBetween checks trimmed string rune length (inclusive).
// Pass 0 for min or max to skip that bound.
//
//	validate.LengthBetween("text", 1, 64) // 1..64 runes
//	validate.LengthBetween("note", 0, 200) // at most 200
func LengthBetween(name string, min, max int) Func {
	return func(in input.Input) error {
		n := utf8.RuneCountInString(strings.TrimSpace(in.Str(name)))
		if min > 0 && n < min {
			return input.UsageFail("cli.too_short", name, min)
		}
		if max > 0 && n > max {
			return input.UsageFail("cli.too_long", name, max)
		}
		return nil
	}
}

// OneOf checks enum membership when the flag is set.
// Unset flags pass (combine with Required when the field is mandatory).
//
//	validate.OneOf("mode", "fast", "safe")
func OneOf(name string, values ...string) Func {
	allowed := map[string]struct{}{}
	for _, v := range values {
		allowed[v] = struct{}{}
	}
	return func(in input.Input) error {
		if !in.IsSet(name) {
			return nil
		}
		v := in.Str(name)
		if _, ok := allowed[v]; ok {
			return nil
		}
		return input.UsageFail("cli.invalid_enum", name, v, strings.Join(values, ", "))
	}
}

// FuncRule wraps a custom predicate. On failure it returns UsageFail(key, args...).
// Prefer this over editing Required / LengthBetween for domain-specific checks.
//
//	validate.FuncRule("cli.invalid_enum", func(in input.Input) bool {
//	    return strings.HasPrefix(in.Str("id"), "srv-")
//	}, "id", in.Str("id"), "srv-*")
func FuncRule(key string, ok func(input.Input) bool, args ...any) Func {
	return func(in input.Input) error {
		if ok(in) {
			return nil
		}
		return input.UsageFail(key, args...)
	}
}
