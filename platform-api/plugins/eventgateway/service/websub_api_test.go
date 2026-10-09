package service

import (
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// mapWebSubChannelsModelToAPI used to return a nil *map for an empty channel
// map, and the caller dereferenced it straight into api.WebSubAPI.Channels
// (a value-type map), so every Get/List/Update of a WebSub API stored without
// channels panicked.
func TestMapWebSubAPIModelToAPI_EmptyChannels(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   map[string]model.WebSubChannel
	}{
		{"nil", nil},
		{"empty", map[string]model.WebSubChannel{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &model.WebSubAPI{
				Handle:  "test",
				Name:    "test",
				Version: "v1",
				Configuration: model.WebSubAPIConfiguration{
					Channels: tc.in,
				},
			}

			got := mapWebSubAPIModelToAPI(m, &utils.APIUtil{})
			if got == nil {
				t.Fatal("expected a mapped WebSubAPI")
			}
			if got.Channels == nil {
				t.Error("expected a non-nil Channels map")
			}
			if len(got.Channels) != 0 {
				t.Errorf("expected no channels, got %d", len(got.Channels))
			}
		})
	}
}
