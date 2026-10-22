// GoAWK interpreter value type (not exported).

package interp

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type valueType uint8

const (
	typeNull valueType = iota
	typeStr
	typeNum
	typeNumStr
)

// An AWK value (these are passed around by value)
type value struct {
	typ valueType // Type of value
	s   string    // String value (for typeStr and typeNumStr)
	n   float64   // Numeric value (for typeNum)
}

// Create a new null value
func null() value {
	return value{}
}

// Create a new number value
func num(n float64) value {
	return value{typ: typeNum, n: n}
}

// Create a new string value
func str(s string) value {
	return value{typ: typeStr, s: s}
}

// Create a new value to represent a "numeric string" from an input field
func numStr(s string) value {
	return value{typ: typeNumStr, s: s}
}

// Create a numeric value from a Go bool
func boolean(b bool) value {
	if b {
		return num(1)
	}
	return num(0)
}

// String returns a string representation of v for debugging.
func (v value) String() string {
	switch v.typ {
	case typeStr:
		return fmt.Sprintf("str(%q)", v.s)
	case typeNum:
		return fmt.Sprintf("num(%s)", v.str("%.6g"))
	case typeNumStr:
		return fmt.Sprintf("numStr(%q)", v.s)
	default:
		return "null()"
	}
}

// Return true if value is a "true string" (a string or a "numeric string"
// from an input field that can't be converted to a number). If false,
// also return the (possibly converted) number.
func (v value) isTrueStr() (float64, bool) {
	switch v.typ {
	case typeStr:
		return 0, true
	case typeNumStr:
		f, err := parseFloat(v.s)
		if err != nil {
			return 0, true
		}
		return f, false
	default: // typeNum, typeNull
		return v.n, false
	}
}

// Return Go bool value of AWK value. For numbers or numeric strings,
// zero is false and everything else is true. For strings, empty
// string is false and everything else is true.
func (v value) boolean() bool {
	switch v.typ {
	case typeStr:
		return v.s != ""
	case typeNumStr:
		f, err := parseFloat(v.s)
		if err != nil {
			return v.s != ""
		}
		return f != 0
	default: // typeNum, typeNull
		return v.n != 0
	}
}

// Like strconv.ParseFloat, but allow "+nan" and "-nan" (Skua: a NaN with that
// sign, printed back as gawk does). Also disallow underscore digit separators.
//
// Skua: as gawk, a string that looks like hex ("0x1A") is not a number, and
// inf/nan are numbers only when signed and exactly "+inf", "-inf", "+nan" or
// "-nan" (so input fields such as "nan", "info" or "Infinity" stay strings).
func parseFloat(s string) (float64, error) {
	s = strings.TrimSpace(s)
	body := s
	if len(s) > 1 && (s[0] == '+' || s[0] == '-') {
		body = s[1:]
		if len(s) == 4 && hasNaNPrefix(body) {
			// ParseFloat doesn't handle "nan" with sign prefix, so handle it here.
			return signedNaN(s[0] == '-'), nil
		}
	}
	if len(body) > 0 && (body[0] == 'i' || body[0] == 'I' || body[0] == 'n' || body[0] == 'N') {
		if body == s || len(s) != 4 || !hasInfPrefix(body) {
			return 0, strconv.ErrSyntax
		}
	}
	if len(body) > 1 && hasHexPrefix(body) {
		return 0, strconv.ErrSyntax
	}
	n, err := strconv.ParseFloat(s, 64)
	if err == nil && strings.IndexByte(s, '_') >= 0 {
		// Underscore separators aren't supported by AWK.
		return 0, strconv.ErrSyntax
	}
	return n, err
}

// Return value's string value, or convert to a string using given
// format if a number value. Integers are a special case and don't
// use floatFormat.
func (v value) str(floatFormat string) string {
	if v.typ == typeNum {
		return numToStr(v.n, floatFormat)
	}
	// For typeStr and typeNumStr we already have the string, for
	// typeNull v.s == "".
	return v.s
}

