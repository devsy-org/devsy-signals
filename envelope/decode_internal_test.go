package envelope

import (
	"reflect"
	"strings"
	"testing"
)

func TestFieldReadersMatchStructTags(t *testing.T) {
	t.Parallel()
	cases := []struct {
		typ  reflect.Type
		read func(*recordFields)
		name string
	}{
		{reflect.TypeFor[Envelope](), func(f *recordFields) { readEnvelope(f) }, "Envelope"},
		{reflect.TypeFor[Actor](), func(f *recordFields) { readActor(f) }, "Actor"},
		{reflect.TypeFor[Error](), func(f *recordFields) { readError(f) }, "Error"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			checkFieldReader(t, c.typ, c.read)
		})
	}
}

func checkFieldReader(t *testing.T, typ reflect.Type, read func(*recordFields)) {
	t.Helper()
	fields := &recordFields{values: make(map[string]any)}
	count := 0
	for i := range typ.NumField() {
		name, _, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
		if name == "v" {
			continue
		}
		fields.values[name] = true
		fields.values[strings.ToUpper(name)] = false
		count++
	}
	read(fields)
	for name, variant := range fields.values {
		if variant == true {
			t.Errorf("reader did not consume %s", name)
		}
	}
	if len(fields.values) != count {
		t.Error("reader consumed a case-folded field")
	}
}
