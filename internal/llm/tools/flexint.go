package tools

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// FlexInt is an int that tolerates the number shapes real models actually
// send.
//
// GORILLA OVERRIDE: measured 2026-08-16 in the session database —
// local.meta/llama-3.3-70b called view with {"offset":"100"} and the strict
// unmarshal failed with `cannot unmarshal string into Go struct field
// ViewParams.offset of type int`, three times, each a fully billed wasted
// turn. Accepting "100" for 100 is LOSSLESS coercion: the value is identical,
// nothing is guessed. This is deliberately unlike tool-NAME resolution
// (toolname.go), where fuzziness is a privilege-escalation primitive and is
// forbidden — a number is a value, not a capability.
//
// Accepted: 100, "100", 100.0 (JSON numbers arrive as floats from some
// serialisers), " 100 ", null (keeps zero). Refused, with a message that names
// the value: "abc", 100.5, {}, true — garbage stays an error.
type FlexInt int

func (f *FlexInt) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "null" {
		return nil
	}
	if strings.HasPrefix(s, `"`) {
		var str string
		if err := json.Unmarshal(b, &str); err != nil {
			return err
		}
		str = strings.TrimSpace(str)
		if str == "" {
			return nil
		}
		n, err := strconv.Atoi(str)
		if err != nil {
			return fmt.Errorf("expected a whole number, got %q", str)
		}
		*f = FlexInt(n)
		return nil
	}
	var n int
	if err := json.Unmarshal(b, &n); err == nil {
		*f = FlexInt(n)
		return nil
	}
	var fl float64
	if err := json.Unmarshal(b, &fl); err == nil {
		if fl != float64(int(fl)) {
			return fmt.Errorf("expected a whole number, got %v", fl)
		}
		*f = FlexInt(int(fl))
		return nil
	}
	return fmt.Errorf("expected a whole number, got %s", s)
}

// Int returns the plain int value.
func (f FlexInt) Int() int { return int(f) }