// Convert a number to a string using the given Go float format. Integers and
// nan/inf are special cases and don't use floatFormat.
func numToStr(n float64, floatFormat string) string {
	switch {
	// Skua: signed, as gawk prints them.
	case math.IsNaN(n):
		if math.Signbit(n) {
			return "-nan"
		}
		return "+nan"
	case math.IsInf(n, 0):
		if n < 0 {
			return "-inf"
		} else {
			return "+inf"
		}
	case n == float64(int64(n)):
		return strconv.FormatInt(int64(n), 10)
	case floatFormat == "%.6g": // speed up the default case
		return strconv.FormatFloat(n, 'g', 6, 64)
	default:
		return fmt.Sprintf(floatFormat, n)
	}
}

// Report whether converting the number n to a string actually applies CONVFMT
// or OFMT: integers and nan/inf are converted without reference to the format
// (as in other AWKs).
func needsNumFormat(n float64) bool {
	return !math.IsNaN(n) && !math.IsInf(n, 0) && n != float64(int64(n))
}

// Return value's number value, converting from string if necessary
func (v value) num() float64 {
	switch v.typ {
	case typeStr, typeNumStr:
		// Ensure string starts with a float and convert it
		return parseFloatPrefix(v.s)
	default: // typeNum, typeNull
		return v.n
	}
}

var asciiSpace = [256]uint8{'\t': 1, '\n': 1, '\v': 1, '\f': 1, '\r': 1, ' ': 1}

// Like strconv.ParseFloat, but parses at the start of string and
// allows things like "1.5foo"
func parseFloatPrefix(s string) float64 {
	// Skip whitespace at start
	i := 0
	for i < len(s) && asciiSpace[s[i]] != 0 {
		i++
	}
	start := i

	// Parse optional sign and check for NaN and Inf (Skua: only when signed,
	// as gawk).
	signed := false
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
		signed = true
	}
	if signed && i+3 <= len(s) {
		if hasNaNPrefix(s[i:]) {
			return signedNaN(s[start] == '-')
		}
		if hasInfPrefix(s[i:]) {
			if s[start] == '-' {
				return math.Inf(-1)
			}
			return math.Inf(1)
		}
	}

	// Parse mantissa: initial digit(s), optional '.', then more digits.
	// Skua: no hex, as gawk ("0x1A" converts to 0).
	gotDigit := false
	for i < len(s) && isDigit(s[i]) {
		gotDigit = true
		i++
	}
	if i < len(s) && s[i] == '.' {
		i++
	}
	for i < len(s) && isDigit(s[i]) {
		gotDigit = true
		i++
	}
	if !gotDigit {
		return 0
	}

	// Parse exponent ("1e" and similar are allowed, but ParseFloat
	// rejects them)
	end := i
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		for i < len(s) && isDigit(s[i]) {
			i++
			end = i
		}
	}

	floatStr := s[start:end]
	f, _ := strconv.ParseFloat(floatStr, 64)
	return f // Returns infinity in case of "value out of range" error
}

func hasHexPrefix(s string) bool {
	return s[0] == '0' && (s[1] == 'x' || s[1] == 'X')
}

func hasNaNPrefix(s string) bool {
	return (s[0] == 'n' || s[0] == 'N') && (s[1] == 'a' || s[1] == 'A') && (s[2] == 'n' || s[2] == 'N')
}

func hasInfPrefix(s string) bool {
	return (s[0] == 'i' || s[0] == 'I') && (s[1] == 'n' || s[1] == 'N') && (s[2] == 'f' || s[2] == 'F')
}

// signedNaN returns a NaN with the sign bit set when negative.
func signedNaN(negative bool) float64 {
	if negative {
		return math.Copysign(math.NaN(), -1)
	}
	return math.NaN()
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}
