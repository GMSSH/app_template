package consts

// Process environment variable names (GMCLI_* prefix).
//
// Examples:
//
//	export GMCLI_PROFILE=development
//	export GMCLI_LANG=zh-CN
//	export GMCLI_LOG_LEVEL=debug
//	export GMCLI_ROOT=/opt/my-plugin
const (
	EnvLang           = "GMCLI_LANG"
	EnvI18nDir        = "GMCLI_I18N_DIR"
	EnvRoot           = "GMCLI_ROOT"
	EnvConfig         = "GMCLI_CONFIG"
	EnvProfile        = "GMCLI_PROFILE"  // production | development | test
	EnvFormat         = "GMCLI_FORMAT"   // json | text (default json, compact)
	EnvEncoding       = "GMCLI_ENCODING" // utf-8 | gbk | gb18030 | gb2312 (default utf-8)
	EnvLogLevel       = "GMCLI_LOG_LEVEL"
	EnvLogDir         = "GMCLI_LOG_DIR"
	EnvLogFile        = "GMCLI_LOG_FILE" // legacy single-file override (runtime)
	EnvLogFilePattern = "GMCLI_LOG_FILE_PATTERN"
	EnvLogMaxDays     = "GMCLI_LOG_MAX_DAYS"
	EnvLogConsole     = "GMCLI_LOG_CONSOLE"
)
