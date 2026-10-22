// Skua: gawk extensions that coding agents use in one-liners: match() with an
// array, gensub(), asort(), asorti(), systime(), strftime() and
// PROCINFO["sorted_in"]. Semantics follow the gawk manual ("String Functions",
// "Time Functions", "Controlling Array Traversal").

package interp

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/benhoyt/goawk/internal/compiler"
	"github.com/benhoyt/goawk/internal/resolver"
)

// matchArray is match(s, re, arr): like match(), and arr[0] is the matched
// text, arr[n] the n-th parenthesized group, with arr[n, "start"] and
// arr[n, "length"] (unmatched groups are left out). arr is cleared first.
func (p *interp) matchArray(s, regex string, scope resolver.Scope, index int) (value, error) {
	re, err := p.compileRegex(regex)
	if err != nil {
		return null(), err
	}
	array := make(map[string]value)
	p.arrays[p.arrayIndex(scope, index)] = array
	loc := re.FindStringSubmatchIndex(s)
	if loc == nil {
		p.matchStart = num(0)
		p.matchLength = num(-1)
		return p.matchStart, nil
	}
	pos := func(byteIndex int) int {
		if p.chars {
			return utf8.RuneCountInString(s[:byteIndex]) + 1
		}
		return byteIndex + 1
	}
	length := func(from, to int) int {
		if p.chars {
			return utf8.RuneCountInString(s[from:to])
		}
		return to - from
	}
	for g := 0; 2*g+1 < len(loc); g++ {
		from, to := loc[2*g], loc[2*g+1]
		if from < 0 {
			continue
		}
		key := strconv.Itoa(g)
		array[key] = numStr(s[from:to])
		array[key+p.subscriptSep+"start"] = num(float64(pos(from)))
		array[key+p.subscriptSep+"length"] = num(float64(length(from, to)))
	}
	p.matchStart = num(float64(pos(loc[0])))
	p.matchLength = num(float64(length(loc[0], loc[1])))
	return p.matchStart, nil
}

// gensub is gensub(re, repl, how, target): returns target with the matches of
// re replaced (all of them if how starts with "g" or "G", otherwise only the
// how-th one). In repl, "\\0" and "&" are the matched text, "\\1".."\\9" the
// groups, "\\&" a literal "&" and "\\\\" a backslash.
func (p *interp) gensub(regex, repl string, how value, target string) (string, error) {
	re, err := p.compileRegex(regex)
	if err != nil {
		return "", err
	}
	global := false
	which := 1
	if h := p.toString(how); strings.HasPrefix(h, "g") || strings.HasPrefix(h, "G") {
		global = true
	} else if n := int(how.num()); n > 0 {
		which = n
	} else {
		fmt.Fprintf(p.errorOutput, "warning: gensub: third argument %q treated as 1\n", h)
	}
	var out strings.Builder
	last := 0
	count := 0
	for _, m := range re.FindAllStringSubmatchIndex(target, -1) {
		count++
		if !global && count != which {
			continue
		}
		out.WriteString(target[last:m[0]])
		for i := 0; i < len(repl); i++ {
			c := repl[i]
			switch {
			case c == '&':
				out.WriteString(target[m[0]:m[1]])
			case c == '\\' && i+1 < len(repl):
				i++
				d := repl[i]
				switch {
				case d >= '0' && d <= '9':
					g := int(d - '0')
					if 2*g+1 < len(m) && m[2*g] >= 0 {
						out.WriteString(target[m[2*g]:m[2*g+1]])
					}
				case d == '&':
					out.WriteByte('&')
				case d == '\\':
					out.WriteByte('\\')
				default:
					out.WriteByte('\\')
					out.WriteByte(d)
				}
			default:
				out.WriteByte(c)
			}
		}
		last = m[1]
		if !global {
			break
		}
	}
	out.WriteString(target[last:])
	return out.String(), nil
}

