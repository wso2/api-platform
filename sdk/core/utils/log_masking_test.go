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
	"bytes"
	"encoding/json"
	"log/slog"
	"reflect"
	"strings"
	"testing"
)

const (
	testJWT    = "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyIn0.c2lnbmF0dXJl"
	testAPIKey = "apk_7f3c9a1e5b2d4f60"
	testURL    = "https://user:pass@idp.internal.example.com/oauth2/token?x=1"
	testSecret = "redis-s3cret-password"
)

func sampleParams() map[string]interface{} {
	return map[string]interface{}{
		"tokenEndpoint": testURL,
		"apiKey":        testAPIKey,
		"token":         testJWT,
		"limit":         1000,
		"costScale":     0.25,
		"enabled":       true,
		"unset":         nil,
		"redis": map[string]interface{}{
			"host":     "redis.internal",
			"port":     int64(6379),
			"password": testSecret,
		},
		"quotas": []interface{}{
			map[string]interface{}{"limit": 10, "duration": "1m"},
			map[string]interface{}{"limit": 20, "duration": "1h"},
		},
		"headers":    map[string]string{"Authorization": "Bearer " + testJWT},
		"issuers":    []string{"https://issuer.example.com"},
		"certPEM":    []byte("-----BEGIN CERTIFICATE-----"),
		"numericKey": map[int]string{1: testAPIKey},
	}
}

func TestMaskLogValue_MasksEveryValueAndKeepsShape(t *testing.T) {
	got := MaskLogValue(sampleParams())

	want := map[string]interface{}{
		"tokenEndpoint": MaskedLogValue,
		"apiKey":        MaskedLogValue,
		"token":         MaskedLogValue,
		"limit":         MaskedLogValue,
		"costScale":     MaskedLogValue,
		"enabled":       MaskedLogValue,
		"unset":         nil,
		"redis": map[string]interface{}{
			"host":     MaskedLogValue,
			"port":     MaskedLogValue,
			"password": MaskedLogValue,
		},
		"quotas": []interface{}{
			map[string]interface{}{"limit": MaskedLogValue, "duration": MaskedLogValue},
			map[string]interface{}{"limit": MaskedLogValue, "duration": MaskedLogValue},
		},
		"headers":    map[string]interface{}{"Authorization": MaskedLogValue},
		"issuers":    []interface{}{MaskedLogValue},
		"certPEM":    MaskedLogValue,
		"numericKey": MaskedLogValue,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MaskLogValue() =\n%#v\nwant\n%#v", got, want)
	}
}

func TestMaskLogValue_Scalars(t *testing.T) {
	str := testAPIKey
	var nilMap map[string]interface{}
	var nilPtr *string
	tests := []struct {
		name string
		in   interface{}
		want interface{}
	}{
		{"nil", nil, nil},
		{"string", testJWT, MaskedLogValue},
		{"empty string", "", MaskedLogValue},
		{"int", 42, MaskedLogValue},
		{"float", 3.5, MaskedLogValue},
		{"bool", false, MaskedLogValue},
		{"pointer to string", &str, MaskedLogValue},
		{"nil pointer", nilPtr, nil},
		{"nil map", nilMap, nil},
		{"struct", struct{ Secret string }{testSecret}, MaskedLogValue},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MaskLogValue(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("MaskLogValue(%v) = %#v, want %#v", tt.in, got, tt.want)
			}
		})
	}
}

func TestMaskLogValue_DoesNotModifyInput(t *testing.T) {
	params := sampleParams()
	before := sampleParams()
	_ = MaskLogValue(params)
	if !reflect.DeepEqual(params, before) {
		t.Fatal("MaskLogValue modified its input")
	}
}

func TestMaskedParams_NoValueReachesHandlerOutput(t *testing.T) {
	secrets := []string{testJWT, testAPIKey, testURL, "idp.internal.example.com", testSecret,
		"redis.internal", "6379", "1000", "0.25", "1m", "1h", "issuer.example.com", "BEGIN CERTIFICATE"}

	handlers := map[string]func(*bytes.Buffer) slog.Handler{
		"json": func(b *bytes.Buffer) slog.Handler {
			return slog.NewJSONHandler(b, &slog.HandlerOptions{Level: slog.LevelDebug})
		},
		"text": func(b *bytes.Buffer) slog.Handler {
			return slog.NewTextHandler(b, &slog.HandlerOptions{Level: slog.LevelDebug})
		},
	}
	for name, newHandler := range handlers {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			slog.New(newHandler(&buf)).Debug("policy params", "params", MaskedParams(sampleParams()))
			out := buf.String()

			for _, s := range secrets {
				if strings.Contains(out, s) {
					t.Errorf("log output contains parameter value %q: %s", s, out)
				}
			}
			for _, key := range []string{"tokenEndpoint", "apiKey", "redis", "password", "quotas", "duration"} {
				if !strings.Contains(out, key) {
					t.Errorf("log output is missing parameter key %q: %s", key, out)
				}
			}
		})
	}
}

func TestMaskedParams_JSONShape(t *testing.T) {
	var buf bytes.Buffer
	slog.New(slog.NewJSONHandler(&buf, nil)).Info("m", "params", MaskedParams(map[string]interface{}{
		"redis":  map[string]interface{}{"password": testSecret},
		"quotas": []interface{}{map[string]interface{}{"limit": 5}},
	}))

	var record struct {
		Params map[string]interface{} `json:"params"`
	}
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("unmarshal log record: %v: %s", err, buf.String())
	}
	want := map[string]interface{}{
		"redis":  map[string]interface{}{"password": MaskedLogValue},
		"quotas": []interface{}{map[string]interface{}{"limit": MaskedLogValue}},
	}
	if !reflect.DeepEqual(record.Params, want) {
		t.Fatalf("params = %#v, want %#v", record.Params, want)
	}
}

func TestMaskedParams_NotEvaluatedWhenLevelDisabled(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	logger.Debug("hidden", "params", MaskedParams(sampleParams()))
	if buf.Len() != 0 {
		t.Fatalf("debug record written at info level: %s", buf.String())
	}
}
