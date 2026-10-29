package domain

import (
	"context"
	"fmt"
	"time"

	"gmssh-cli-plugin/internal/biz/domain/inspector"
	"gmssh-cli-plugin/internal/biz/domain/security"
	"gmssh-cli-plugin/internal/command"
	"gmssh-cli-plugin/internal/input"
	"gmssh-cli-plugin/internal/middleware"
)

func init() {
	command.Define("domain").
		Summary("cmd.domain.summary").
		Group("group.network").
		Arg(command.Arg{
			Name: "action", Type: command.TypeString, Required: true,
			Desc: "Action: probe | overview | rdap | dns | multidns | web | ssl | port | redirect | mail | dnssec",
		}).
		Arg(command.Arg{
			Name: "domain", Type: command.TypeString, Required: false,
			Desc: "Target domain name, e.g. example.com",
		}).
		Examples(
			"domain --action probe --json",
			"domain --action overview --domain example.com --json",
			"domain --action ssl --domain example.com --json",
		).
		Handle(Run).
		Register()
}

// Run dispatches domain actions
func Run(ctx *middleware.Context, in input.Input) (any, error) {
	action := in.Str("action")
	if action == "" {
		return nil, command.UsageFail("cli.missing_required", "action")
	}

	if action == "probe" {
		return map[string]any{
			"status":  "ok",
			"version": "1.0.0",
			"engine":  "domain-inspector",
			"time":    time.Now().Format(time.RFC3339),
		}, nil
	}

	rawDomain := in.Str("domain")
	if rawDomain == "" {
		return nil, command.UsageFail("cli.missing_required", "domain")
	}

	domain, err := security.NormalizeDomain(rawDomain)
	if err != nil {
		// 把具体原因带给用户，而不是统一的 "Operation failed"
		return nil, command.UsageFail("cli.invalid_domain", rawDomain, err.Error())
	}

	execCtx := context.Background()

	switch action {
	case "overview":
		return inspector.RunOverview(execCtx, domain)
	case "rdap":
		return inspector.InspectRDAP(execCtx, domain)
	case "dns":
		return inspector.InspectDNS(execCtx, domain)
	case "multidns":
		return inspector.InspectMultiDNS(execCtx, domain)
	case "web":
		return inspector.InspectWeb(execCtx, domain)
	case "ssl":
		return inspector.InspectSSL(execCtx, domain)
	case "port":
		return inspector.InspectPort(execCtx, domain)
	case "redirect":
		return inspector.InspectRedirect(execCtx, domain)
	case "mail":
		return inspector.InspectMail(execCtx, domain)
	case "dnssec":
		return inspector.InspectDNSSEC(execCtx, domain)
	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}
}
