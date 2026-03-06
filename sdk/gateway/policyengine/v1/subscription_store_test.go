package policyenginev1

import "testing"

func TestSubscriptionStore_ReplaceAllAndIsActive(t *testing.T) {
	store := NewSubscriptionStore()

	subs := []SubscriptionData{
		{APIId: "api-1", ApplicationId: "app-1", Status: "ACTIVE"},
		{APIId: "api-1", ApplicationId: "app-2", Status: "INACTIVE"},
		{APIId: "api-2", ApplicationId: "app-1", Status: "ACTIVE"},
		// Invalid entries should be ignored
		{APIId: "", ApplicationId: "no-api", Status: "ACTIVE"},
		{APIId: "api-3", ApplicationId: "", Status: "ACTIVE"},
	}

	store.ReplaceAll(subs)

	// Ensure only valid entries were stored (3 total: 2 for api-1, 1 for api-2).
	store.mu.RLock()
	if len(store.data) != 2 {
		t.Fatalf("expected 2 APIs in store, got %d", len(store.data))
	}
	if len(store.data["api-1"]) != 2 {
		t.Fatalf("expected 2 subscriptions for api-1, got %d", len(store.data["api-1"]))
	}
	if len(store.data["api-2"]) != 1 {
		t.Fatalf("expected 1 subscription for api-2, got %d", len(store.data["api-2"]))
	}
	store.mu.RUnlock()

	// Invalid entries (empty APIId or ApplicationId) should not be present/active.
	if store.IsActive("", "no-api") {
		t.Fatalf("expected IsActive(\"\", \"no-api\") to be false for invalid APIId")
	}
	if store.IsActive("api-3", "") {
		t.Fatalf("expected IsActive(\"api-3\", \"\") to be false for invalid ApplicationId")
	}

	tests := []struct {
		name          string
		apiID         string
		appID         string
		expectedAlive bool
	}{
		{
			name:          "active subscription",
			apiID:         "api-1",
			appID:         "app-1",
			expectedAlive: true,
		},
		{
			name:          "inactive subscription",
			apiID:         "api-1",
			appID:         "app-2",
			expectedAlive: false,
		},
		{
			name:          "missing subscription",
			apiID:         "api-1",
			appID:         "unknown",
			expectedAlive: false,
		},
		{
			name:          "different api same app",
			apiID:         "api-2",
			appID:         "app-1",
			expectedAlive: true,
		},
		{
			name:          "empty ids",
			apiID:         "",
			appID:         "",
			expectedAlive: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := store.IsActive(tt.apiID, tt.appID)
			if got != tt.expectedAlive {
				t.Fatalf("IsActive(%q,%q) = %v, want %v", tt.apiID, tt.appID, got, tt.expectedAlive)
			}
		})
	}
}

