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

package server

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/service"
)

func TestNewPortalPublisher(t *testing.T) {
	valid := config.APIPortalPublication{
		RequestTimeout:        5 * time.Second,
		ListingRequestTimeout: 5 * time.Second,
		ContentRequestTimeout: 10 * time.Second,
		MaxRetries:            1,
		RetryBackoff:          time.Second,
		OverallTimeout:        time.Minute,
	}
	noRequestTimeout, noListingTimeout, noContentTimeout := valid, valid, valid
	noRequestTimeout.RequestTimeout = 0
	noListingTimeout.ListingRequestTimeout = 0
	noContentTimeout.ContentRequestTimeout = 0

	tests := []struct {
		name    string
		cfg     config.APIPortalPublication
		wantErr bool
	}{
		{"valid limits", valid, false},
		{"invalid request timeout", noRequestTimeout, true},
		{"invalid listing request timeout", noListingTimeout, true},
		{"invalid content request timeout", noContentTimeout, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resetSharedHTTPClient(t)
			if err := InitSharedHTTPClient(&config.Server{}); err != nil {
				t.Fatalf("InitSharedHTTPClient: %v", err)
			}

			publisher, err := newPortalPublisher(service.NewAPIPortalAuthRegistry(nil, nil), &tc.cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
			if (err != nil) != tc.wantErr {
				t.Fatalf("want error=%v, got %v", tc.wantErr, err)
			}
			if !tc.wantErr && publisher == nil {
				t.Fatal("want a publisher")
			}
		})
	}
}
