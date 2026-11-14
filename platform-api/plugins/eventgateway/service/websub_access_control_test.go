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

package service

import (
	"errors"
	"testing"

	"github.com/wso2/api-platform/httpkit/httpclient"
	"github.com/wso2/api-platform/httpkit/netguard"
	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// errReachedRepository is what stubWebSubRepo answers, proving a request got past the
// access-control check to the repository.
var errReachedRepository = errors.New("reached the repository")

type stubWebSubRepo struct {
	repository.WebSubAPIRepository
}

func (stubWebSubRepo) Exists(string, string) (bool, error) { return false, errReachedRepository }

func (stubWebSubRepo) GetByHandle(string, string) (*model.WebSubAPI, error) {
	return nil, errReachedRepository
}

// useClusterDenyRule installs server.network_security.access_control refusing in-cluster
// service names for the duration of the test, and removes it afterwards.
func useClusterDenyRule(t *testing.T) {
	t.Helper()
	policy, err := netguard.GuardConfig{Enabled: true, Mode: "deny", Hostnames: []string{`.*\.svc\.cluster\.local`}}.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	cfg := httpclient.DefaultConfig()
	cfg.DestinationGuard = httpclient.DestinationGuardConfig{Enabled: true, Policy: policy}
	if err := utils.InitUserURLClientFromConfig(cfg); err != nil {
		t.Fatalf("InitUserURLClientFromConfig: %v", err)
	}
	t.Cleanup(func() {
		// Installing a shared client drops the access-control client built beside it.
		if err := utils.InitSharedHTTPClientFromConfig(httpclient.DefaultConfig(), 0); err != nil {
			t.Errorf("reset the shared HTTP client: %v", err)
		}
	})
}

func webSubRequest(upstreamURL string) *api.WebSubAPI {
	id := "netac-websub"
	return &api.WebSubAPI{
		Id: &id, DisplayName: "NETAC WebSub", Version: "v1.0", ProjectId: "default",
		Upstream: api.Upstream{Main: api.UpstreamDefinition{Url: &upstreamURL}},
	}
}

func assertDestinationRefused(t *testing.T, err error, field string) {
	t.Helper()
	if !apperror.DestinationNotAllowed.Is(err) {
		t.Fatalf("err = %v, want DESTINATION_NOT_ALLOWED", err)
	}
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || len(appErr.FieldErrors) != 1 || appErr.FieldErrors[0].Field != field {
		t.Fatalf("err = %v, want one field error naming %s", err, field)
	}
}

// Create and update refuse an upstream access control refuses before the repository is
// read. The positive control is the same request with a permitted upstream, which gets
// past the check to the repository.
func TestWebSubAPIService_ChecksRequestURLs(t *testing.T) {
	useClusterDenyRule(t)
	svc := &WebSubAPIService{repo: stubWebSubRepo{}}

	_, err := svc.Create("org-1", "alice", webSubRequest("http://hub.default.svc.cluster.local/hub"))
	assertDestinationRefused(t, err, "upstream.main.url")
	_, err = svc.Update("org-1", "netac-websub", "alice", webSubRequest("http://hub.default.svc.cluster.local/hub"))
	assertDestinationRefused(t, err, "upstream.main.url")

	if _, err := svc.Create("org-1", "alice", webSubRequest("https://hub.example.com/hub")); !errors.Is(err, errReachedRepository) {
		t.Errorf("create with a permitted upstream: err = %v, want it to reach the repository", err)
	}
	if _, err := svc.Update("org-1", "netac-websub", "alice", webSubRequest("https://hub.example.com/hub")); !errors.Is(err, errReachedRepository) {
		t.Errorf("update with a permitted upstream: err = %v, want it to reach the repository", err)
	}
}
