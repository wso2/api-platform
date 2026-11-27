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
func MaskLogValue(v interface{}) interface{} {
	switch t := v.(type) {
	case nil:
		return nil
	case map[string]interface{}:
		if t == nil {
			return nil
		}
		out := make(map[string]interface{}, len(t))
		for k, e := range t {
			out[k] = MaskLogValue(e)
		}
		return out
	case []interface{}:
		if t == nil {
			return nil
		}
		out := make([]interface{}, len(t))
		for i, e := range t {
			out[i] = MaskLogValue(e)
		}
		return out
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Interface:
		if rv.IsNil() {
			return nil
		}
		return MaskLogValue(rv.Elem().Interface())
	case reflect.Map:
		if rv.IsNil() {
			return nil
		}
		if rv.Type().Key().Kind() != reflect.String {
			return MaskedLogValue
		}
		out := make(map[string]interface{}, rv.Len())
		iter := rv.MapRange()
		for iter.Next() {
			out[iter.Key().String()] = MaskLogValue(iter.Value().Interface())
		}
		return out
	case reflect.Slice:
		if rv.IsNil() {
			return nil
		}
		// A byte slice is a single value (a key, a certificate), not a list.
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return MaskedLogValue
		}
		fallthrough
	case reflect.Array:
		out := make([]interface{}, rv.Len())
		for i := range out {
			out[i] = MaskLogValue(rv.Index(i).Interface())
		}
		return out
	default:
		return MaskedLogValue
	}
}
