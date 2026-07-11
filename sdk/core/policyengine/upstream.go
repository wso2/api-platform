package policyengine

// UpstreamInfo identifies the resolved upstream target for one of an API's
// upstream slots (main or sandbox) or a named upstream definition: the Envoy
// cluster name (used for cluster_header dynamic routing) and the raw resolved
// URL. This is the shared wire shape used on both sides of the xDS route-config
// channel between gateway-controller and policy-engine.
type UpstreamInfo struct {
	ClusterName string `json:"cluster_name" yaml:"cluster_name"`
	URL         string `json:"url" yaml:"url"`
	BasePath    string `json:"base_path" yaml:"base_path"`
}

// ToMap converts UpstreamInfo into a structpb-compatible map for embedding as a
// nested object in xDS route-config metadata (structpb.Struct only accepts
// string/bool/number/nil/slice/map values, not arbitrary structs).
func (u UpstreamInfo) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"cluster_name": u.ClusterName,
		"url":          u.URL,
		"base_path":    u.BasePath,
	}
}

// UpstreamInfoFromMap decodes an UpstreamInfo from the generic map produced when
// the xDS metadata struct is unmarshalled back to JSON on the consumer side.
func UpstreamInfoFromMap(m map[string]interface{}) UpstreamInfo {
	get := func(k string) string {
		if s, ok := m[k].(string); ok {
			return s
		}
		return ""
	}
	return UpstreamInfo{
		ClusterName: get("cluster_name"),
		URL:         get("url"),
		BasePath:    get("base_path"),
	}
}
