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
	if err := dataValueErr(reflect.ValueOf(e.Data), 2); err != nil {
		return fmt.Errorf("data: %w", err)
	}
	return nil
}

func dataValueErr(v reflect.Value, depth int) error {
	v, err := indirectDataValue(v)
	if err != nil {
		return err
	}
	if dataContainer(v) && depth > maxDepth {
		return fmt.Errorf("%w: limit is %d", ErrDepth, maxDepth)
	}
	switch v.Kind() {
	case reflect.Map:
		return dataMapErr(v, depth)
	case reflect.Slice, reflect.Array:
		return dataElementsErr(v, depth)
	default:
		return dataLeafErr(v)
	}
}

func dataLeafErr(v reflect.Value) error {
	switch v.Kind() {
	case reflect.String:
		return wireStringErr(v.String())
	case reflect.Struct:
		return fmt.Errorf("use a map for data objects, got %s", v.Type())
	default:
		return nil
	}
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

func dataMapErr(v reflect.Value, depth int) error {
	iter := v.MapRange()
	for iter.Next() {
		key := iter.Key()
		if err := wireMarshalerErr(key); err != nil {
			return err
		}
		if key.Kind() == reflect.String {
			if err := wireStringErr(key.String()); err != nil {
				return fmt.Errorf("object key: %w", err)
			}
		}
		if err := dataValueErr(iter.Value(), depth+1); err != nil {
			return err
		}
	}
	return nil
}

func dataElementsErr(v reflect.Value, depth int) error {
	if v.Kind() == reflect.Slice && byteSlice(v.Type()) {
		return nil
	}
	for i := range v.Len() {
		if err := dataValueErr(v.Index(i), depth+1); err != nil {
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
