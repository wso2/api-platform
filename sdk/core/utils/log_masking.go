/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package utils

import (
	"log/slog"
	"reflect"
)

// MaskedLogValue replaces every policy parameter value written to a log.
const MaskedLogValue = "***"

// MaskedParams wraps policy parameters, or any value derived from them, for
// logging. The logged form keeps map keys, nesting and list lengths, so a log
// still shows which parameters were set, but every value is replaced with
// MaskedLogValue. Parameter values can be URLs, tokens, API keys, passwords or
// numbers, and none of them may reach a log.
//
// Masking runs only when the record is emitted, so a Debug call on a hot path
// costs nothing while Debug is off:
//
//	slog.Debug("processing", "params", utils.MaskedParams(params))
func MaskedParams(v interface{}) slog.LogValuer {
	return maskedParams{v: v}
}

type maskedParams struct {
	v interface{}
}

func (m maskedParams) LogValue() slog.Value {
	return slog.AnyValue(MaskLogValue(m.v))
}

// MaskLogValue returns a copy of v with map keys and list lengths kept and
// every other value replaced with MaskedLogValue. Maps with string keys become
// map[string]interface{}, slices and arrays become []interface{}, and a nil
// value stays nil so an unset parameter remains visible. v is never modified.
//
// A map, slice or pointer that contains itself, directly or through other
// containers, is written as MaskedLogValue where it refers back to itself, so
// masking always terminates. A container reached twice without forming a cycle
// is masked in full each time.
func MaskLogValue(v interface{}) interface{} {
	return maskValue(reflect.ValueOf(v), make(map[containerRef]struct{}))
}

// containerRef identifies a map, slice or pointer on the current traversal
// path. A slice is identified by its backing array and length, so a shorter
// sub-slice of the same array is not mistaken for a cycle.
type containerRef struct {
	kind reflect.Kind
	ptr  uintptr
	len  int
}

func maskValue(rv reflect.Value, path map[containerRef]struct{}) interface{} {
	switch rv.Kind() {
	case reflect.Invalid:
		return nil
	case reflect.Interface:
		if rv.IsNil() {
			return nil
		}
		return maskValue(rv.Elem(), path)
	case reflect.Pointer:
		if rv.IsNil() {
			return nil
		}
		return enter(containerRef{kind: reflect.Pointer, ptr: rv.Pointer()}, path, func() interface{} {
			return maskValue(rv.Elem(), path)
		})
	case reflect.Map:
		if rv.IsNil() {
			return nil
		}
		if rv.Type().Key().Kind() != reflect.String {
			return MaskedLogValue
		}
		return enter(containerRef{kind: reflect.Map, ptr: rv.Pointer()}, path, func() interface{} {
			out := make(map[string]interface{}, rv.Len())
			iter := rv.MapRange()
			for iter.Next() {
				out[iter.Key().String()] = maskValue(iter.Value(), path)
			}
			return out
		})
	case reflect.Slice:
		if rv.IsNil() {
			return nil
		}
		// A byte slice is a single value (a key, a certificate), not a list.
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return MaskedLogValue
		}
		return enter(containerRef{kind: reflect.Slice, ptr: rv.Pointer(), len: rv.Len()}, path, func() interface{} {
			return maskElements(rv, path)
		})
	case reflect.Array:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return MaskedLogValue
		}
		// An array is held by value, so it cannot contain itself.
		return maskElements(rv, path)
	default:
		return MaskedLogValue
	}
}

// enter masks a container through mask unless the container is already on the
// traversal path, in which case it is a cycle and is written as MaskedLogValue.
func enter(ref containerRef, path map[containerRef]struct{}, mask func() interface{}) interface{} {
	if _, onPath := path[ref]; onPath {
		return MaskedLogValue
	}
	path[ref] = struct{}{}
	defer delete(path, ref)
	return mask()
}

func maskElements(rv reflect.Value, path map[containerRef]struct{}) []interface{} {
	out := make([]interface{}, rv.Len())
	for i := range out {
		out[i] = maskValue(rv.Index(i), path)
	}
	return out
}
