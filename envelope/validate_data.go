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
	// Charges are a lower bound on expanded JSON bytes: values cost at least
	// one byte, and encoding does not shrink strings, keys, numbers or bytes.
	checks := dataChecks{seen: make(map[dataVisit]int), remaining: maxRecordBytes}
	if err := checks.valueErr(reflect.ValueOf(e.Data), 2); err != nil {
		return fmt.Errorf("data: %w", err)
	}
	return nil
}

type dataChecks struct {
	seen      map[dataVisit]int
	remaining int
	pointers  map[pointerVisit]pointerTarget
	path      []pointerVisit
}

type dataVisit struct {
	ptr   uintptr
	typ   reflect.Type
	len   int
	depth int
}

func (c *dataChecks) valueErr(v reflect.Value, depth int) error {
	before := c.remaining
	if err := c.charge(1); err != nil {
		return err
	}
	v, err := c.indirectDataValue(v)
	if err != nil {
		return err
	}
	if !dataContainer(v) {
		return c.dataLeafErr(v)
	}
	if depth > maxDepth {
		return fmt.Errorf("%w: limit is %d", ErrDepth, maxDepth)
	}
	return c.visitContainer(v, depth, before)
}

func (c *dataChecks) charge(n int) error {
	if n > c.remaining {
		return dataBudgetErr()
	}
	c.remaining -= n
	return nil
}

func dataBudgetErr() error {
	return fmt.Errorf(
		"%w: expanded data exceeds the %d-byte record limit",
		ErrTooLarge,
		maxRecordBytes,
	)
}

func (c *dataChecks) visitContainer(v reflect.Value, depth, before int) error {
	visit := newDataVisit(v, depth)
	if count := c.seen[visit]; count != 0 {
		return c.charge(count - 1)
	}
	if err := c.containerErr(v, depth); err != nil {
		return err
	}
	if visit.ptr != 0 {
		c.seen[visit] = before - c.remaining
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

func (c *dataChecks) containerErr(v reflect.Value, depth int) error {
	if v.Kind() == reflect.Map {
		return c.mapErr(v, depth)
	}
	return c.elementsErr(v, depth)
}

func (c *dataChecks) dataLeafErr(v reflect.Value) error {
	switch v.Kind() {
	case reflect.Invalid, reflect.Bool:
		return nil
	case reflect.Slice:
		return c.charge(v.Len())
	case reflect.Map:
		return dataMapTypeErr(v.Type())
	case reflect.String:
		return c.stringLeafErr(v)
	case reflect.Float32, reflect.Float64:
		return floatErr(v.Float())
	case reflect.Struct:
		return fmt.Errorf("use a map for data objects, got %s", v.Type())
	default:
		return dataIntErr(v)
	}
}

func (c *dataChecks) stringLeafErr(v reflect.Value) error {
	text := v.String()
	if v.Type() == reflect.TypeFor[json.Number]() {
		// The value visit already pays for the first byte of the number.
		if err := c.charge(max(0, len(text)-1)); err != nil {
			return err
		}
		return numberErr(text)
	}
	if err := c.charge(len(text)); err != nil {
		return err
	}
	return wireStringErr(text)
}

func dataIntErr(v reflect.Value) error {
	if integerKind(v.Kind()) {
		return nil
	}
	return fmt.Errorf("unsupported data value type %s", v.Type())
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

func (c *dataChecks) mapErr(v reflect.Value, depth int) error {
	keyTypeErr := dataMapTypeErr(v.Type())
	iter := v.MapRange()
	for iter.Next() {
		if err := c.dataKeyErr(iter.Key()); err != nil {
			return err
		}
		if keyTypeErr != nil {
			return keyTypeErr
		}
		if err := c.valueErr(iter.Value(), depth+1); err != nil {
			return err
		}
	}
	return keyTypeErr
}

func (c *dataChecks) dataKeyErr(key reflect.Value) error {
	key, err := c.indirectDataValue(key)
	if err != nil {
		return err
	}
	if key.Kind() != reflect.String {
		return nil
	}
	text := key.String()
	if err := c.charge(len(text)); err != nil {
		return err
	}
	if err := wireStringErr(text); err != nil {
		return fmt.Errorf("object key: %w", err)
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

func (c *dataChecks) elementsErr(v reflect.Value, depth int) error {
	for i := range v.Len() {
		if err := c.valueErr(v.Index(i), depth+1); err != nil {
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
