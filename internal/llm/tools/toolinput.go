package tools

// GORILLA OVERRIDE (2026-08-20): models send numbers as strings, and a strict
// decoder turns that into an unrecoverable loop.
//
// Observed live: meta/llama-3.3-70b-instruct called web_fetch with
// {"url":"https://www.debian.org/","format":"markdown","timeout":"30"} - the
// timeout quoted. json.Unmarshal into `Timeout int` fails with "cannot
// unmarshal string into Go struct field FetchParams.timeout of type int", the
// tool returns an error, the model reads the error, forms the SAME call again,
// and the pair loop until the user gives up. Four identical failures in
// forty-four seconds, each one paid for.
//
// The intent was never ambiguous. "30" and 30 mean the same thing here, and
// refusing the first is pedantry that costs the user money. JSON Schema says
// integer, but a model is not a validating client and cannot be made into one
// by rejecting it harder.
//
// This is deliberately NOT a general "accept anything" decoder:
//   - It only relaxes fields whose DESTINATION is numeric or boolean. A string
//     field receiving "30" or "true" is left completely alone, so a search for
//     the literal text "true" still searches for text.
//   - It runs only AFTER a strict decode has already failed, so well-formed
//     arguments never touch this path and pay nothing for it.
//   - If the relaxed decode also fails it returns the ORIGINAL error, because
//     the first error describes what the model actually got wrong.
//
// Same family as the v0.1.83 repair for tool calls arriving with null
// arguments: the wire is not as clean as the schema promises, and the choice is
// between meeting it where it is or shipping a loop.

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
)

// UnmarshalToolInput decodes a model's tool arguments into a parameter struct,
// accepting string-encoded numbers and booleans for fields that want the real
// thing.
func UnmarshalToolInput(raw string, into any) error {
	strictErr := json.Unmarshal([]byte(raw), into)
	if strictErr == nil {
		return nil
	}

	// GORILLA OVERRIDE (2026-09-01): repair unescaped Windows paths.
	//
	// A model working on Windows writes what it sees, and what it sees is
	// C:\Users\someone\project\main.go. Put in JSON unescaped, "\U" is not a
	// legal escape, the decode fails with
	//
	//	invalid escape sequence `\U` in string
	//
	// and the tool returns that to the model — which reads it, forms the same
	// call again, and loops. This is the identical failure mode this file was
	// written for (quoted numbers), in the identical shape, and it fires on the
	// single most-used tool in the product: `view` accounts for 92 of the 130
	// tool calls in the local session database.
	//
	// The repair is narrow on purpose. It runs only after a strict decode has
	// already failed, it only touches backslashes INSIDE string literals, and it
	// only doubles a backslash that is not already starting a legal JSON escape
	// — so "\n" stays a newline and "\\" stays an escaped backslash. A path is
	// recovered; nothing else is reinterpreted.
	//
	// GORILLA OVERRIDE (2026-10-04): bare control characters, and containers that
	// arrive as strings. Both ideas come from alibaba/open-code-review
	// (internal/tool/comment_args_repair.go, Apache-2.0); the code is ours.
	//
	// A small model asked to write a file puts a REAL newline inside the JSON
	// string instead of the two characters backslash-n. JSON forbids every byte
	// below 0x20 inside a string, so the whole call is refused and the content is
	// lost. Escaping those bytes is lossless: a raw newline inside a string
	// literal has no other possible meaning.
	//
	// The repairs are tried alone and together, and one is used only if the
	// result then decodes. base keeps the first repaired text that is at least
	// valid JSON, so the field-level coercions below have something to work on.
	base := []byte(raw)
	var candidates [][]byte
	if slashes, ok := escapeLoneBackslashes([]byte(raw)); ok {
		candidates = append(candidates, slashes)
	}
	if controls, ok := escapeBareControls([]byte(raw)); ok {
		candidates = append(candidates, controls)
		if both, ok := escapeLoneBackslashes(controls); ok {
			candidates = append(candidates, both)
		}
	}
	for _, c := range candidates {
		if err := json.Unmarshal(c, into); err == nil {
			return nil
		}
		if !json.Valid(base) && json.Valid(c) {
			base = c
		}
	}

	coerced, ok := coerceFields(base, into)
	if !ok {
		return strictErr
	}
	if err := json.Unmarshal(coerced, into); err != nil {
		// The strict error names the field the model got wrong; this one would
		// describe our rewritten copy, which the model never sent.
		return strictErr
	}
	return nil
}

