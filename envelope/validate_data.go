package envelope

import (
	"encoding"
	"encoding/json"
	"fmt"
	"reflect"
)

func (e Envelope) checkData() error {
	if e.Kind == KindAudit || e.Data == nil {
		return nil
	}
	seen := make(dataChecks)
	if err := seen.valueErr(reflect.ValueOf(e.Data), 2); err != nil {
		return fmt.Errorf("data: %w", err)
	}
	return nil
}

type dataChecks map[dataVisit]struct{}

type dataVisit struct {
	ptr   uintptr
	typ   reflect.Type
	len   int
	depth int
}

func (seen dataChecks) valueErr(v reflect.Value, depth int) error {
	v, err := indirectDataValue(v)
	if err != nil {
		return err
	}
	if !dataContainer(v) {
		return dataLeafErr(v)
	}
	if depth > maxDepth {
		return fmt.Errorf("%w: limit is %d", ErrDepth, maxDepth)
	}
	visit := newDataVisit(v, depth)
	if _, ok := seen[visit]; ok {
		return nil
	}
	if err := seen.containerErr(v, depth); err != nil {
		return err
	}
	if visit.ptr != 0 {
		seen[visit] = struct{}{}
	}
	return nil
}

func newDataVisit(v reflect.Value, depth int) dataVisit {
	// A subtree accepted at a shallower position may exceed the limit here.
	if v.Kind() != reflect.Array {
		return dataVisit{v.Pointer(), v.Type(), v.Len(), depth}
	}
	if v.CanAddr() {
		return dataVisit{v.Addr().Pointer(), v.Type(), v.Len(), depth}
	}
	return dataVisit{}
}

func (seen dataChecks) containerErr(v reflect.Value, depth int) error {
	if v.Kind() == reflect.Map {
		return seen.mapErr(v, depth)
	}
	return seen.elementsErr(v, depth)
}

func dataLeafErr(v reflect.Value) error {
	switch v.Kind() {
	case reflect.Invalid, reflect.Slice, reflect.Bool:
		return nil
	case reflect.Map:
		return dataMapTypeErr(v.Type())
	case reflect.String:
		if v.Type() == reflect.TypeFor[json.Number]() {
			return numberErr(v.String())
		}
		return wireStringErr(v.String())
	case reflect.Float32, reflect.Float64:
		return floatErr(v.Float())
	case reflect.Struct:
		return fmt.Errorf("use a map for data objects, got %s", v.Type())
	default:
		return dataIntErr(v)
	}
}

func dataIntErr(v reflect.Value) error {
	if integerKind(v.Kind()) {
		return nil
	}
	return fmt.Errorf("unsupported data value type %s", v.Type())
}

func indirectDataValue(v reflect.Value) (reflect.Value, error) {
	var pointers map[reflect.Value]struct{}
	for v.IsValid() {
		if err := wireMarshalerErr(v); err != nil {
			return reflect.Value{}, err
		}
		switch v.Kind() {
		case reflect.Interface:
			v = v.Elem()
		case reflect.Pointer:
			if _, seen := pointers[v]; seen {
				return reflect.Value{}, fmt.Errorf("%w: cyclic pointer in data", ErrDepth)
			}
			if pointers == nil {
				pointers = make(map[reflect.Value]struct{})
			}
			pointers[v] = struct{}{}
			v = v.Elem()
		default:
			return v, nil
		}
	}
	return v, nil
}

func dataContainer(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Map:
		return !v.IsNil()
	case reflect.Slice:
		return !v.IsNil() && !byteSlice(v.Type())
	case reflect.Array:
		return true
	default:
		return false
	}
}

func byteSlice(t reflect.Type) bool {
	elem := t.Elem()
	return elem.Kind() == reflect.Uint8 && !wireMarshalerType(reflect.PointerTo(elem))
}

func (seen dataChecks) mapErr(v reflect.Value, depth int) error {
	keyTypeErr := dataMapTypeErr(v.Type())
	iter := v.MapRange()
	for iter.Next() {
		if err := dataKeyErr(iter.Key()); err != nil {
			return err
		}
		if keyTypeErr != nil {
			return keyTypeErr
		}
		if err := seen.valueErr(iter.Value(), depth+1); err != nil {
			return err
		}
	}
	return keyTypeErr
}

func dataKeyErr(key reflect.Value) error {
	key, err := indirectDataValue(key)
	if err != nil {
		return err
	}
	if key.Kind() == reflect.String {
		if err := wireStringErr(key.String()); err != nil {
			return fmt.Errorf("object key: %w", err)
		}
	}
	return nil
}

func dataMapTypeErr(t reflect.Type) error {
	kind := t.Key().Kind()
	if kind == reflect.String || integerKind(kind) {
		return nil
	}
	return fmt.Errorf("unsupported data object key type %s: use string or integer keys", t.Key())
}

func integerKind(kind reflect.Kind) bool {
	return kind >= reflect.Int && kind <= reflect.Uintptr
}

func (seen dataChecks) elementsErr(v reflect.Value, depth int) error {
	for i := range v.Len() {
		if err := seen.valueErr(v.Index(i), depth+1); err != nil {
			return fmt.Errorf("element %d: %w", i, err)
		}
	}
	return nil
}

func wireStringErr(s string) error {
	if !validWireString(s) {
		return fmt.Errorf("%w: string must be valid UTF-8 without NUL", ErrText)
	}
	return nil
}

func wireMarshalerErr(v reflect.Value) error {
	if v.IsValid() && hasWireMarshaler(v) {
		return fmt.Errorf("%w: custom marshaler %s is not allowed", ErrText, v.Type())
	}
	return nil
}

func hasWireMarshaler(v reflect.Value) bool {
	return wireMarshalerType(v.Type()) || (v.CanAddr() && wireMarshalerType(v.Addr().Type()))
}

func wireMarshalerType(t reflect.Type) bool {
	return t.Implements(reflect.TypeFor[json.Marshaler]()) ||
		t.Implements(reflect.TypeFor[encoding.TextMarshaler]())
}
