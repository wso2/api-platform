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

package policyengine

import "testing"

func TestUpstreamInfoRoundTrip(t *testing.T) {
	info := UpstreamInfo{
		ClusterName: "cluster_https_backend_example_com",
		URL:         "https://backend.example.com",
		BasePath:    "/v1",
	}

	got := UpstreamInfoFromMap(info.ToMap())

	if got != info {
		t.Errorf("round trip mismatch: got %+v, want %+v", got, info)
	}
}

func TestUpstreamInfoFromMapMissingFields(t *testing.T) {
	got := UpstreamInfoFromMap(map[string]interface{}{"cluster_name": "c1"})
	want := UpstreamInfo{ClusterName: "c1"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestUpstreamInfoFromMapWrongType(t *testing.T) {
	got := UpstreamInfoFromMap(map[string]interface{}{"cluster_name": 42})
	want := UpstreamInfo{}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
