package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/constants"
)

func TestSetLocation(t *testing.T) {
	tests := []struct {
		name     string
		segments []string
		want     string
	}{
		{
			name:     "single resource",
			segments: []string{"rest-apis", "my-api"},
			want:     constants.APIBasePath + "/rest-apis/my-api",
		},
		{
			name:     "nested resource",
			segments: []string{"gateways", "gw-1", "tokens", "550e8400-e29b-41d4-a716-446655440000"},
			want:     constants.APIBasePath + "/gateways/gw-1/tokens/550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:     "segment needing escaping",
			segments: []string{"projects", "a b/c"},
			want:     constants.APIBasePath + "/projects/a%20b%2Fc",
		},
		{
			name:     "empty segment suppresses header",
			segments: []string{"projects", ""},
			want:     "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			setLocation(w, tt.segments...)
			if got := w.Header().Get("Location"); got != tt.want {
				t.Errorf("Location = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStrOrEmpty(t *testing.T) {
	if got := strOrEmpty(nil); got != "" {
		t.Errorf("strOrEmpty(nil) = %q, want empty", got)
	}
	s := "id-1"
	if got := strOrEmpty(&s); got != "id-1" {
		t.Errorf("strOrEmpty(&s) = %q, want %q", got, s)
	}
}
