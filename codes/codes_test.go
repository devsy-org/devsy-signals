package codes_test

import (
	"regexp"
	"slices"
	"testing"

	"github.com/devsy-org/devsy-signals/codes"
)

var codeRE = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z0-9_]+)*$`)

func TestCodesAreTyped(t *testing.T) {
	t.Parallel()
	if got := textOf(codes.Timeout); got != "timeout" {
		t.Errorf("String() = %q", got)
	}
	if len(typedAll(codes.All())) == 0 {
		t.Fatal("no codes")
	}
	byString, okString := codes.Lookup("timeout")
	byCode, okCode := codes.Lookup(codes.Timeout)
	if !okString || !okCode || byString != byCode {
		t.Errorf("Lookup differs: %+v/%v vs %+v/%v", byString, okString, byCode, okCode)
	}
}

func textOf(c codes.Code) string { return c.String() }

func typedAll(all []codes.Code) []codes.Code { return all }

func TestLookup(t *testing.T) {
	t.Parallel()
	info, ok := codes.Lookup(codes.APIRateLimited)
	if !ok || info.Class != codes.ClassTransient || !info.Retryable || info.Exit != 75 {
		t.Fatalf("api.rate_limited = %+v, %v", info, ok)
	}
	if _, ok := codes.Lookup("no.such_code"); ok {
		t.Fatal("unregistered code found")
	}
}

func TestRegistryIsConsistent(t *testing.T) {
	t.Parallel()
	all := codes.All()
	if !slices.IsSorted(all) {
		t.Error("All is not sorted")
	}
	for _, code := range all {
		info, ok := codes.Lookup(code)
		if !ok {
			t.Fatalf("%s listed but not found", code)
		}
		if !codeRE.MatchString(code.String()) {
			t.Errorf("%s has a bad format", code)
		}
		if !info.Class.Known() || info.Class.Exit() != info.Exit {
			t.Errorf("%s: class %s does not give exit %d", code, info.Class, info.Exit)
		}
	}
}

func TestClassExit(t *testing.T) {
	t.Parallel()
	want := map[codes.Class]int{
		codes.ClassInternal:         1,
		codes.ClassUsage:            2,
		codes.ClassConfig:           2,
		codes.ClassEnvironment:      3,
		codes.ClassTransient:        75,
		codes.ClassRecoverableBuild: 79,
		codes.ClassTimeout:          124,
		codes.ClassCanceled:         130,
	}
	for class, exit := range want {
		if got := class.Exit(); got != exit {
			t.Errorf("%s exit = %d, want %d", class, got, exit)
		}
	}
	if codes.Class("nope").Known() {
		t.Error("unregistered class is known")
	}
}