// asort sorts src's values (asorti: its indices) into dst[1..n], and returns
// n. Numbers sort before strings, numbers numerically, strings by bytes (gawk's
// default "@val_type_asc" / "@ind_str_asc" orders). dst may be src.
func (p *interp) asort(src, dst map[string]value, indices bool) int {
	items := make([]value, 0, len(src))
	if indices {
		keys := make([]string, 0, len(src))
		for k := range src {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			items = append(items, str(k))
		}
	} else {
		for _, v := range src {
			items = append(items, v)
		}
		sort.SliceStable(items, func(i, j int) bool {
			ni, iStr := items[i].isTrueStr()
			nj, jStr := items[j].isTrueStr()
			switch {
			case !iStr && !jStr:
				return ni < nj
			case iStr != jStr:
				return !iStr // numbers first
			default:
				return p.toString(items[i]) < p.toString(items[j])
			}
		})
	}
	for k := range dst {
		delete(dst, k)
	}
	for i, v := range items {
		dst[strconv.Itoa(i+1)] = v
	}
	return len(items)
}

// forInStep runs one iteration of a for-in loop body with the loop variable
// set to index. stop reports a break.
func (p *interp) forInStep(varScope resolver.Scope, varIndex int, index string, loopCode []compiler.Opcode) (stop bool, err error) {
	switch varScope {
	case resolver.Global:
		p.globals[varIndex] = str(index)
	case resolver.Local:
		p.frame[varIndex] = str(index)
	default: // resolver.Special
		err := p.setSpecial(varIndex, str(index))
		if err != nil {
			return false, err
		}
	}
	err = p.execute(loopCode)
	if err == errBreak {
		return true, nil
	}
	return false, err
}

// sortedInKeys returns array's indices in the order PROCINFO["sorted_in"]
// names (gawk's "@ind_str_asc", "@val_num_desc", ...), or nil to iterate in
// any order: when the program never uses PROCINFO, it is unset, "@unsorted",
// or a user function name (not supported).
func (p *interp) sortedInKeys(array map[string]value) []string {
	procinfo, ok := p.arrayIndexes["PROCINFO"]
	if !ok {
		return nil
	}
	order, ok := p.array(resolver.Global, procinfo)["sorted_in"]
	if !ok {
		return nil
	}
	mode := p.toString(order)
	desc := strings.HasSuffix(mode, "_desc")
	var less func(a, b string) bool
	switch strings.TrimSuffix(strings.TrimSuffix(mode, "_asc"), "_desc") {
	case "@ind_str":
		less = func(a, b string) bool { return a < b }
	case "@ind_num":
		less = func(a, b string) bool {
			na, nb := parseFloatPrefix(a), parseFloatPrefix(b)
			if na != nb {
				return na < nb
			}
			return a < b
		}
	case "@val_str":
		less = func(a, b string) bool {
			va, vb := p.toString(array[a]), p.toString(array[b])
			if va != vb {
				return va < vb
			}
			return a < b
		}
	case "@val_num":
		less = func(a, b string) bool {
			va, vb := array[a].num(), array[b].num()
			if va != vb {
				return va < vb
			}
			return a < b
		}
	case "@val_type":
		less = func(a, b string) bool {
			na, aStr := array[a].isTrueStr()
			nb, bStr := array[b].isTrueStr()
			switch {
			case aStr != bStr:
				return !aStr // numbers first
			case !aStr && na != nb:
				return na < nb
			case aStr && p.toString(array[a]) != p.toString(array[b]):
				return p.toString(array[a]) < p.toString(array[b])
			}
			return a < b
		}
	default:
		return nil
	}
	keys := make([]string, 0, len(array))
	for k := range array {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if desc {
			return less(keys[j], keys[i])
		}
		return less(keys[i], keys[j])
	})
	return keys
}

// strftime is strftime([format [, timestamp [, utc]]]) with gawk's default
// format, using C strftime conversions.
func (p *interp) strftime(args []value) string {
	format := "%a %b %e %H:%M:%S %Z %Y"
	t := time.Now()
	if len(args) > 0 {
		format = p.toString(args[0])
	}
	if len(args) > 1 {
		t = time.Unix(int64(args[1].num()), 0)
	}
	if len(args) > 2 && args[2].boolean() {
		t = t.UTC()
	} else {
		t = t.Local()
	}
	return formatTime(format, t)
}

