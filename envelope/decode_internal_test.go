package envelope

import (
	"reflect"
	"strings"
	"testing"
)

func TestFieldPredicatesMatchStructTags(t *testing.T) {
	t.Parallel()
	cases := []struct {
		typ  reflect.Type
		is   func(string) bool
		name string
	}{
		{reflect.TypeFor[Envelope](), isEnvelopeField, "Envelope"},
		{reflect.TypeFor[Actor](), isActorField, "Actor"},
		{reflect.TypeFor[Error](), isErrorField, "Error"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			for i := range c.typ.NumField() {
				name, _, _ := strings.Cut(c.typ.Field(i).Tag.Get("json"), ",")
				if !c.is(name) {
					t.Errorf("%s is a field of %s but the predicate rejects it", name, c.name)
				}
				if c.is(strings.ToUpper(name)) || c.is(name+"x") {
					t.Errorf("predicate accepts a variant of %s", name)
				}
			}
		})
	}
}
