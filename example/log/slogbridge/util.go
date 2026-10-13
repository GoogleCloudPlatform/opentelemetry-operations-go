// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Code copied from opentelemetry-go-contrib/bridges/otelzap/convert.go
// See https://github.com/open-telemetry/opentelemetry-go-contrib/blob/main/bridges/otelzap/convert.go
package main

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

// convertValue converts various types to attribute.Value.
func ConvertValue(v any) attribute.Value {
	// Handling the most common types without reflect is a small perf win.
	switch val := v.(type) {
	case bool:
		return attribute.BoolValue(val)
	case string:
		return attribute.StringValue(val)
	case int:
		return attribute.Int64Value(int64(val))
	case int8:
		return attribute.Int64Value(int64(val))
	case int16:
		return attribute.Int64Value(int64(val))
	case int32:
		return attribute.Int64Value(int64(val))
	case int64:
		return attribute.Int64Value(val)
	case uint:
		return convertUintValue(uint64(val))
	case uint8:
		return attribute.Int64Value(int64(val))
	case uint16:
		return attribute.Int64Value(int64(val))
	case uint32:
		return attribute.Int64Value(int64(val))
	case uint64:
		return convertUintValue(val)
	case uintptr:
		return convertUintValue(uint64(val))
	case float32:
		return attribute.Float64Value(float64(val))
	case float64:
		return attribute.Float64Value(val)
	case time.Duration:
		return attribute.Int64Value(val.Nanoseconds())
	case complex64:
		r := attribute.Float64("r", real(complex128(val)))
		i := attribute.Float64("i", imag(complex128(val)))
		return attribute.MapValue(r, i)
	case complex128:
		r := attribute.Float64("r", real(val))
		i := attribute.Float64("i", imag(val))
		return attribute.MapValue(r, i)
	case time.Time:
		return attribute.Int64Value(val.UnixNano())
	case []byte:
		return attribute.ByteSliceValue(val)
	case error:
		return attribute.StringValue(val.Error())
	}

	t := reflect.TypeOf(v)
	if t == nil {
		return attribute.Value{}
	}
	val := reflect.ValueOf(v)
	switch t.Kind() {
	case reflect.Struct:
		return attribute.StringValue(fmt.Sprintf("%+v", v))
	case reflect.Slice, reflect.Array:
		items := make([]attribute.Value, 0, val.Len())
		for i := 0; i < val.Len(); i++ {
			items = append(items, ConvertValue(val.Index(i).Interface()))
		}
		return attribute.SliceValue(items...)
	case reflect.Map:
		kvs := make([]attribute.KeyValue, 0, val.Len())
		for _, k := range val.MapKeys() {
			var key string
			switch k.Kind() {
			case reflect.String:
				key = k.String()
			default:
				key = fmt.Sprintf("%+v", k.Interface())
			}
			kvs = append(kvs, attribute.KeyValue{
				Key:   attribute.Key(key),
				Value: ConvertValue(val.MapIndex(k).Interface()),
			})
		}
		return attribute.MapValue(kvs...)
	case reflect.Pointer, reflect.Interface:
		if val.IsNil() {
			return attribute.Value{}
		}
		return ConvertValue(val.Elem().Interface())
	}

	// Try to handle this as gracefully as possible.
	//
	// Don't panic here. it is preferable to have user's open issue
	// asking why their attributes have a "unhandled: " prefix than
	// say that their code is panicking.
	return attribute.StringValue(fmt.Sprintf("unhandled: (%s) %+v", t, v))
}

// convertUintValue converts a uint64 to an attribute.Value.
// If the value is too large to fit in an int64, it is converted to a string.
func convertUintValue(v uint64) attribute.Value {
	if v > math.MaxInt64 {
		return attribute.StringValue(strconv.FormatUint(v, 10))
	}
	return attribute.Int64Value(int64(v))
}
