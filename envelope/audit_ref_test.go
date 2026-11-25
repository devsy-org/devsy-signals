package envelope_test

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/devsy-org/devsy-signals/attrkey"
	"github.com/devsy-org/devsy-signals/envelope"
)

type refCase struct {
	name string
	ref  string
	git  bool
}

func refCases() []refCase {
	return []refCase{
		{"one level", "main", true},
		{"nested", "refs/heads/topic", true},
		{"intermediate trailing dot", "feature./topic", true},
		{"at in name", "topic@main", true},
		{"empty", "", false},
		{"space", "a b", false},
		{"C0", "a\x1fb", false},
		{"DEL", "a\x7fb", false},
		{"tilde", "a~b", false},
		{"caret", "a^b", false},
		{"colon", "a:b", false},
		{"question", "a?b", false},
		{"asterisk", "a*b", false},
		{"opening bracket", "a[b", false},
		{"backslash", `a\b`, false},
		{"two dots", "a..b", false},
		{"reflog", "a@{b", false},
		{"lone at", "@", false},
		{"leading slash", "/topic", false},
		{"trailing slash", "topic/", false},
		{"double slash", "a//b", false},
		{"leading dot component", "a/.b", false},
		{"lock component", "a.lock/b", false},
		{"lock suffix", "a/b.lock", false},
		{"trailing dot", "a/b.", false},
		{"256 bytes", strings.Repeat("\u00e9", 128), true},
		{"258 bytes", strings.Repeat("\u00e9", 129), true},
	}
}

func TestValidateAuditRef(t *testing.T) {
	t.Parallel()
	for _, tc := range refCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := auditRecord(t, envelope.AuditSessionRequest)
			e.Data[attrkey.VCSRefHeadName] = tc.ref
			want := tc.git && len(tc.ref) <= 256
			if err := e.Validate(); (err == nil) != want {
				t.Errorf("Validate() = %v; want valid=%t", err, want)
			}
		})
	}
}

func TestAuditRefCasesAgainstGit(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	for _, tc := range refCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if strings.HasPrefix(tc.ref, "-") {
				t.Fatal("the Git CLI would treat this fixture as an option")
			}
			cmd := exec.CommandContext(ctx, "git", "check-ref-format", "--allow-onelevel")
			cmd.Args = append(cmd.Args, tc.ref)
			err := cmd.Run()
			var exit *exec.ExitError
			rejected := errors.As(err, &exit) && exit.ExitCode() == 1
			if (err == nil) != tc.git || (!tc.git && !rejected) {
				t.Errorf("git check-ref-format = %v; want valid=%t", err, tc.git)
			}
		})
	}
}
