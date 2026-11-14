/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package counterproto

import (
	"testing"
	"time"
)

func TestCodecRoundTrip(t *testing.T) {
	window := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	in := &SyncRequest{
		Source: "engine-1", ReportID: "engine-1:7",
		Adds: []Add{{Key: KeyFor("budget", "a", window), Delta: 0.00065}},
		Keys: []Key{KeyFor("budget", "a", window), KeyFor("budget", "b", time.Time{})},
	}
	data, err := Codec{}.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	out := &SyncRequest{}
	if err := (Codec{}).Unmarshal(data, out); err != nil {
		t.Fatal(err)
	}
	if out.ReportID != in.ReportID || len(out.Adds) != 1 || out.Adds[0].Delta != 0.00065 || len(out.Keys) != 2 {
		t.Fatalf("round trip lost data: %+v", out)
	}
	if got := out.Keys[0].WindowTime(); !got.Equal(window) {
		t.Fatalf("window %v, want %v", got, window)
	}
	if !out.Keys[1].WindowTime().IsZero() {
		t.Fatal("a key without a window decodes to the zero time")
	}
}

func TestCodecRejectsOtherTypes(t *testing.T) {
	if _, err := (Codec{}).Marshal("text"); err == nil {
		t.Fatal("encoded a type that is not a counterproto message")
	}
	if err := (Codec{}).Unmarshal([]byte(`{}`), new(string)); err == nil {
		t.Fatal("decoded into a type that is not a counterproto message")
	}
	if FullSyncMethod != "/wso2.apip.counters.v1.Counters/Sync" {
		t.Fatalf("method path %s", FullSyncMethod)
	}
}
