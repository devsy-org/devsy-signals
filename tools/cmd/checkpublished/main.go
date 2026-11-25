// Command checkpublished checks the published error-code inventory against a Git base.
package main

import (
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/devsy-org/devsy-signals/tools/internal/registry"
)

const (
	schemaDir     = "schema"
	inventoryPath = schemaDir + "/errors.published.yaml"
	registryPath  = schemaDir + "/errors.yaml"

	modeTree       = "040000"
	modeFile       = "100644"
	modeExecutable = "100755"
	modeSymlink    = "120000"
)

func main() {
	base := flag.String("base", "", "Git base ref to compare with")
	flag.Parse()
	if err := run(context.Background(), "..", *base); err != nil {
		fmt.Fprintln(os.Stderr, "checkpublished:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, root, base string) error {
	if base == "" || strings.HasPrefix(base, "-") {
		return errors.New("-base must name a Git ref or commit")
	}
	path, err := baselinePath(ctx, root, base)
	if err != nil || path == "" {
		return err
	}
	previous, err := git(ctx, root, "show", base+":"+path)
	if err != nil {
		return err
	}
	baseline, err := loadBaseline(path, previous)
	if err != nil {
		return fmt.Errorf("base %s: %w", path, err)
	}
	return checkCurrent(root, baseline)
}

func baselinePath(ctx context.Context, root, base string) (string, error) {
	dir, err := treeModes(ctx, root, base, schemaDir)
	if err != nil || len(dir) == 0 {
		return "", err
	}
	if err := checkMode(schemaDir, dir[schemaDir], modeTree); err != nil {
		return "", err
	}
	files, err := treeModes(ctx, root, base, inventoryPath, registryPath)
	if err != nil {
		return "", err
	}
	return firstBaselineFile(files)
}

// firstBaselineFile validates every schema file at base, not only the one
// used, so a link cannot hide behind a valid sibling.
func firstBaselineFile(files map[string]string) (string, error) {
	found := ""
	for _, path := range []string{inventoryPath, registryPath} {
		mode, ok := files[path]
		if !ok {
			continue
		}
		if err := checkMode(path, mode, modeFile, modeExecutable); err != nil {
			return "", err
		}
		found = cmp.Or(found, path)
	}
	return found, nil
}

// treeModes maps each listed path that exists at base to its Git mode. Git
// lists a path itself and never follows a symlink, so a link is reported
// instead of being traversed.
func treeModes(ctx context.Context, root, base string, paths ...string) (map[string]string, error) {
	args := append([]string{"ls-tree", "--full-tree", base, "--"}, paths...)
	out, err := git(ctx, root, args...)
	if err != nil {
		return nil, err
	}
	modes := map[string]string{}
	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		meta, path, ok := strings.Cut(line, "\t")
		if fields := strings.Fields(meta); ok && len(fields) > 0 {
			modes[path] = fields[0]
		}
	}
	return modes, nil
}

func checkMode(path, mode string, allowed ...string) error {
	switch {
	case slices.Contains(allowed, mode):
		return nil
	case mode == modeSymlink:
		return fmt.Errorf("base %s is a symlink; it must not be one", path)
	default:
		return fmt.Errorf("base %s has unsupported Git mode %s", path, mode)
	}
}

func loadBaseline(path, data string) (registry.Inventory, error) {
	if path == inventoryPath {
		return registry.LoadInventory(strings.NewReader(data))
	}
	f, err := registry.Parse([]byte(data))
	if err != nil {
		return registry.Inventory{}, err
	}
	return registry.InventoryFromRegistry(f), nil
}

func checkCurrent(root string, base registry.Inventory) error {
	if err := rejectSymlinks(root, schemaDir, inventoryPath, registryPath); err != nil {
		return err
	}
	file, err := os.OpenInRoot(root, inventoryPath)
	if err != nil {
		return fmt.Errorf("read inventory: %w", err)
	}
	current, err := registry.LoadInventory(file)
	if err := errors.Join(err, file.Close()); err != nil {
		return fmt.Errorf("current inventory: %w", err)
	}
	return registry.CheckAppendOnly(base, current)
}

func rejectSymlinks(root string, paths ...string) error {
	for _, path := range paths {
		info, err := os.Lstat(filepath.Join(root, path))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect %s: %w", path, err)
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%s is a symlink; it must not be one", path)
		}
	}
	return nil
}

func git(ctx context.Context, root string, args ...string) (string, error) {
	//nolint:gosec // Git gets fixed subcommands and a base argument that cannot be an option.
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", args[0], err)
	}
	return string(out), nil
}
