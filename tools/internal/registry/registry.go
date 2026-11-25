// Package registry loads and checks schema/errors.yaml.
package registry

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	codeRE  = regexp.MustCompile(`^[a-z][a-z0-9_]*\.[a-z0-9_]+$`)
	classRE = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)
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

type rawEntry struct {
	Code      string `yaml:"code"`
	Class     string `yaml:"class"`
	Retryable *bool  `yaml:"retryable"`
	Exit      int    `yaml:"exit"`
	Summary   string `yaml:"summary"`
}

type rawFile struct {
	Classes map[string]int `yaml:"classes"`
	Codes   []rawEntry     `yaml:"codes"`
}

func (r rawFile) file() (File, error) {
	f := File{Classes: r.Classes}
	errs := []error{}
	for _, e := range r.Codes {
		if e.Retryable == nil {
			errs = append(errs, fmt.Errorf("code %q must set retryable to true or false", e.Code))
			continue
		}
		f.Codes = append(f.Codes, Entry{
			Code: e.Code, Class: e.Class, Retryable: *e.Retryable, Exit: e.Exit, Summary: e.Summary,
		})
	}
	return f, errors.Join(errs...)
}

// Validate checks code format, uniqueness, class membership, exit codes and
// that no code generates a Go identifier the generated package declares.
func (f File) Validate() error {
	errs := []error{}
	if len(f.Classes) == 0 {
		errs = append(errs, errors.New("classes is empty"))
	}
	for class := range f.Classes {
		if !classRE.MatchString(class) {
			errs = append(errs, fmt.Errorf("class %q must be lowercase snake_case", class))
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

func (f File) reservedNames() map[string]bool {
	reserved := map[string]bool{"Class": true, "Info": true, "Lookup": true, "All": true}
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
	case strings.TrimSpace(e.Summary) == "":
		return fmt.Errorf("code %q summary must not be blank", e.Code)
	case strings.ContainsAny(e.Summary, "\r\n\v\f\u0085\u2028\u2029"):
		return fmt.Errorf("code %q summary must be a single line", e.Code)
	}
	return nil
}

func validCode(code string) bool {
	switch code {
	case "unknown", "canceled", "timeout":
		return true
	default:
		return codeRE.MatchString(code)
	}
}
