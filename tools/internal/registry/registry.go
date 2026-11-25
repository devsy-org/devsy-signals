// Package registry loads and checks schema/errors.yaml.
package registry

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	codeRE  = regexp.MustCompile(`^[a-z][a-z0-9_]*\.[a-z0-9_]+$`)
	classRE = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)
	// exitRE is the one accepted spelling of an exit value: unsigned decimal
	// without leading zeros.
	exitRE = regexp.MustCompile(`^(0|[1-9][0-9]*)$`)
)

// Entry is one registered error code.
type Entry struct {
	Code      string `yaml:"code"`
	Class     string `yaml:"class"`
	Retryable bool   `yaml:"retryable"`
	Exit      int    `yaml:"exit"`
	Summary   string `yaml:"summary"`
}

// File is the parsed errors.yaml.
type File struct {
	Classes map[string]int `yaml:"classes"`
	Codes   []Entry        `yaml:"codes"`
}

// Load reads and validates the registry at path.
func Load(path string) (File, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path comes from the tool flags
	if err != nil {
		return File{}, fmt.Errorf("read registry: %w", err)
	}
	return Parse(data)
}

// Parse decodes and validates registry bytes. Unknown fields, a missing
// retryable flag and extra YAML documents are errors.
func Parse(data []byte) (File, error) {
	var raw rawFile
	if err := decodeYAML(bytes.NewReader(data), "errors.yaml", &raw); err != nil {
		return File{}, err
	}
	f, err := raw.file()
	if err != nil {
		return File{}, err
	}
	if err := f.Validate(); err != nil {
		return File{}, err
	}
	return f, nil
}

func decodeYAML(src io.Reader, name string, dst any) error {
	dec := yaml.NewDecoder(src)
	dec.KnownFields(true)
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("decode %s: %w", name, err)
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%s must hold exactly one YAML document", name)
	}
	return nil
}

// strictBool accepts only the YAML true and false literals. yaml.v3 otherwise
// reads the YAML 1.1 words on, off, yes and no into a bool.
type strictBool bool

func (b *strictBool) UnmarshalYAML(n *yaml.Node) error {
	var v bool
	if decorated(n) || n.Kind != yaml.ScalarNode || n.Tag != "!!bool" || n.Decode(&v) != nil {
		return fmt.Errorf("line %d: retryable must be true or false", n.Line)
	}
	*b = strictBool(v)
	return nil
}

// decorated reports an explicit tag or an anchor. yaml.v3 hands a decoder the
// anchored node for an alias, so an alias to a value shows the anchor too.
func decorated(n *yaml.Node) bool {
	return n.Anchor != "" || n.Style&yaml.TaggedStyle != 0
}

// strictInt accepts only an unquoted scalar that yaml.v3 tags !!int. That
// excludes floats such as 125.9, booleans, null, quoted strings and a quoted
// value forced with an explicit tag (!!int "125"). The spelling must also
// match exitRE, one canonical form: plain unsigned decimal. yaml.v3 strips
// underscores before it decodes and reads a leading zero as octal, where YAML
// 1.2 reads decimal, and exits never need a sign or a radix, so underscores,
// leading zeros, signs and 0x, 0o and 0b prefixes are all refused (1_0, 017,
// +5, 0x7D). Zero passes here and fails the range check. Explicit tags,
// anchors and aliases are refused too; plain is set so the caller can name
// the class or code, which the node does not know.
type strictInt struct {
	value int
	plain bool
}

func (i *strictInt) UnmarshalYAML(n *yaml.Node) error {
	if decorated(n) {
		return nil
	}
	var v int
	if !plainInt(n) || n.Decode(&v) != nil {
		return fmt.Errorf("line %d: exit must be an integer", n.Line)
	}
	*i = strictInt{value: v, plain: true}
	return nil
}

const errPlainExit = "exit must be a plain integer without a tag, anchor or alias"

func plainInt(n *yaml.Node) bool {
	const quoted = yaml.DoubleQuotedStyle | yaml.SingleQuotedStyle | yaml.LiteralStyle | yaml.FoldedStyle
	if n.Kind != yaml.ScalarNode || n.Tag != "!!int" || n.Style&quoted != 0 {
		return false
	}
	return exitRE.MatchString(n.Value)
}

// exitValues converts decoded class exits. A null value decodes to nil and is
// rejected.
func exitValues(raw map[string]*strictInt) (map[string]int, error) {
	exits := make(map[string]int, len(raw))
	for _, class := range slices.Sorted(maps.Keys(raw)) {
		switch {
		case raw[class] == nil:
			return nil, fmt.Errorf("class %q must set exit to an integer", class)
		case !raw[class].plain:
			return nil, fmt.Errorf("class %q: %s", class, errPlainExit)
		}
		exits[class] = raw[class].value
	}
	return exits, nil
}

