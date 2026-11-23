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
)

const inventoryPath = "schema/errors.published.yaml"

func main() {
	base := flag.String("base", "", "Git base ref to compare with")
	flag.Parse()
	if err := run(context.Background(), *base); err != nil {
		fmt.Fprintln(os.Stderr, "checkpublished:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, base string) error {
	if base == "" || strings.HasPrefix(base, "-") {
		return errors.New("-base must name a Git ref or commit")
	}
	current, err := os.ReadFile("../" + inventoryPath)
	if err != nil {
		return fmt.Errorf("read inventory: %w", err)
	}
	diff, err := git(ctx, "diff", "--no-ext-diff", "--no-textconv", "--no-renames",
		"--name-status", base, "--", inventoryPath)
	if err != nil {
		return err
	}
	if diff == "" || diff == "A\t"+inventoryPath+"\n" {
		return nil
	}
	previous, err := git(ctx, "show", base+":"+inventoryPath)
	if err != nil {
		return err
	}
	return checkAppendOnly(previous, string(current))
}

func git(ctx context.Context, args ...string) (string, error) {
	//nolint:gosec // Git gets fixed subcommands and a base argument that cannot be an option.
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = ".."
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", args[0], err)
	}
	return string(out), nil
}

func checkAppendOnly(base, current string) error {
	const active = "    status: active\n"
	const retired = "    status: retired\n"
	for i, want := range strings.SplitAfter(base, "\n") {
		if want == "" {
			break
		}
		got, rest, newline := strings.Cut(current, "\n")
		if current == "" {
			return fmt.Errorf("%s:%d: existing line removed", inventoryPath, i+1)
		}
		if newline {
			got += "\n"
		}
		if got != want && (want != active || got != retired) {
			return fmt.Errorf("%s:%d: existing line changed", inventoryPath, i+1)
		}
		current = rest
	}
	return nil
}
