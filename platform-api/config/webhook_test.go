/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateWebhookConfig_AutoSeedReceiverURL(t *testing.T) {
	tests := []struct {
		name        string
		receiverURL string
		wantErr     string
	}{
		{"missing", "", "requires webhook.receiver_url"},
		{"plain http", "http://platform-api.example.com/api/internal/v0.9/webhook/events", "absolute https URL"},
		{"relative", "/api/internal/v0.9/webhook/events", "absolute https URL"},
		{"no host", "https:///api/internal/v0.9/webhook/events", "absolute https URL"},
		{"unparseable", "https://exa mple.com/%zz", "absolute https URL"},
		{"https", "https://platform-api.example.com/api/internal/v0.9/webhook/events", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := &Webhook{Enabled: true, Secret: "s3cret", AutoSeedSubscribers: true, ReceiverURL: tc.receiverURL}
			err := validateWebhookConfig(w)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

// The receiver URL is only meaningful for the seed, so an operator who leaves
// auto-seed off is not forced to configure it.
func TestValidateWebhookConfig_ReceiverURLIgnoredWithoutAutoSeed(t *testing.T) {
	w := &Webhook{Enabled: true, Secret: "s3cret", ReceiverURL: "http://not-checked.example.com"}
	require.NoError(t, validateWebhookConfig(w))
}
