// Command gencodes writes codes/codes_gen.go and ts/src/codes.ts from schema/errors.yaml.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/devsy-org/devsy-signals/tools/internal/gen"
	"github.com/devsy-org/devsy-signals/tools/internal/registry"
)

func main() {
	root := flag.String("root", "..", "repository root")
	flag.Parse()
	if err := run(*root); err != nil {
		fmt.Fprintln(os.Stderr, "gencodes:", err)
		os.Exit(1)
	}
}

func run(root string) error {
	f, err := registry.Load(filepath.Join(root, "schema", "errors.yaml"))
	if err != nil {
		return err
	}
	goSrc, tsSrc, err := gen.Render(f)
	if err != nil {
		return err
	}
	for path, data := range map[string][]byte{
		filepath.Join(root, "codes", "codes_gen.go"): goSrc,
		filepath.Join(root, "ts", "src", "codes.ts"): tsSrc,
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return fmt.Errorf("create directory: %w", err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			return fmt.Errorf("write generated file: %w", err)
		}
	}
	return nil
}