func formatTime(format string, t time.Time) string {
	var b strings.Builder
	for i := 0; i < len(format); i++ {
		c := format[i]
		if c != '%' || i+1 >= len(format) {
			b.WriteByte(c)
			continue
		}
		i++
		// Skip the E and O modifiers, as glibc does in the C locale.
		if (format[i] == 'E' || format[i] == 'O') && i+1 < len(format) {
			i++
		}
		writeTimeConv(&b, format[i], t)
	}
	return b.String()
}

func writeTimeConv(b *strings.Builder, c byte, t time.Time) {
	hour12 := t.Hour() % 12
	if hour12 == 0 {
		hour12 = 12
	}
	isoYear, isoWeek := t.ISOWeek()
	switch c {
	case 'a':
		b.WriteString(t.Format("Mon"))
	case 'A':
		b.WriteString(t.Format("Monday"))
	case 'b', 'h':
		b.WriteString(t.Format("Jan"))
	case 'B':
		b.WriteString(t.Format("January"))
	case 'c':
		b.WriteString(formatTime("%a %b %e %H:%M:%S %Y", t))
	case 'C':
		fmt.Fprintf(b, "%02d", t.Year()/100)
	case 'd':
		fmt.Fprintf(b, "%02d", t.Day())
	case 'D':
		b.WriteString(formatTime("%m/%d/%y", t))
	case 'e':
		fmt.Fprintf(b, "%2d", t.Day())
	case 'F':
		b.WriteString(formatTime("%Y-%m-%d", t))
	case 'g':
		fmt.Fprintf(b, "%02d", isoYear%100)
	case 'G':
		fmt.Fprintf(b, "%d", isoYear)
	case 'H':
		fmt.Fprintf(b, "%02d", t.Hour())
	case 'I':
		fmt.Fprintf(b, "%02d", hour12)
	case 'j':
		fmt.Fprintf(b, "%03d", t.YearDay())
	case 'k':
		fmt.Fprintf(b, "%2d", t.Hour())
	case 'l':
		fmt.Fprintf(b, "%2d", hour12)
	case 'm':
		fmt.Fprintf(b, "%02d", int(t.Month()))
	case 'M':
		fmt.Fprintf(b, "%02d", t.Minute())
	case 'n':
		b.WriteByte('\n')
	case 'p':
		b.WriteString(t.Format("PM"))
	case 'r':
		b.WriteString(formatTime("%I:%M:%S %p", t))
	case 'R':
		b.WriteString(formatTime("%H:%M", t))
	case 's':
		fmt.Fprintf(b, "%d", t.Unix())
	case 'S':
		fmt.Fprintf(b, "%02d", t.Second())
	case 't':
		b.WriteByte('\t')
	case 'T':
		b.WriteString(formatTime("%H:%M:%S", t))
	case 'u':
		wd := int(t.Weekday())
		if wd == 0 {
			wd = 7
		}
		fmt.Fprintf(b, "%d", wd)
	case 'U':
		fmt.Fprintf(b, "%02d", (t.YearDay()+6-int(t.Weekday()))/7)
	case 'V':
		fmt.Fprintf(b, "%02d", isoWeek)
	case 'w':
		fmt.Fprintf(b, "%d", int(t.Weekday()))
	case 'W':
		fmt.Fprintf(b, "%02d", (t.YearDay()+6-(int(t.Weekday())+6)%7)/7)
	case 'x':
		b.WriteString(formatTime("%m/%d/%y", t))
	case 'X':
		b.WriteString(formatTime("%H:%M:%S", t))
	case 'y':
		fmt.Fprintf(b, "%02d", t.Year()%100)
	case 'Y':
		fmt.Fprintf(b, "%d", t.Year())
	case 'z':
		b.WriteString(t.Format("-0700"))
	case 'Z':
		b.WriteString(t.Format("MST"))
	case '%':
		b.WriteByte('%')
	default:
		b.WriteByte('%')
		b.WriteByte(c)
	}
}
