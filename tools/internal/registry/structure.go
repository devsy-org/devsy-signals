package registry

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

const (
	errFeature = "YAML anchors, aliases, tags and merge keys are not allowed in the registry"

	maxStructureErrors = 20
)

// checkStructure walks the whole YAML tree once and refuses every feature the
// registry files do not need: anchors, aliases, explicit tags, merge keys and
// any mapping key that is not a plain string. Banning them keeps the file a
// plain data tree, so the decoder cannot receive a field name, a class or a
// value through a merge or an alias.
//
// The bare non-specific tag "!" needs a source scan. yaml.v3 resolves "! 1" to
// !!int with no TaggedStyle flag, so the Node cannot tell it from a plain
// scalar. A node's Line and Column point at its first property, so the scan
// reads the source from there. data must be UTF-8 without a byte order mark,
// which keeps Column aligned with runes.
func checkStructure(data []byte) error {
	var doc yaml.Node
	if yaml.Unmarshal(data, &doc) != nil || len(doc.Content) == 0 {
		return nil
	}
	c := structureChecker{src: splitSource(string(data))}
	c.walk(doc.Content[0], fieldCtx{}, true)
	return errors.Join(c.errs...)
}

// fieldCtx names the entry or class a node belongs to, and the field it sits
// under, so an error can say which code or class to fix.
type fieldCtx struct{ label, field string }

type structureChecker struct {
	src  [][]rune
	errs []error
}

func (c *structureChecker) walk(n *yaml.Node, ctx fieldCtx, top bool) {
	if !c.checkNode(n, ctx) {
		return
	}
	if n.Kind == yaml.MappingNode {
		c.walkMapping(n, ctx, top)
		return
	}
	for _, child := range n.Content {
		c.walk(child, ctx, false)
	}
}

func (c *structureChecker) walkMapping(n *yaml.Node, ctx fieldCtx, top bool) {
	label := ownerLabel(n)
	if label == "" {
		label = ctx.label
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, val := n.Content[i], n.Content[i+1]
		if !c.checkKey(key, fieldCtx{label, key.Value}) {
			continue
		}
		if top && key.Value == "classes" && val.Kind == yaml.MappingNode {
			c.walkClasses(val)
			continue
		}
		c.walk(val, fieldCtx{label, key.Value}, false)
	}
}

func (c *structureChecker) walkClasses(n *yaml.Node) {
	if !c.checkNode(n, fieldCtx{}) {
		return
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, val := n.Content[i], n.Content[i+1]
		ctx := fieldCtx{fmt.Sprintf("class %q", key.Value), "exit"}
		if c.checkKey(key, ctx) {
			c.walk(val, ctx, false)
		}
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

// checkNode reports false when the node carries a banned feature.
func (c *structureChecker) checkNode(n *yaml.Node, ctx fieldCtx) bool {
	switch {
	case n.Anchor != "":
		c.fail(n, ctx, errFeature+" (found an anchor)")
	case n.Kind == yaml.AliasNode:
		c.fail(n, ctx, errFeature+" (found an alias)")
	case n.Style&yaml.TaggedStyle != 0:
		c.fail(n, ctx, errFeature+" (found an explicit tag)")
	case c.hasBareTag(position{n.Line - 1, n.Column - 1}):
		c.fail(n, ctx, errFeature+" (found a bare ! tag)")
	default:
		return true
	}
	return false
}

func (c *structureChecker) checkKey(key *yaml.Node, ctx fieldCtx) bool {
	if !c.checkNode(key, ctx) {
		return false
	}
	switch {
	case key.Tag == "!!merge":
		c.fail(key, ctx, errFeature+" (found a merge key)")
	case key.Kind != yaml.ScalarNode || key.Tag != "!!str":
		c.fail(key, ctx, "mapping keys must be plain strings")
	default:
		return true
	}
	return false
}

func (c *structureChecker) fail(n *yaml.Node, ctx fieldCtx, msg string) {
	if len(c.errs) >= maxStructureErrors {
		return
	}
	label := ""
	if ctx.label != "" {
		label = ctx.label + ": "
	}
	switch ctx.field {
	case "exit":
		msg = errPlainExit + ": " + msg
	case "retryable":
		msg = errRetryable + ": " + msg
	}
	c.errs = append(c.errs, fmt.Errorf("%sline %d: %s", label, n.Line, msg))
}

type position struct{ line, col int }

// hasBareTag reads the properties that start at a node: tags and anchors,
// separated by blanks, line breaks and comments.
func (c *structureChecker) hasBareTag(pos position) bool {
	for c.at(pos) == '!' || c.at(pos) == '&' {
		tok, next := c.token(pos)
		if string(tok) == "!" {
			return true
		}
		pos = c.skipSpace(next)
	}
	return false
}

func (c *structureChecker) at(p position) rune {
	if p.line < 0 || p.line >= len(c.src) || p.col < 0 || p.col >= len(c.src[p.line]) {
		return 0
	}
	return c.src[p.line][p.col]
}

func (c *structureChecker) token(p position) ([]rune, position) {
	row := c.src[p.line]
	end := p.col
	for end < len(row) && !unicode.IsSpace(row[end]) && !strings.ContainsRune(",]}", row[end]) {
		end++
	}
	return row[p.col:end], position{p.line, end}
}

func (c *structureChecker) skipSpace(p position) position {
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
