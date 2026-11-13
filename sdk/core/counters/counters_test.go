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

package counters

import (
	"testing"
	"time"
)

// fixed is a Service whose totals are set by the test.
type fixed struct {
	adds  map[Key]float64
	total float64
	ok    bool
}

func (f *fixed) Add(key Key, delta float64) { f.adds[key] += delta }
func (f *fixed) Total(Key) (float64, bool)  { return f.total, f.ok }

func TestLocalServiceWhenNothingIsInstalled(t *testing.T) {
	Install(nil)
	key := Key{Namespace: "test", ID: "a", Window: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}
	other := Key{Namespace: "test", ID: "a", Window: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)}

	Add(key, 1.5)
	Add(key, 2)

	if got, ok := Total(key); got != 3.5 || !ok {
		t.Fatalf("Total = %v, %v; want 3.5, true", got, ok)
	}
	if got, _ := Total(other); got != 0 {
		t.Fatalf("a new window starts at zero, got %v", got)
	}
	if Shared() {
		t.Fatal("the local service is not shared")
	}
}

func TestInstalledServiceIsUsed(t *testing.T) {
	svc := &fixed{adds: map[Key]float64{}, total: 7, ok: false}
	Install(svc)
	defer Install(nil)
	key := Key{Namespace: "test", ID: "b"}

	Add(key, 1)

	if svc.adds[key] != 1 {
		t.Fatal("Add did not reach the installed service")
	}
	if got, ok := Total(key); got != 7 || ok {
		t.Fatalf("Total = %v, %v; want the installed service's 7, false", got, ok)
	}
	if !Shared() {
		t.Fatal("an installed service is shared")
	}
}