// coerceFields rewrites a value whose SHAPE is wrong for the field it is headed
// to, and nothing else:
//
//   - a quoted scalar ("30", "true") headed for a numeric or boolean field;
//   - a container serialised into a string headed for a slice, map or struct
//     field;
//   - one bare string headed for a []string field, which becomes a list of one.
//
// A string headed for a string field is never touched. Reports false when there
// was nothing it could safely change.
func coerceFields(raw []byte, into any) ([]byte, bool) {
	kinds := scalarFieldKinds(into)
	containers := containerFieldTypes(into)
	if len(kinds) == 0 && len(containers) == 0 {
		return nil, false
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, false // not an object; nothing keyed to fix
	}

	changed := false
	for key, val := range obj {
		var s string
		if err := json.Unmarshal(val, &s); err != nil {
			continue // not a string, so its shape is not this function's business
		}
		if kind, wanted := kinds[key]; wanted {
			if lit, ok := scalarLiteral(s, kind); ok {
				obj[key] = json.RawMessage(lit)
				changed = true
			}
			continue
		}
		if typ, wanted := containers[key]; wanted {
			if lit, ok := containerLiteral(s, typ); ok {
				obj[key] = json.RawMessage(lit)
				changed = true
			}
		}
	}
	if !changed {
		return nil, false
	}
	out, err := json.Marshal(obj)
	if err != nil {
		return nil, false
	}
	return out, true
}

// scalarLiteral turns "30" into 30 and "true" into true, refusing anything that
// is not exactly a value of the wanted kind. "30 seconds" is not a number and
// must stay an error the model can read.
func scalarLiteral(s string, kind reflect.Kind) (string, bool) {
	switch kind {
	case reflect.Bool:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return "", false
		}
		return strconv.FormatBool(b), true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if _, err := strconv.ParseInt(s, 10, 64); err != nil {
			return "", false
		}
		return s, true
	case reflect.Float32, reflect.Float64:
		if _, err := strconv.ParseFloat(s, 64); err != nil {
			return "", false
		}
		return s, true
	}
	return "", false
}

// scalarFieldKinds maps json keys to the kind of the field behind them, for
// numeric and boolean fields only.
func scalarFieldKinds(into any) map[string]reflect.Kind {
	t := reflect.TypeOf(into)
	for t != nil && t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct {
		return nil
	}
	out := map[string]reflect.Kind{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name := jsonKey(f)
		if name == "" {
			continue
		}
		ft := f.Type
		for ft.Kind() == reflect.Ptr {
			ft = ft.Elem()
		}
		switch ft.Kind() {
		case reflect.Bool,
			reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Float32, reflect.Float64:
			out[name] = ft.Kind()
		}
	}
	return out
}

func jsonKey(f reflect.StructField) string {
	tag, ok := f.Tag.Lookup("json")
	if !ok {
		return f.Name
	}
	for i := 0; i < len(tag); i++ {
		if tag[i] == ',' {
			tag = tag[:i]
			break
		}
	}
	if tag == "-" {
		return ""
	}
	if tag == "" {
		return f.Name
	}
	return tag
}

