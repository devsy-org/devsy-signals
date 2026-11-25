package registry

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// checkBareTags rejects the bare non-specific tag "!" on exit and retryable
// values. yaml.v3 resolves "! 1" to !!int and "! false" to !!bool with no
// TaggedStyle flag, so the Node tree cannot tell them from plain scalars. The
// node's Line and Column point at its first property (tag or anchor), so this
// reads the source from there. It covers the class exits, every exit and
// retryable key in the file (which includes anchored mappings that a merge key
// reuses), block and flow style, and tags that follow an anchor.
func checkBareTags(data []byte) error {
	var doc yaml.Node
	if yaml.Unmarshal(data, &doc) != nil || len(doc.Content) == 0 {
		return nil
	}
	// yaml.v3 does not count a leading byte order mark in Column.
	c := tagChecker{src: splitSource(strings.TrimPrefix(string(data), "\ufeff"))}
	c.walkMapping(doc.Content[0], true)
	return errors.Join(c.errs...)
}

type tagChecker struct {
	src  [][]rune
	errs []error
}

func (c *tagChecker) walk(n *yaml.Node) {
	if n.Kind == yaml.SequenceNode {
		for _, child := range n.Content {
			c.walk(child)
		}
		return
	}
	c.walkMapping(n, false)
}

func (c *tagChecker) walkMapping(n *yaml.Node, top bool) {
	if n.Kind != yaml.MappingNode {
		return
	}
	label := ownerLabel(n)
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, val := n.Content[i], n.Content[i+1]
		switch {
		case top && key.Value == "classes":
			c.checkClasses(val)
		case key.Value == "exit":
			c.check(val, label, errPlainExit)
		case key.Value == "retryable":
			c.check(val, label, errRetryable)
		default:
			c.walk(val)
		}
	}
}

func (c *tagChecker) checkClasses(n *yaml.Node) {
	if n.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		c.check(n.Content[i+1], fmt.Sprintf("class %q", n.Content[i].Value), errPlainExit)
	}
}

func ownerLabel(n *yaml.Node) string {
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == "code" && n.Content[i+1].Kind == yaml.ScalarNode {
			return fmt.Sprintf("code %q", n.Content[i+1].Value)
		}
	}
	return ""
}

func (c *tagChecker) check(n *yaml.Node, label, msg string) {
	if n.Kind != yaml.ScalarNode || !c.hasBareTag(position{n.Line - 1, n.Column - 1}) {
		return
	}
	if label != "" {
		label += ": "
	}
	c.errs = append(c.errs, fmt.Errorf("%sline %d: %s", label, n.Line, msg))
}

type position struct{ line, col int }

// hasBareTag walks the properties that start at a node: tags and anchors,
// separated by blanks, line breaks and comments.
func (c *tagChecker) hasBareTag(pos position) bool {
	for c.at(pos) == '!' || c.at(pos) == '&' {
		tok, next := c.token(pos)
		if string(tok) == "!" {
			return true
		}
		pos = c.skipSpace(next)
	}
	return false
}

func (c *tagChecker) at(p position) rune {
	if p.line < 0 || p.line >= len(c.src) || p.col < 0 || p.col >= len(c.src[p.line]) {
		return 0
	}
	return c.src[p.line][p.col]
}

func (c *tagChecker) token(p position) ([]rune, position) {
	row := c.src[p.line]
	end := p.col
	for end < len(row) && !unicode.IsSpace(row[end]) && !strings.ContainsRune(",]}", row[end]) {
		end++
	}
	return row[p.col:end], position{p.line, end}
}

func (c *tagChecker) skipSpace(p position) position {
	for p.line < len(c.src) {
		row := c.src[p.line]
		for p.col < len(row) && unicode.IsSpace(row[p.col]) {
			p.col++
		}
		if p.col < len(row) && !isComment(row, p.col) {
			return p
		}
		p = position{p.line + 1, 0}
	}
	return p
}

func isComment(row []rune, col int) bool {
	return row[col] == '#' && (col == 0 || unicode.IsSpace(row[col-1]))
}

// splitSource splits at every line break yaml.v3 counts, so Line and Column
// from a node index straight into the result.
func splitSource(text string) [][]rune {
	breaks := strings.NewReplacer(
		"\r\n", "\n", "\r", "\n", "\u0085", "\n", "\u2028", "\n", "\u2029", "\n")
	parts := strings.Split(breaks.Replace(text), "\n")
	lines := make([][]rune, len(parts))
	for i, part := range parts {
		lines[i] = []rune(part)
	}
	return lines
}
