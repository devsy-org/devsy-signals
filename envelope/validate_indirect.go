package envelope

import (
	"fmt"
	"reflect"
)

type pointerVisit struct {
	addr uintptr
	typ  reflect.Type
}

type pointerTarget struct {
	value reflect.Value
	// A pending entry marks a pointer on the current resolution path.
	ready bool
}

func (c *dataChecks) indirectDataValue(v reflect.Value) (reflect.Value, error) {
	for v.IsValid() {
		if err := wireMarshalerErr(v); err != nil {
			return reflect.Value{}, err
		}
		switch v.Kind() {
		case reflect.Interface:
			v = v.Elem()
		case reflect.Pointer:
			target, cached, err := c.followPointer(v)
			if err != nil {
				return reflect.Value{}, err
			}
			if cached {
				return c.rememberTarget(target), nil
			}
			v = target
		default:
			return c.rememberTarget(v), nil
		}
	}
	return c.rememberTarget(v), nil
}

func (c *dataChecks) followPointer(v reflect.Value) (reflect.Value, bool, error) {
	key := pointerVisit{v.Pointer(), v.Type()}
	if target, ok := c.pointers[key]; ok {
		if !target.ready {
			return reflect.Value{}, false, fmt.Errorf("%w: cyclic pointer in data", ErrDepth)
		}
		return target.value, true, nil
	}
	if c.pointers == nil {
		c.pointers = make(map[pointerVisit]pointerTarget)
	}
	c.pointers[key] = pointerTarget{}
	c.path = append(c.path, key)
	return v.Elem(), false, nil
}

func (c *dataChecks) rememberTarget(v reflect.Value) reflect.Value {
	for _, key := range c.path {
		c.pointers[key] = pointerTarget{value: v, ready: true}
	}
	c.path = c.path[:0]
	return v
}
