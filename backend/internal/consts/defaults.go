package consts

// Built-in defaults used when config files / env vars are absent.
const (
	DefaultLang            = "en"
	DefaultEncoding        = "utf-8"
	DefaultProfile         = "production"
	DefaultLogLevel        = "error"
	DefaultLogDir          = "../../logs"
	DefaultRuntimeLogFile  = "runtime-2006-01-02.log"  // Go time layout
	DefaultBusinessLogFile = "business-2006-01-02.log" // Go time layout
	DefaultLogFilePattern  = `^\w+-\d{4}-\d{2}-\d{2}\.log$`
	DefaultLogMaxDays      = 7
)
