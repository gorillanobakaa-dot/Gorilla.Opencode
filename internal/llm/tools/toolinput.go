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
	if repaired, ok := escapeLoneBackslashes([]byte(raw)); ok {
		if err := json.Unmarshal(repaired, into); err == nil {
			return nil
		}
	}

	coerced, ok := coerceScalarStrings([]byte(raw), into)
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

// coerceScalarStrings rewrites quoted scalars to bare ones, but ONLY for keys
// whose destination field is numeric or boolean. Reports false when there was
// nothing it could safely change.
func coerceScalarStrings(raw []byte, into any) ([]byte, bool) {
	kinds := scalarFieldKinds(into)
	if len(kinds) == 0 {
		return nil, false
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, false // not an object; nothing keyed to fix
	}

	changed := false
	for key, val := range obj {
		kind, wanted := kinds[key]
		if !wanted {
			continue
		}
		var s string
		if err := json.Unmarshal(val, &s); err != nil {
			continue // already a number/bool, or something else entirely
		}
		lit, ok := scalarLiteral(s, kind)
		if !ok {
			continue
		}
		obj[key] = json.RawMessage(lit)
		changed = true
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
