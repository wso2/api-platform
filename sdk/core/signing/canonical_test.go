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

package signing

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCanonicalJSONRFC8785Example(t *testing.T) {
	// RFC 8785 §3.2.2.
	// %u stands for a JSON \u escape, kept out of the literal so editors and
	// tooling cannot unescape it.
	in := strings.ReplaceAll(`{
  "numbers": [333333333.33333329, 1E30, 4.50, 2e-3, 0.000000000000000000000000001],
  "string": "%u20ac$%u000F%u000aA'%u0042%u0022%u005c\\\"\/",
  "literals": [null, true, false]
}`, "%u", "\x5cu")
	want := `{"literals":[null,true,false],"numbers":[333333333.3333333,1e+30,4.5,0.002,1e-27],"string":"€$\u000f\nA'B\"\\\\\"/"}`

	got, err := CanonicalJSON([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestCanonicalJSONSortsByUTF16CodeUnits(t *testing.T) {
	// RFC 8785 §3.2.3: U+1F600 (a surrogate pair) sorts before U+FB33.
	in := `{"€":"Euro","\r":"CR","דּ":"Hebrew","1":"One","😀":"Smiley","\u0080":"Ctl","ö":"Latin"}`
	got, err := CanonicalJSON([]byte(in))
	if err != nil {
		t.Fatal(err)
	}

	var order []string
	dec := json.NewDecoder(strings.NewReader(string(got)))
	_, _ = dec.Token()
	for dec.More() {
		k, _ := dec.Token()
		v, _ := dec.Token()
		_ = k
		order = append(order, v.(string))
	}
	want := []string{"CR", "One", "Ctl", "Latin", "Euro", "Smiley", "Hebrew"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("order = %v, want %v", order, want)
	}
}

func TestCanonicalNumber(t *testing.T) {
	tests := map[string]string{
		"0":                      "0",
		"-0":                     "0",
		"1":                      "1",
		"-1.5":                   "-1.5",
		"4.50":                   "4.5",
		"1e21":                   "1e+21",
		"1e20":                   "100000000000000000000",
		"123456789012345680000":  "123456789012345680000",
		"0.000001":               "0.000001",
		"1e-7":                   "1e-7",
		"1.5e-7":                 "1.5e-7",
		"9007199254740992":       "9007199254740992",
		"5e-324":                 "5e-324",
		"1.7976931348623157e308": "1.7976931348623157e+308",
		"333333333.33333329":     "333333333.3333333",
	}
	for in, want := range tests {
		got, err := canonicalNumber(json.Number(in))
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if got != want {
			t.Errorf("canonicalNumber(%s) = %s, want %s", in, got, want)
		}
	}
}

func TestCanonicalJSONRejectsNonIJSON(t *testing.T) {
	for name, in := range map[string]string{
		"duplicate member": `{"a":1,"a":2}`,
		"overflow":         `{"a":1e400}`,
		"trailing data":    `{} {}`,
		"invalid UTF-8":    "{\"a\":\"\xff\"}",
		"too deep":         strings.Repeat("[", maxCanonicalDepth+2) + strings.Repeat("]", maxCanonicalDepth+2),
	} {
		if _, err := CanonicalJSON([]byte(in)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestCanonicalJSONIsIndependentOfFormatting(t *testing.T) {
	a, err := CanonicalJSON([]byte(strings.ReplaceAll(`{"b": [1, 2.0, {"y": true, "x": null}], "a": "%u00e9"}`, "%u", "\x5cu")))
	if err != nil {
		t.Fatal(err)
	}
	b, err := CanonicalJSON([]byte("{\"a\":\"é\",\n\t\"b\":[1,2,{\"x\":null,\"y\":true}]}"))
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Fatalf("%s != %s", a, b)
	}
}
