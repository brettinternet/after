// Package compare compares stored finite observations, never project code.
package compare

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

const maxJSON = 1 << 20
const maxChanges = 2048

// Change uses RFC 6901 pointers. An absent value is omitted, unlike JSON null.
// Values retain exact number tokens; equality uses exact decimal values.
type Change struct {
	Path   string          `json:"path"`
	Kind   string          `json:"kind"`
	Before json.RawMessage `json:"before,omitempty"`
	After  json.RawMessage `json:"after,omitempty"`
}

// JSON rejects ambiguous/over-budget input instead of silently losing evidence.
func JSON(before, after []byte) ([]Change, error) {
	a, err := parse(before)
	if err != nil {
		return nil, err
	}
	b, err := parse(after)
	if err != nil {
		return nil, err
	}
	changes := []Change{}
	if err := walk(a, b, "", &changes); err != nil {
		return nil, err
	}
	return changes, nil
}

func parse(raw []byte) (any, error) {
	if len(raw) > maxJSON || !utf8.Valid(raw) || !validSurrogates(raw) {
		return nil, errors.New("JSON size or Unicode limit")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	nodes := 0
	v, err := value(d, 0, &nodes)
	if err != nil {
		return nil, err
	}
	if _, err = d.Token(); err != io.EOF {
		return nil, errors.New("expected exactly one JSON value")
	}
	return v, nil
}

func value(d *json.Decoder, depth int, nodes *int) (any, error) {
	*nodes++
	if depth > 64 || *nodes > 32768 {
		return nil, errors.New("JSON structure budget")
	}
	t, err := d.Token()
	if err != nil {
		return nil, errors.New("invalid JSON")
	}
	switch v := t.(type) {
	case json.Delim:
		switch v {
		case '{':
			m := map[string]any{}
			for d.More() {
				key, e := d.Token()
				if e != nil {
					return nil, errors.New("invalid JSON key")
				}
				k, ok := key.(string)
				if !ok {
					return nil, errors.New("invalid JSON key")
				}
				if _, ok = m[k]; ok {
					return nil, errors.New("duplicate JSON key")
				}
				item, e := value(d, depth+1, nodes)
				if e != nil {
					return nil, e
				}
				m[k] = item
			}
			if end, e := d.Token(); e != nil || end != json.Delim('}') {
				return nil, errors.New("invalid JSON object")
			}
			return m, nil
		case '[':
			a := []any{}
			for d.More() {
				item, e := value(d, depth+1, nodes)
				if e != nil {
					return nil, e
				}
				a = append(a, item)
			}
			if end, e := d.Token(); e != nil || end != json.Delim(']') {
				return nil, errors.New("invalid JSON array")
			}
			return a, nil
		}
		return nil, errors.New("unexpected JSON delimiter")
	case json.Number:
		if _, err := decimal(v); err != nil {
			return nil, err
		}
	}
	return t, nil
}

// Normalize a decimal without float conversion or exponent-sized allocations.
func decimal(n json.Number) (string, error) {
	s := string(n)
	if len(s) > 1024 {
		return "", errors.New("number token budget")
	}
	sign := ""
	if strings.HasPrefix(s, "-") {
		sign = "-"
		s = s[1:]
	}
	mant, exp, has := strings.Cut(strings.ToLower(s), "e")
	e := int64(0)
	if has {
		var err error
		e, err = strconv.ParseInt(exp, 10, 32)
		if err != nil {
			return "", errors.New("number exponent budget")
		}
	}
	if i := strings.IndexByte(mant, '.'); i >= 0 {
		e -= int64(len(mant) - i - 1)
		mant = mant[:i] + mant[i+1:]
	}
	mant = strings.TrimLeft(mant, "0")
	if mant == "" {
		return "0", nil
	}
	trimmed := strings.TrimRight(mant, "0")
	e += int64(len(mant) - len(trimmed))
	return sign + trimmed + "e" + strconv.FormatInt(e, 10), nil
}

func validSurrogates(b []byte) bool {
	// JSON syntax is checked separately. Escaped backslashes are skipped, so a
	// literal "\\uD800" is not mistaken for a surrogate escape.
	for i := 0; i < len(b); i++ {
		if b[i] != '\\' {
			continue
		}
		i++
		if i >= len(b) {
			return false
		}
		if b[i] != 'u' {
			continue
		}
		if i+4 >= len(b) {
			return false
		}
		n, e := strconv.ParseUint(string(b[i+1:i+5]), 16, 16)
		if e != nil {
			return false
		}
		i += 4
		if n >= 0xdc00 && n <= 0xdfff {
			return false
		}
		if n >= 0xd800 && n <= 0xdbff {
			if i+6 >= len(b) || b[i+1] != '\\' || b[i+2] != 'u' {
				return false
			}
			low, e := strconv.ParseUint(string(b[i+3:i+7]), 16, 16)
			if e != nil || low < 0xdc00 || low > 0xdfff {
				return false
			}
			i += 6
		}
	}
	return true
}

func pointer(path, key string) string {
	return path + "/" + strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
}
func add(out *[]Change, path, kind string, a, b any) error {
	if len(*out) >= maxChanges {
		return errors.New("difference budget exceeded")
	}
	c := Change{Path: path, Kind: kind}
	if kind != "added" {
		c.Before, _ = json.Marshal(a)
	}
	if kind != "removed" {
		c.After, _ = json.Marshal(b)
	}
	*out = append(*out, c)
	return nil
}
func walk(a, b any, path string, out *[]Change) error {
	switch x := a.(type) {
	case map[string]any:
		if y, ok := b.(map[string]any); ok {
			keys := make([]string, 0, len(x)+len(y))
			for k := range x {
				keys = append(keys, k)
			}
			for k := range y {
				if _, ok := x[k]; !ok {
					keys = append(keys, k)
				}
			}
			slices.Sort(keys)
			for _, k := range keys {
				av, ao := x[k]
				bv, bo := y[k]
				p := pointer(path, k)
				var err error
				switch {
				case !ao:
					err = add(out, p, "added", nil, bv)
				case !bo:
					err = add(out, p, "removed", av, nil)
				default:
					err = walk(av, bv, p, out)
				}
				if err != nil {
					return err
				}
			}
			return nil
		}
	case []any:
		if y, ok := b.([]any); ok {
			for i := 0; i < max(len(x), len(y)); i++ {
				p := pointer(path, strconv.Itoa(i))
				var err error
				switch {
				case i >= len(x):
					err = add(out, p, "added", nil, y[i])
				case i >= len(y):
					err = add(out, p, "removed", x[i], nil)
				default:
					err = walk(x[i], y[i], p, out)
				}
				if err != nil {
					return err
				}
			}
			return nil
		}
	case json.Number:
		if y, ok := b.(json.Number); ok {
			nx, _ := decimal(x)
			ny, _ := decimal(y)
			if nx == ny {
				return nil
			}
		}
	case string:
		if y, ok := b.(string); ok && x == y {
			return nil
		}
	case bool:
		if y, ok := b.(bool); ok && x == y {
			return nil
		}
	case nil:
		if b == nil {
			return nil
		}
	}
	return add(out, path, "changed", a, b)
}
