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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// consoleRequestTimeout copies the console's own request timeout, which this repo's Go code
// cannot read.
const consoleRequestTimeout = 70 * time.Second

const minConsoleMargin = 3 * time.Second

func validAPIPortalPublication() APIPortalPublication {
	return APIPortalPublication{
		RequestTimeout:        5 * time.Second,
		ListingRequestTimeout: 15 * time.Second,
		MaxRetries:            1,
		RetryBackoff:          time.Second,
		OverallTimeout:        65 * time.Second,
	}
}

func TestAPIPortalPublication_WorstCases(t *testing.T) {
	c := validAPIPortalPublication()
	assert.Equal(t, 11*time.Second, c.RequestWorstCase(), "two 5s attempts and one 1s wait")
	assert.Equal(t, 31*time.Second, c.ListingRequestWorstCase(), "two 15s attempts and one 1s wait")
	assert.Equal(t, 64*time.Second, c.PublishWorstCase(), "three light calls and one push")

	c.MaxRetries = 0
	assert.Equal(t, 5*time.Second, c.RequestWorstCase(), "no retries means no wait")
	assert.Equal(t, 15*time.Second, c.ListingRequestWorstCase())
}

func TestAPIPortalPublication_PlanListingBudget(t *testing.T) {
	c := validAPIPortalPublication()
	assert.Equal(t, c.RequestWorstCase(), c.PlanListingBudget(), "the whole listing gets the time of one call")
	assert.Equal(t, c.PlanListingBudget()+2*c.RequestWorstCase()+c.ListingRequestWorstCase(), c.PublishWorstCase(),
		"the publish worst case counts the listing at its budget, however many pages it has")

	c.MaxRetries = 0
	assert.Equal(t, c.RequestTimeout, c.PlanListingBudget(), "no retries means a single attempt's time")
}

func TestLoadConfig_APIPortalPublication_DefaultsFitTheConsole(t *testing.T) {
	cfg, err := loadWithKeys(t, "")
	require.NoError(t, err)

	c := cfg.APIPortalPublication
	assert.GreaterOrEqual(t, c.OverallTimeout, c.PublishWorstCase())
	assert.Less(t, c.OverallTimeout, cfg.Listeners.Timeouts.Write)
	assert.LessOrEqual(t, c.OverallTimeout+minConsoleMargin, consoleRequestTimeout,
		"the deadline must end before the console stops waiting")
}

func TestLoadConfig_APIPortalPublication_Override(t *testing.T) {
	cfg, err := loadWithKeys(t, `
[platform_api.api_portal_publication]
request_timeout      = "4s"
listing_request_timeout = "10s"
max_retries          = 1
retry_backoff        = "500ms"
overall_timeout    = "50s"
`)
	require.NoError(t, err)
	assert.Equal(t, APIPortalPublication{
		RequestTimeout:        4 * time.Second,
		ListingRequestTimeout: 10 * time.Second,
		MaxRetries:            1,
		RetryBackoff:          500 * time.Millisecond,
		OverallTimeout:        50 * time.Second,
	}, cfg.APIPortalPublication)
}

func TestLoadConfig_APIPortalPublication_RejectsUnfitCombination(t *testing.T) {
	_, err := loadWithKeys(t, `
[platform_api.api_portal_publication]
listing_request_timeout = "20s"
`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "api_portal_publication.overall_timeout")
}

func TestValidateAPIPortalPublicationConfig(t *testing.T) {
	const writeTimeout = 120 * time.Second

	tests := []struct {
		name    string
		mutate  func(*APIPortalPublication)
		write   time.Duration
		wantErr string
	}{
		{name: "defaults pass", mutate: func(*APIPortalPublication) {}, write: writeTimeout},
		{name: "no retries needs a shorter deadline", mutate: func(c *APIPortalPublication) {
			c.MaxRetries = 0
			c.OverallTimeout = 30 * time.Second
		}, write: writeTimeout},
		{name: "deadline exactly at the worst case passes", mutate: func(c *APIPortalPublication) {
			c.OverallTimeout = c.PublishWorstCase()
		}, write: writeTimeout},
		{name: "write timeout disabled skips the ceiling", mutate: func(c *APIPortalPublication) {
			c.OverallTimeout = 10 * time.Minute
		}, write: 0},

		{name: "zero attempt timeout", mutate: func(c *APIPortalPublication) { c.RequestTimeout = 0 }, write: writeTimeout,
			wantErr: "api_portal_publication.request_timeout must be greater than zero"},
		{name: "negative attempt timeout", mutate: func(c *APIPortalPublication) { c.RequestTimeout = -time.Second }, write: writeTimeout,
			wantErr: "api_portal_publication.request_timeout must be greater than zero"},
		{name: "zero push attempt timeout", mutate: func(c *APIPortalPublication) { c.ListingRequestTimeout = 0 }, write: writeTimeout,
			wantErr: "api_portal_publication.listing_request_timeout must be greater than zero"},
		{name: "zero operation timeout", mutate: func(c *APIPortalPublication) { c.OverallTimeout = 0 }, write: writeTimeout,
			wantErr: "api_portal_publication.overall_timeout must be greater than zero"},
		{name: "negative retries", mutate: func(c *APIPortalPublication) { c.MaxRetries = -1 }, write: writeTimeout,
			wantErr: "api_portal_publication.max_retries must not be negative"},
		{name: "negative backoff", mutate: func(c *APIPortalPublication) { c.RetryBackoff = -time.Second }, write: writeTimeout,
			wantErr: "api_portal_publication.retry_backoff must not be negative"},
		{name: "push shorter than light", mutate: func(c *APIPortalPublication) { c.ListingRequestTimeout = time.Second }, write: writeTimeout,
			wantErr: "must not be shorter than api_portal_publication.request_timeout"},
		{name: "deadline below the worst case", mutate: func(c *APIPortalPublication) {
			c.OverallTimeout = c.PublishWorstCase() - time.Nanosecond
		}, write: writeTimeout, wantErr: "shorter than the worst case of a publish"},
		{name: "deadline reaches the write timeout", mutate: func(c *APIPortalPublication) {
			c.OverallTimeout = writeTimeout
		}, write: writeTimeout, wantErr: "must be shorter than server.timeouts.write"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := validAPIPortalPublication()
			tc.mutate(&c)
			err := validateAPIPortalPublicationConfig(&c, tc.write)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}
