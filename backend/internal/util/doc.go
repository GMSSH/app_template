// Package util collects small, domain-free helpers shared across internal packages.
//
// Keep additions generic (files, strings, bools, paths, display width). Do not put
// CLI parsing, i18n catalogs, logging policy, or business rules here — those belong
// in their own packages. Prefer a new focused file inside this package over a
// second catch-all package.
//
// Files:
//   - bool.go    — ParseBool / BoolLiteral
//   - display.go — terminal width helpers for help alignment
//   - file.go    — Exists / FileExists / WriteFile
//   - path.go    — ResolveUnderRoot (safe join)
//   - strings.go — ContainsString / SplitCSV / ToString
//
// Example:
//
//	if util.FileExists(path) { ... }
//	full, err := util.ResolveUnderRoot(root, "prefs.json")
//	on, ok := util.ParseBool("yes")
package util