type rawEntry struct {
	Code      string      `yaml:"code"`
	Class     string      `yaml:"class"`
	Retryable *strictBool `yaml:"retryable"`
	Exit      *strictInt  `yaml:"exit"`
	Summary   string      `yaml:"summary"`
}

type rawFile struct {
	Classes map[string]*strictInt `yaml:"classes"`
	Codes   []rawEntry            `yaml:"codes"`
}

func (r rawFile) file() (File, error) {
	classes, err := exitValues(r.Classes)
	if err != nil {
		return File{}, err
	}
	f := File{Classes: classes}
	errs := []error{}
	for _, e := range r.Codes {
		entry, err := e.entry()
		if err != nil {
			errs = append(errs, err)
			continue
		}
		f.Codes = append(f.Codes, entry)
	}
	return f, errors.Join(errs...)
}

func (e rawEntry) entry() (Entry, error) {
	switch {
	case e.Retryable == nil:
		return Entry{}, fmt.Errorf("code %q must set retryable to true or false", e.Code)
	case e.Exit == nil:
		return Entry{}, fmt.Errorf("code %q must set exit to an integer", e.Code)
	case !e.Exit.plain:
		return Entry{}, fmt.Errorf("code %q: %s", e.Code, errPlainExit)
	}
	return Entry{
		Code:      e.Code,
		Class:     e.Class,
		Retryable: bool(*e.Retryable),
		Exit:      e.Exit.value,
		Summary:   e.Summary,
	}, nil
}

const (
	minExit = 1
	maxExit = 125 // shells reserve 0 and 126 and above

	// signalClass alone may use signalExit, the 128+SIGINT convention for a
	// canceled run, which falls outside the shell-safe range.
	signalClass = "canceled"
	signalExit  = 130
)

func checkClassExit(class string, exit int) error {
	switch {
	case class == signalClass && exit == signalExit:
		return nil
	case exit == signalExit:
		return fmt.Errorf("class %q exit %d is reserved for class %s", class, exit, signalClass)
	case exit < minExit || exit > maxExit:
		return fmt.Errorf(
			"class %q exit %d must be between %d and %d",
			class,
			exit,
			minExit,
			maxExit,
		)
	}
	return nil
}

// Validate checks code format, uniqueness, class membership, exit codes and
// that no code generates a Go identifier the generated package declares.
func (f File) Validate() error {
	errs := []error{}
	if len(f.Classes) == 0 {
		errs = append(errs, errors.New("classes is empty"))
	}
	for class, exit := range f.Classes {
		if !classRE.MatchString(class) {
			errs = append(errs, fmt.Errorf("class %q must be lowercase snake_case", class))
		}
		if err := checkClassExit(class, exit); err != nil {
			errs = append(errs, err)
		}
	}
	reserved := f.reservedNames()
	seen := map[string]bool{}
	for _, e := range f.Codes {
		if err := f.checkEntry(e, seen, reserved); err != nil {
			errs = append(errs, err)
		}
		seen[e.Code] = true
	}
	return errors.Join(errs...)
}

// GeneratedNames returns the top-level names the generated package declares
// besides the class and code constants. A test parses the generated file to
// keep this list complete.
func GeneratedNames() []string {
	return []string{"Class", "Code", "Info", "Lookup", "All"}
}

func (f File) reservedNames() map[string]bool {
	reserved := map[string]bool{}
	for _, name := range GeneratedNames() {
		reserved[name] = true
	}
	for class := range f.Classes {
		reserved[ConstName("class."+class)] = true
	}
	return reserved
}

func initialism(part string) (string, bool) {
	switch part {
	case "api", "dns", "http", "id", "ip", "sdk", "ssh", "tls", "uri", "url":
		return strings.ToUpper(part), true
	}
	return "", false
}

// ConstName returns the Go constant name generated for a code.
func ConstName(code string) string {
	var b strings.Builder
	for _, part := range strings.FieldsFunc(code, func(r rune) bool { return r == '.' || r == '_' }) {
		if up, ok := initialism(part); ok {
			b.WriteString(up)
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	return b.String()
}

func (f File) checkEntry(e Entry, seen, reserved map[string]bool) error {
	if !validCode(e.Code) {
		return fmt.Errorf("code %q must have the form domain.reason", e.Code)
	}
	exit, ok := f.Classes[e.Class]
	switch {
	case seen[e.Code]:
		return fmt.Errorf("code %q is listed twice", e.Code)
	case !ok:
		return fmt.Errorf("code %q has unknown class %q", e.Code, e.Class)
	case e.Exit != exit:
		return fmt.Errorf("code %q has exit %d, class %s needs %d", e.Code, e.Exit, e.Class, exit)
	case reserved[ConstName(e.Code)]:
		return fmt.Errorf("code %q generates the reserved Go name %s", e.Code, ConstName(e.Code))
	}
	return checkSummary(e.Code, e.Summary)
}

func validCode(code string) bool {
	switch code {
	case "unknown", "canceled", "timeout":
		return true
	default:
		return codeRE.MatchString(code)
	}
}
