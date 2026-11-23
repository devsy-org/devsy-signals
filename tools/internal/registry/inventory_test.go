package registry_test

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/devsy-org/devsy-signals/tools/internal/registry"
)

const (
	sampleCode  = "sample.error"
	sampleClass = "internal"
)

type publishedCode struct {
	Code      string `yaml:"code"`
	Class     string `yaml:"class"`
	Retryable *bool  `yaml:"retryable"`
	Status    string `yaml:"status"`
}

func TestPublishedInventory(t *testing.T) {
	t.Parallel()
	f, err := registry.Load("../../../schema/errors.yaml")
	require.NoError(t, err)

	inventory, err := os.Open("../../../schema/errors.published.yaml")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, inventory.Close()) })
	require.NoError(t, checkInventory(f, inventory))
}

func TestInventoryRejectsChanges(t *testing.T) {
	t.Parallel()
	const active = "  - {code: sample.error, class: internal, retryable: false, status: active}\n"
	for name, tc := range map[string]struct {
		code      registry.Entry
		inventory string
		want      string
	}{
		"rename": {
			registry.Entry{Code: "sample.renamed", Class: sampleClass},
			active,
			`registry code "sample.renamed" is missing from the inventory`,
		},
		"reuse retired": {
			registry.Entry{Code: sampleCode, Class: sampleClass},
			strings.Replace(active, "active", "retired", 1), `retired code "sample.error" is in the registry`,
		},
		"change class": {
			registry.Entry{Code: sampleCode, Class: "config"},
			active,
			`code "sample.error" has different class or retryable values`,
		},
		"change retryable": {
			registry.Entry{Code: sampleCode, Class: sampleClass, Retryable: true},
			active,
			`code "sample.error" has different class or retryable values`,
		},
		"remove active": {registry.Entry{}, active, `active code "sample.error" is missing from the registry`},
		"duplicate": {
			registry.Entry{Code: sampleCode, Class: sampleClass},
			active + active,
			`inventory code "sample.error" is listed twice`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var f registry.File
			if tc.code.Code != "" {
				f.Codes = []registry.Entry{tc.code}
			}
			err := checkInventory(f, strings.NewReader("codes:\n"+tc.inventory))
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func checkInventory(f registry.File, src io.Reader) error {
	published, err := readInventory(src)
	if err != nil {
		return err
	}
	active := make(map[string]registry.Entry, len(f.Codes))
	for _, entry := range f.Codes {
		if _, ok := published[entry.Code]; !ok {
			return fmt.Errorf("registry code %q is missing from the inventory", entry.Code)
		}
		active[entry.Code] = entry
	}
	for _, entry := range published {
		registryEntry, exists := active[entry.Code]
		if err := entry.check(registryEntry, exists); err != nil {
			return err
		}
	}
	return nil
}

func readInventory(src io.Reader) (map[string]publishedCode, error) {
	var raw struct {
		Codes []publishedCode `yaml:"codes"`
	}
	dec := yaml.NewDecoder(src)
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode errors.published.yaml: %w", err)
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("errors.published.yaml must hold exactly one YAML document")
	}
	published := make(map[string]publishedCode, len(raw.Codes))
	for _, entry := range raw.Codes {
		if _, dup := published[entry.Code]; dup {
			return nil, fmt.Errorf("inventory code %q is listed twice", entry.Code)
		}
		if !entry.complete() {
			return nil, fmt.Errorf(
				"inventory code %q must set code, class and retryable",
				entry.Code,
			)
		}
		published[entry.Code] = entry
	}
	return published, nil
}

func (p publishedCode) complete() bool {
	return p.Code != "" && p.Class != "" && p.Retryable != nil
}

func (p publishedCode) check(entry registry.Entry, exists bool) error {
	switch {
	case p.Status == "retired":
		if exists {
			return fmt.Errorf("retired code %q is in the registry", p.Code)
		}
	case p.Status != "active":
		return fmt.Errorf("inventory code %q has invalid status %q", p.Code, p.Status)
	case !exists:
		return fmt.Errorf("active code %q is missing from the registry", p.Code)
	case p.Class != entry.Class || *p.Retryable != entry.Retryable:
		return fmt.Errorf("code %q has different class or retryable values", p.Code)
	}
	return nil
}
