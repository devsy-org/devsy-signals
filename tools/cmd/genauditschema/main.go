// Command genauditschema writes schema/audit-data.schema.json from the allow-list.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/devsy-org/devsy-signals/tools/internal/auditschema"
)

func main() {
	root := flag.String("root", "..", "repository root")
	flag.Parse()
	if err := run(*root); err != nil {
		fmt.Fprintln(os.Stderr, "genauditschema:", err)
		os.Exit(1)
	}
}

func run(root string) error {
	raw, err := readSource(root)
	if err != nil {
		return err
	}
	out, err := auditschema.Render(raw)
	if err != nil {
		return err
	}
	if err := os.WriteFile(
		filepath.Join(root, "schema", "audit-data.schema.json"),
		out,
		0o600,
	); err != nil {
		return fmt.Errorf("write generated audit schema: %w", err)
	}
	return nil
}

func readSource(root string) ([]byte, error) {
	dir, err := os.OpenRoot(filepath.Join(root, "schema"))
	if err != nil {
		return nil, fmt.Errorf("open schema directory: %w", err)
	}
	raw, err := fs.ReadFile(dir.FS(), "audit-allowlist.json")
	if err := errors.Join(err, dir.Close()); err != nil {
		return nil, fmt.Errorf("read audit allow-list: %w", err)
	}
	return raw, nil
}
