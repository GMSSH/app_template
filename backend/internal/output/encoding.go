package output

import (
	"fmt"
	"io"
	"strings"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"

	"gmssh-cli-plugin/internal/consts"
)

// ResolveEncoding maps a charset name to an encoding.Encoding.
// Empty / utf-8 / utf8 → nil encoding (pass-through; Go strings are already UTF-8).
// Supported: utf-8, gbk, gb18030, gb2312 (alias of gbk).
func ResolveEncoding(name string) (encoding.Encoding, string, error) {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" {
		n = consts.DefaultEncoding
	}
	switch n {
	case "utf-8", "utf8", "unicode":
		return nil, "utf-8", nil
	case "gbk":
		return simplifiedchinese.GBK, "gbk", nil
	case "gb18030":
		return simplifiedchinese.GB18030, "gb18030", nil
	case "gb2312":
		return simplifiedchinese.GBK, "gb2312", nil
	default:
		return nil, "", fmt.Errorf("%s", name)
	}
}

// NewEncodedWriter wraps w so UTF-8 Go strings are converted to the target charset.
// When enc is nil (utf-8), w is returned unchanged.
func NewEncodedWriter(w io.Writer, enc encoding.Encoding) io.Writer {
	if w == nil || enc == nil {
		return w
	}
	return transform.NewWriter(w, enc.NewEncoder())
}
