package envelope

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

type recordParser struct {
	data  []byte
	pos   int
	depth int
}

type nullValue struct{}

func jsonValue(value any) any {
	if _, ok := value.(nullValue); ok {
		return nil
	}
	return value
}

// Use the standard decoder for scalars only: its structural depth limit can
// reject records below the byte limit. The parser enforces maxDepth itself, so
// recursion never goes deeper than maxDepth frames.
func parseRecord(data []byte) (map[string]any, error) {
	p := recordParser{data: data}
	if !p.take('{') {
		return nil, p.expected("a JSON object")
	}
	root, err := p.object()
	if err != nil {
		return nil, err
	}
	p.skipSpace()
	if p.pos != len(data) {
		return nil, p.expected("end of input after the JSON object")
	}
	return root, nil
}

func (p *recordParser) enter() error {
	if p.depth >= maxDepth {
		return fmt.Errorf("%w: limit is %d levels", ErrDepth, maxDepth)
	}
	p.depth++
	return nil
}

func (p *recordParser) object() (map[string]any, error) {
	if err := p.enter(); err != nil {
		return nil, err
	}
	defer func() { p.depth-- }()
	values := map[string]any{}
	if p.take('}') {
		return values, nil
	}
	for {
		name, value, err := p.member(values)
		if err != nil {
			return nil, err
		}
		values[name] = jsonValue(value)
		if p.take('}') {
			return values, nil
		}
		if !p.take(',') {
			return nil, p.expected("',' or '}'")
		}
	}
}

func (p *recordParser) member(values map[string]any) (string, any, error) {
	if !p.take('"') {
		return "", nil, p.expected("a quoted object key")
	}
	name, err := p.readString()
	if err != nil {
		return "", nil, err
	}
	if _, duplicate := values[name]; duplicate {
		return "", nil, fmt.Errorf("duplicate key %q", name)
	}
	if !p.take(':') {
		return "", nil, p.expected("':' after the object key")
	}
	value, err := p.value()
	return name, value, err
}

func (p *recordParser) array() ([]any, error) {
	if err := p.enter(); err != nil {
		return nil, err
	}
	defer func() { p.depth-- }()
	values := []any{}
	if p.take(']') {
		return values, nil
	}
	for {
		value, err := p.value()
		if err != nil {
			return nil, err
		}
		values = append(values, jsonValue(value))
		if p.take(']') {
			return values, nil
		}
		if !p.take(',') {
			return nil, p.expected("',' or ']'")
		}
	}
}

func (p *recordParser) value() (any, error) {
	p.skipSpace()
	if p.pos == len(p.data) {
		return nil, p.expected("a value")
	}
	switch p.data[p.pos] {
	case '{':
		p.pos++
		return p.object()
	case '[':
		p.pos++
		return p.array()
	case '"':
		p.pos++
		return p.readString()
	default:
		return p.scalar()
	}
}

func (p *recordParser) readString() (string, error) {
	start := p.pos - 1
	for p.pos < len(p.data) {
		switch p.data[p.pos] {
		case '\\':
			p.pos++
		case '"':
			p.pos++
			var value string
			if err := json.Unmarshal(p.data[start:p.pos], &value); err != nil {
				return "", fmt.Errorf("string at byte %d: %w", start, err)
			}
			return value, nil
		}
		p.pos++
	}
	return "", fmt.Errorf("unterminated string at byte %d", start)
}

func (p *recordParser) scalar() (any, error) {
	start := p.pos
	for p.pos < len(p.data) && strings.IndexByte(" \t\r\n,]}", p.data[p.pos]) < 0 {
		p.pos++
	}
	raw := p.data[start:p.pos]
	switch string(raw) {
	case "null":
		return nullValue{}, nil
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err != nil {
		return nil, fmt.Errorf("value at byte %d: %w", start, err)
	}
	return number, nil
}

func (p *recordParser) take(delimiter byte) bool {
	p.skipSpace()
	if p.pos == len(p.data) || p.data[p.pos] != delimiter {
		return false
	}
	p.pos++
	return true
}

func (p *recordParser) skipSpace() {
	for p.pos < len(p.data) && strings.IndexByte(" \t\r\n", p.data[p.pos]) >= 0 {
		p.pos++
	}
}

func (p *recordParser) expected(what string) error {
	return fmt.Errorf("expected %s at byte %d", what, p.pos)
}

func checkText(data []byte) error {
	if !utf8.Valid(data) {
		return fmt.Errorf("%w: invalid UTF-8", ErrText)
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return fmt.Errorf("%w: NUL byte", ErrText)
	}
	return checkEscapedText(data)
}

func checkEscapedText(data []byte) error {
	inString := false
	for i := 0; i+1 < len(data); i++ {
		if data[i] == '"' {
			inString = !inString
		}
		if !inString || data[i] != '\\' {
			continue
		}
		skip, err := checkEscape(data[i:])
		if err != nil {
			return err
		}
		i += skip - 1
	}
	return nil
}

func checkEscape(data []byte) (int, error) {
	r, ok := unicodeEscape(data)
	if !ok {
		return 2, nil
	}
	if r == 0 {
		return 0, fmt.Errorf("%w: escaped NUL", ErrText)
	}
	if !utf16.IsSurrogate(r) {
		return 6, nil
	}
	next, ok := unicodeEscape(data[6:])
	if !ok || utf16.DecodeRune(r, next) == utf8.RuneError {
		return 0, fmt.Errorf("%w: unpaired surrogate \\u%04x", ErrText, r)
	}
	return 12, nil
}

func unicodeEscape(data []byte) (rune, bool) {
	if len(data) < 6 || data[0] != '\\' || data[1] != 'u' {
		return 0, false
	}
	value, err := strconv.ParseUint(string(data[2:6]), 16, 16)
	return rune(value), err == nil
}
