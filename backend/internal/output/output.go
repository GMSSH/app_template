// Package output renders command results to stdout as JSON or plain text.
//
// Single responsibility: serialization only. Handlers must return data (or
// error); they must not fmt.Print. The Respond middleware and command.fail
// call these helpers.
//
// Default wire format is a compact JSON envelope (UTF-8, no \uXXXX escapes):
//
//	Success → {"ok":true,"data":<handler result>}
//	Failure → {"ok":false,"error":{"code":N,"type":"...","message":"..."}}
//
// Text mode (--format text / GMCLI_FORMAT=text) prints human-readable lines
// for success and a plain message for failure. Byte charset is controlled by
// --encoding / GMCLI_ENCODING (default utf-8); see NewEncodedWriter.
package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
)

// SuccessEnvelope is the default success wire format.
type SuccessEnvelope struct {
	OK   bool `json:"ok"`
	Data any  `json:"data"`
}

// ErrorBody is the error object inside FailureEnvelope.
type ErrorBody struct {
	Code    int    `json:"code"`
	Type    string `json:"type"`
	Message string `json:"message"`
	Key     string `json:"key,omitempty"`   // i18n key (customize in i18n/*.ini)
	Frame   string `json:"frame,omitempty"` // source file:line when known
}

// FailureEnvelope is the default failure wire format.
type FailureEnvelope struct {
	OK    bool      `json:"ok"`
	Error ErrorBody `json:"error"`
}

// OK wraps a successful handler result in the standard JSON envelope.
func OK(data any) SuccessEnvelope {
	return SuccessEnvelope{OK: true, Data: data}
}

// Err wraps a failure in the standard JSON envelope.
// key/frame are optional (i18n key and source file:line).
func Err(code int, typ, message, key, frame string) FailureEnvelope {
	return FailureEnvelope{
		OK: false,
		Error: ErrorBody{
			Code:    code,
			Type:    typ,
			Message: message,
			Key:     key,
			Frame:   frame,
		},
	}
}

// WriteJSON encodes result and writes one JSON document to stdout.
// Encoding errors are also printed to stderr when stderr is non-nil.
func WriteJSON(stdout, stderr io.Writer, result any, compact bool) error {
	data, err := marshalResult(result, compact)
	if err != nil {
		if stderr != nil {
			_, _ = fmt.Fprintln(stderr, err.Error())
		}
		return err
	}
	_, err = fmt.Fprintln(stdout, string(data))
	return err
}

func marshalResult(result any, compact bool) ([]byte, error) {
	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	// Emit raw UTF-8 code points instead of \uXXXX so hosts / scripts that
	// expect readable Chinese (or other non-ASCII) do not see escaped noise.
	data = unescapeJSONUnicode(data)
	if compact {
		return data, nil
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "  "); err != nil {
		return data, nil
	}
	return buf.Bytes(), nil
}

// unescapeJSONUnicode rewrites JSON \uXXXX (and surrogate pairs) inside
// string literals to raw UTF-8 bytes. Other escape sequences are preserved.
func unescapeJSONUnicode(data []byte) []byte {
	var buf bytes.Buffer
	buf.Grow(len(data))
	inString := false
	escaped := false
	for i := 0; i < len(data); {
		c := data[i]
		if !inString {
			buf.WriteByte(c)
			if c == '"' {
				inString = true
			}
			i++
			continue
		}
		if escaped {
			buf.WriteByte(c)
			escaped = false
			i++
			continue
		}
		if c == '\\' {
			if i+5 < len(data) && data[i+1] == 'u' {
				r1, err1 := parseHexRune(data[i+2 : i+6])
				if err1 == nil {
					i += 6
					if utf16.IsSurrogate(r1) && i+5 < len(data) && data[i] == '\\' && data[i+1] == 'u' {
						r2, err2 := parseHexRune(data[i+2 : i+6])
						if err2 == nil {
							buf.WriteRune(utf16.DecodeRune(r1, r2))
							i += 6
							continue
						}
					}
					buf.WriteRune(r1)
					continue
				}
			}
			buf.WriteByte(c)
			escaped = true
			i++
			continue
		}
		if c == '"' {
			inString = false
		}
		buf.WriteByte(c)
		i++
	}
	return buf.Bytes()
}

func parseHexRune(b []byte) (rune, error) {
	v, err := strconv.ParseUint(string(b), 16, 32)
	if err != nil {
		return 0, err
	}
	return rune(v), nil
}

// WriteText renders result as human-readable lines (maps/lists indented).
// Structs are converted via JSON round-trip so json tags control field names.
func WriteText(w io.Writer, result any) error {
	plain, err := toPlain(result)
	if err != nil {
		_, writeErr := fmt.Fprintf(w, "%v\n", result)
		return writeErr
	}
	if plain == nil {
		return nil
	}
	lines := renderPlain(plain, 0)
	if len(lines) == 0 {
		return nil
	}
	_, err = fmt.Fprintf(w, "%s\n", strings.Join(lines, "\n"))
	return err
}

func jsonMarshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSpace(buf.Bytes()), nil
}

func jsonUnmarshal(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

func toPlain(result any) (any, error) {
	data, err := jsonMarshal(result)
	if err != nil {
		return nil, err
	}
	return jsonUnmarshal(data)
}

func scalarText(v any) (string, bool) {
	switch val := v.(type) {
	case nil:
		return "null", true
	case bool:
		return fmt.Sprint(val), true
	case string:
		return val, true
	case json.Number:
		return val.String(), true
	default:
		return "", false
	}
}

func renderPlain(v any, indent int) []string {
	pad := strings.Repeat(" ", indent)
	if text, ok := scalarText(v); ok {
		return []string{pad + text}
	}
	switch val := v.(type) {
	case map[string]any:
		return renderMap(val, indent, pad)
	case []any:
		return renderList(val, indent, pad)
	default:
		return []string{pad + fmt.Sprint(v)}
	}
}

func renderMap(val map[string]any, indent int, pad string) []string {
	if len(val) == 0 {
		return []string{pad + "{}"}
	}
	keys := make([]string, 0, len(val))
	for key := range val {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, renderKeyed(key, val[key], indent, pad)...)
	}
	return lines
}

func renderKeyed(key string, child any, indent int, pad string) []string {
	if text, ok := scalarText(child); ok {
		return []string{pad + key + ": " + text}
	}
	lines := []string{pad + key + ":"}
	return append(lines, renderPlain(child, indent+2)...)
}

func renderList(val []any, indent int, pad string) []string {
	if len(val) == 0 {
		return []string{pad + "[]"}
	}
	lines := make([]string, 0, len(val))
	for _, item := range val {
		lines = append(lines, renderListItem(item, indent, pad)...)
	}
	return lines
}

func renderListItem(item any, indent int, pad string) []string {
	if text, ok := scalarText(item); ok {
		return []string{pad + "- " + text}
	}
	lines := []string{pad + "-"}
	return append(lines, renderPlain(item, indent+2)...)
}
