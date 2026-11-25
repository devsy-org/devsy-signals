// Command checkpublished checks the published error-code inventory against a Git base.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/devsy-org/devsy-signals/tools/internal/registry"
)

const (
	inventoryPath = "schema/errors.published.yaml"
	registryPath  = "schema/errors.yaml"
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
	paths, err := git(
		ctx,
		root,
		"ls-tree",
		"--full-tree",
		"--name-only",
		base,
		"--",
		inventoryPath,
		registryPath,
	)
	if err != nil {
		return "", err
	}
	for _, path := range []string{inventoryPath, registryPath} {
		if strings.Contains("\n"+paths, "\n"+path+"\n") {
			return path, nil
		}
	}
	return "", nil
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
