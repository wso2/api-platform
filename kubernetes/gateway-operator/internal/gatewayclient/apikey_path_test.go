package gatewayclient

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildAPIKeysPath_AllParents(t *testing.T) {
	cases := []struct {
		name       string
		parentKind string
		parent     string
		key        string
		want       string
		wantErr    bool
	}{
		{
			name: "rest api with handle", parentKind: ApiKeyParentKindRestApi,
			parent: "petstore", key: "k1",
			want: ManagementAPIBasePath + "/rest-apis/petstore/api-keys/k1",
		},
		{
			name: "llm provider collection", parentKind: ApiKeyParentKindLlmProvider,
			parent: "openai", key: "",
			want: ManagementAPIBasePath + "/llm-providers/openai/api-keys",
		},
		{
			name: "llm proxy", parentKind: ApiKeyParentKindLlmProxy,
			parent: "openai-proxy", key: "k1",
			want: ManagementAPIBasePath + "/llm-proxies/openai-proxy/api-keys/k1",
		},
		{
			name: "url-encoded handle", parentKind: ApiKeyParentKindRestApi,
			parent: "pet store", key: "ke y",
			want: ManagementAPIBasePath + "/rest-apis/pet%20store/api-keys/ke%20y",
		},
		{
			name: "unsupported kind", parentKind: "Unknown",
			parent: "x", key: "y", wantErr: true,
		},
		{
			name: "missing parent", parentKind: ApiKeyParentKindRestApi,
			parent: "", key: "k1", wantErr: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := BuildAPIKeysPath(c.parentKind, c.parent, c.key)
			if c.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, c.want, got)
		})
	}
}
