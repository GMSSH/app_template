package command

import (
	"fmt"
	"io"
	"strings"

	"gmssh-cli-plugin/internal/util"
)

// Localized help rendering for root and per-command screens.
// All user-visible strings come from i18n keys (see i18n/*.ini).

// rootGlobalHelpKeys lists i18n keys for root-help global options.
// Keep in sync when extending globalCatalog in flags.go.
var rootGlobalHelpKeys = []string{
	"help.opt_format",
	"help.opt_json",
	"help.opt_compact",
	"help.opt_encoding",
	"help.opt_lang",
	"help.opt_debug",
	"help.opt_log_level",
	"help.opt_log_dir",
	"help.opt_log_file",
	"help.opt_log_max_days",
	"help.opt_log_console",
}

// printRootHelp writes the top-level help screen (commands + global options).
func printRootHelp(w io.Writer) {
	writeRootHeader(w)
	writeGlobalOptions(w)
	writeCommandGroups(w)
	_, _ = fmt.Fprintf(w, "\n%s\n", T("help.tip"))
}

func writeRootHeader(w io.Writer) {
	_, _ = fmt.Fprintf(w, "%s\n\n", T("help.title", AppName))
	_, _ = fmt.Fprintf(w, "%s\n", T("help.usage"))
	_, _ = fmt.Fprintf(w, "%s\n", T("help.usage_cmd"))
	_, _ = fmt.Fprintf(w, "%s\n", T("help.usage_root"))
	_, _ = fmt.Fprintf(w, "%s\n", T("help.usage_cmd_help"))
	_, _ = fmt.Fprintf(w, "%s\n\n", T("help.usage_version"))
}

func writeGlobalOptions(w io.Writer) {
	_, _ = fmt.Fprintf(w, "%s\n", T("help.global"))
	for i, key := range rootGlobalHelpKeys {
		suffix := "\n"
		if i == len(rootGlobalHelpKeys)-1 {
			suffix = "\n\n"
		}
		_, _ = fmt.Fprintf(w, "%s%s", T(key), suffix)
	}
}

func writeCommandGroups(w io.Writer) {
	commands := Commands()
	_, _ = fmt.Fprintf(w, "%s\n", T("help.commands", len(commands)))
	if len(commands) == 0 {
		_, _ = fmt.Fprintf(w, "%s\n\n", T("help.commands_empty"))
		return
	}
	groups, order := groupCommands(commands)
	nameWidth := commandNameWidth(commands) + 2
	for _, key := range order {
		_, _ = fmt.Fprintf(w, "\n[%s]\n", T(key))
		for _, cmd := range groups[key] {
			_, _ = fmt.Fprintf(w, "  %s%s\n", util.PadRight(cmd.Name, nameWidth), T(cmd.Summary))
		}
	}
}

func groupCommands(commands []*Command) (map[string][]*Command, []string) {
	groups := map[string][]*Command{}
	order := make([]string, 0, len(commands))
	for _, cmd := range commands {
		key := cmd.Group
		if key == "" {
			key = "group.other"
		}
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], cmd)
	}
	return groups, order
}

func commandNameWidth(commands []*Command) int {
	width := 0
	for _, cmd := range commands {
		if w := util.DisplayWidth(cmd.Name); w > width {
			width = w
		}
	}
	return width
}

// printCommandHelp writes detailed help for one registered command.
func printCommandHelp(w io.Writer, cmd *Command) {
	_, _ = fmt.Fprintf(w, "%s — %s\n\n", cmd.Name, T(cmd.Summary))
	_, _ = fmt.Fprintf(w, "%s\n", T("help.command_usage"))
	_, _ = fmt.Fprintf(w, "%s\n\n", T("help.command_usage_line", cmd.Name+usageSuffix(cmd.Args)))
	writeCommandArgs(w, cmd)
	writeCommandExamples(w, cmd)
	_, _ = fmt.Fprintf(w, "%s\n", T("help.exit_codes"))
}

func writeCommandArgs(w io.Writer, cmd *Command) {
	_, _ = fmt.Fprintf(w, "%s\n", T("help.args"))
	if len(cmd.Args) == 0 {
		_, _ = fmt.Fprintf(w, "%s\n\n", T("help.no_args"))
		return
	}
	nameWidth, typeWidth := argColumnWidths(cmd.Args)
	for _, arg := range cmd.Args {
		_, _ = fmt.Fprintln(w, formatArgLine(arg, nameWidth, typeWidth))
	}
	_, _ = fmt.Fprintf(w, "%s\n\n", T("help.global_hint"))
}

func argColumnWidths(args []Arg) (nameWidth, typeWidth int) {
	for _, arg := range args {
		if w := util.DisplayWidth("--" + arg.Name); w > nameWidth {
			nameWidth = w
		}
		if w := util.DisplayWidth(argTypeText(arg)); w > typeWidth {
			typeWidth = w
		}
	}
	return nameWidth + 2, typeWidth + 2
}

func formatArgLine(arg Arg, nameWidth, typeWidth int) string {
	required := T("help.optional")
	if arg.Required {
		required = T("help.required")
	}
	line := "  " + util.PadRight("--"+arg.Name, nameWidth) + util.PadRight(argTypeText(arg), typeWidth) + required
	if arg.Desc != "" {
		line += "  " + T(arg.Desc)
	}
	if arg.Default != "" {
		line += " " + T("help.default", arg.Default)
	}
	if len(arg.Enum) > 0 {
		line += " " + T("help.enum", strings.Join(arg.Enum, ", "))
	}
	return line
}

func writeCommandExamples(w io.Writer, cmd *Command) {
	if len(cmd.Example) == 0 {
		return
	}
	_, _ = fmt.Fprintf(w, "%s\n", T("help.example"))
	for _, example := range cmd.Example {
		_, _ = fmt.Fprintf(w, "  main %s\n", example)
	}
	_, _ = fmt.Fprintln(w)
}

func usageSuffix(args []Arg) string {
	if len(args) == 0 {
		return ""
	}
	placeholder := T("help.value_placeholder")
	required := make([]string, 0, len(args))
	optional := make([]string, 0, len(args))
	for _, arg := range args {
		if arg.Required {
			required = append(required, "--"+arg.Name+" "+placeholder)
			continue
		}
		optional = append(optional, "[--"+arg.Name+" "+placeholder+"]")
	}
	return " " + strings.Join(append(required, optional...), " ")
}

func argTypeText(arg Arg) string {
	if len(arg.Enum) > 0 {
		return "<" + strings.Join(arg.Enum, "|") + ">"
	}
	return "<" + string(arg.Type) + ">"
}
