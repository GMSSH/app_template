// Package demo is the multi-command business example for the scaffold.
//
// It shows how one package registers several Handles and exercises:
//   - fluent Define(...).Register() (same style as hello)
//   - validate.Rules
//   - command-local middleware (Timing)
//   - usage errors (exit 2) vs runtime Fail (exit 1)
//   - structured results formatted by Respond (--json / text)
//
// Commands:
//
//	demo-echo   Handle(Echo)   format: structured Result → text/JSON
//	demo-upper  Handle(Upper)  Rules + Timing MW + UsageFail
//	demo-div    Handle(Div)    Timing MW + Fail on divide-by-zero
//
//	main demo-echo  --text hi
//	main demo-echo  --text hi --json
//	main demo-upper --text hi
//	main demo-upper --text fail-usage   # exit 2
//	main demo-div   --a 10 --b 2
//	main demo-div   --a 10 --b 0        # exit 1
package demo
