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