// escapeLoneBackslashes doubles any backslash inside a JSON string literal that
// is not already introducing a legal escape sequence. Reports false when there
// was nothing to change, so the caller can keep the original error.
//
// The legal escapes are the eight in the JSON grammar: " \ / b f n r t, plus u
// followed by four hex digits. A backslash before anything else — U, s, p, a
// digit, a space — cannot be valid JSON, so doubling it is the only reading that
// could have been meant.
func escapeLoneBackslashes(raw []byte) ([]byte, bool) {
	out := make([]byte, 0, len(raw)+16)
	inString := false
	changed := false

	for i := 0; i < len(raw); i++ {
		c := raw[i]

		if !inString {
			if c == '"' {
				inString = true
			}
			out = append(out, c)
			continue
		}

		if c == '"' {
			inString = false
			out = append(out, c)
			continue
		}

		if c != '\\' {
			out = append(out, c)
			continue
		}

		// A trailing backslash cannot be repaired into anything sensible.
		if i+1 >= len(raw) {
			out = append(out, c)
			continue
		}

		next := raw[i+1]
		switch next {
		case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
			// A legal two-character escape: copy both, and skip the second so a
			// literal \\ is never mistaken for an escape of the character after it.
			out = append(out, c, next)
			i++
		case 'u':
			if i+5 < len(raw) && isHex(raw[i+2]) && isHex(raw[i+3]) && isHex(raw[i+4]) && isHex(raw[i+5]) {
				out = append(out, raw[i:i+6]...)
				i += 5
			} else {
				out = append(out, '\\', '\\')
				changed = true
			}
		default:
			out = append(out, '\\', '\\')
			changed = true
		}
	}

	if !changed {
		return nil, false
	}
	return out, true
}

func isHex(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

// containerFieldTypes maps json keys to the type of the field behind them, for
// slice, array, map and struct fields only.
func containerFieldTypes(into any) map[string]reflect.Type {
	t := reflect.TypeOf(into)
	for t != nil && t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct {
		return nil
	}
	out := map[string]reflect.Type{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name := jsonKey(f)
		if name == "" {
			continue
		}
		ft := f.Type
		for ft.Kind() == reflect.Ptr {
			ft = ft.Elem()
		}
		switch ft.Kind() {
		case reflect.Slice, reflect.Array, reflect.Map, reflect.Struct:
			out[name] = ft
		}
	}
	return out
}

// containerLiteral recovers the container a model serialised into a string.
//
// Three cases, in order of how much is assumed:
//
//  1. The string is itself valid JSON of the right outer shape. Unwrap it; this
//     assumes nothing.
//  2. The string looks like a container but does not parse, because the model
//     dropped one level of escaping. Repair it, and accept the result only if it
//     passes repairedContainerAcceptable.
//  3. The field is a list of strings and the model sent one plain string. That
//     is a list of one.
func containerLiteral(s string, typ reflect.Type) (string, bool) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return "", false
	}

	open := byte('{')
	if typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array {
		open = '['
	}
	if trimmed[0] == open {
		if json.Valid([]byte(trimmed)) {
			return trimmed, true
		}
		repaired, n := repairSerializedContainer(trimmed)
		if n > 0 && repairedContainerAcceptable(repaired, typ) {
			return repaired, true
		}
		return "", false
	}

	if typ.Kind() == reflect.Slice && typ.Elem().Kind() == reflect.String {
		lit, err := json.Marshal([]string{s})
		if err != nil {
			return "", false
		}
		return string(lit), true
	}
	return "", false
}

// escapeBareControls escapes every byte below 0x20 that sits inside a JSON
// string literal. Reports false when there was nothing to change.
func escapeBareControls(raw []byte) ([]byte, bool) {
	out := make([]byte, 0, len(raw)+16)
	inString := false
	changed := false
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if !inString {
			if c == '"' {
				inString = true
			}
			out = append(out, c)
			continue
		}
		switch {
		case c == '\\' && i+1 < len(raw):
			// Copy an escape pair whole, so an escaped quote does not end the string.
			out = append(out, c, raw[i+1])
			i++
		case c == '"':
			inString = false
			out = append(out, c)
		case c < 0x20:
			out = append(out, controlEscape(c)...)
			changed = true
		default:
			out = append(out, c)
		}
	}
	if !changed {
		return nil, false
	}
	return out, true
}

// controlEscape is the JSON escape for a control character: the short form
// where JSON has one, the six-character unicode form otherwise.
func controlEscape(c byte) string {
	switch c {
	case '\n':
		return `\n`
	case '\r':
		return `\r`
	case '\t':
		return `\t`
	case '\b':
		return `\b`
	case '\f':
		return `\f`
	}
	const hex = "0123456789abcdef"
	return `\u00` + string([]byte{hex[c>>4], hex[c&0x0f]})
}

// repairSerializedContainer escapes what makes a model-serialised container
// invalid: quotes inside prose, bare control characters, backslashes that open
// no legal escape. It returns the text and how many characters it escaped.
//
// A real closing quote is always followed by ',' '}' ']' ':' or the end of the
// text. A quote followed by anything else is content. So the scan never misses
// a genuine terminator; its one possible mistake is ending a string EARLY, at a
// content quote that happens to precede punctuation. repairedContainerAcceptable
// exists to catch that.
func repairSerializedContainer(s string) (string, int) {
	var b strings.Builder
	b.Grow(len(s) + 16)
	escaped := 0
	inString := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !inString {
			if c == '"' {
				inString = true
			}
			b.WriteByte(c)
			continue
		}
		switch {
		case c == '\\':
			if legalEscapeAt(s, i+1) {
				b.WriteByte(c)
				i++
				b.WriteByte(s[i])
			} else {
				b.WriteString(`\\`)
				escaped++
			}
		case c == '"':
			j := i + 1
			for j < len(s) && (s[j] == ' ' || s[j] == '\t' || s[j] == '\r' || s[j] == '\n') {
				j++
			}
			if j >= len(s) || s[j] == ',' || s[j] == '}' || s[j] == ']' || s[j] == ':' {
				inString = false
				b.WriteByte(c)
			} else {
				b.WriteString(`\"`)
				escaped++
			}
		case c < 0x20:
			b.WriteString(controlEscape(c))
			escaped++
		default:
			b.WriteByte(c)
		}
	}
	return b.String(), escaped
}

func legalEscapeAt(s string, i int) bool {
	if i >= len(s) {
		return false
	}
	switch s[i] {
	case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
		return true
	case 'u':
		return i+4 < len(s) && isHex(s[i+1]) && isHex(s[i+2]) && isHex(s[i+3]) && isHex(s[i+4])
	}
	return false
}

// repairedContainerAcceptable decides whether a repaired container may be used.
// Parsing again is not enough: a misjudged terminator yields JSON that is valid
// and wrong. Two checks, both from the upstream repair:
//
//   - No object may carry a key the destination does not define. Prose re-read
//     as structure almost never spells a real field name.
//   - No string may hold an odd number of double quotes. A value cut short at a
//     misjudged terminator keeps an unpaired quote; prose quotes come in pairs.
//
// A refusal costs nothing: the caller reports the original error and the model
// resends, which is what happened before this repair existed.
func repairedContainerAcceptable(repaired string, typ reflect.Type) bool {
	var v any
	if err := json.Unmarshal([]byte(repaired), &v); err != nil {
		return false
	}
	return valueAcceptable(v, typ)
}

func valueAcceptable(v any, typ reflect.Type) bool {
	for typ != nil && typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	switch x := v.(type) {
	case string:
		return strings.Count(x, `"`)%2 == 0
	case []any:
		var elem reflect.Type
		if typ != nil && (typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array) {
			elem = typ.Elem()
		}
		for _, e := range x {
			if !valueAcceptable(e, elem) {
				return false
			}
		}
	case map[string]any:
		var fields map[string]reflect.Type
		if typ != nil && typ.Kind() == reflect.Struct {
			fields = map[string]reflect.Type{}
			for i := 0; i < typ.NumField(); i++ {
				if name := jsonKey(typ.Field(i)); name != "" {
					fields[name] = typ.Field(i).Type
				}
			}
		}
		for k, e := range x {
			var ft reflect.Type
			if fields != nil {
				known := false
				if ft, known = fields[k]; !known {
					return false
				}
			} else if typ != nil && typ.Kind() == reflect.Map {
				ft = typ.Elem()
			}
			if !valueAcceptable(e, ft) {
				return false
			}
		}
	}
	return true
}
