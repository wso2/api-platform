/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cloudconsole

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"

	corecatalog "github.com/wso2/api-platform/tests/framework/core/catalog/cloudconsole"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
)

// kindPortalRestore hands the settings of the shared managed API Portal back to the values they
// had before a scenario changed them. The portal is the organisation's, so a scenario that updates
// it leaves it as it found it.
var kindPortalRestore = cleanup.Kind{Name: "api-portal-restore", Order: 88}

// portalRestorableFields are the settings an update can change and a restore puts back.
var portalRestorableFields = []string{"name", "description", "loginEnvironment", "selfSignup"}

// portalRestore is the content of a kindPortalRestore record: the portal and the settings it had.
type portalRestore struct {
	Handle string         `json:"handle"`
	Fields map[string]any `json:"fields"`
}

// registerPortalRestore records the portal's current settings for restoring when the scenario
// ends. It does so once per scenario and portal, before the first change.
func (s *Steps) registerPortalRestore(ctx context.Context, handle string) error {
	registry, err := cleanup.Of(ctx)
	if err != nil {
		return err
	}
	for _, pending := range registry.Pending() {
		if pending.Kind != kindPortalRestore {
			continue
		}
		var held portalRestore
		if json.Unmarshal([]byte(pending.ID), &held) == nil && held.Handle == handle {
			return nil
		}
	}
	platform, err := s.organisation(ctx)
	if err != nil {
		return err
	}
	resp, err := platform.get(ctx, portalPath(handle))
	if err != nil {
		return fmt.Errorf("reading managed API Portal %q before changing it: %w", handle, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("reading managed API Portal %q before changing it: %w", handle, responseError(resp))
	}
	record, err := portalRestoreOf(handle, resp.Body)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("recording the settings of managed API Portal %q: %w", handle, err)
	}
	return cleanup.Register(ctx, cleanup.Resource{
		Kind: kindPortalRestore, ID: string(encoded), Actor: resourceActor,
		Description: "settings of managed API Portal " + handle,
	})
}

// portalRestoreOf picks the restorable settings out of a portal's answer. A setting the answer does
// not carry is left out, since there is no value to put back.
func portalRestoreOf(handle string, answer []byte) (portalRestore, error) {
	var portal map[string]any
	if err := json.Unmarshal(answer, &portal); err != nil {
		return portalRestore{}, fmt.Errorf("the answer for managed API Portal %q is not a JSON object: %w", handle, err)
	}
	record := portalRestore{Handle: handle, Fields: map[string]any{}}
	for _, field := range portalRestorableFields {
		if value, present := portal[field]; present && value != nil {
			record.Fields[field] = value
		}
	}
	return record, nil
}

// portalChanges returns the settings of a portal that differ from the ones to restore.
func portalChanges(current map[string]any, want portalRestore) map[string]any {
	changes := map[string]any{}
	for field, value := range want.Fields {
		if !reflect.DeepEqual(current[field], value) {
			changes[field] = value
		}
	}
	return changes
}

// portalRestoreDeleter is the cleanup handler of kindPortalRestore. It puts back every setting that
// still differs, then waits until the portal is active again, because a portal rebuilds after
// some changes and the scenarios after it need it active.
func portalRestoreDeleter(topo *frameworkruntime.Topology, client *httpx.Client) cleanup.Deleter {
	return func(ctx context.Context, resource cleanup.Resource) error {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*portalBuildTimeout)
		defer cancel()
		var record portalRestore
		if err := json.Unmarshal([]byte(resource.ID), &record); err != nil {
			return fmt.Errorf("reading the settings to restore: %w", err)
		}
		base, err := topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
		if err != nil {
			return err
		}
		bearer, err := bearerToken(ctx)
		if err != nil {
			return err
		}
		return restorePortal(ctx, organisationAPI{client: client, base: base, token: bearer}, record)
	}
}

// restorePortal puts the settings of a portal back. A portal that is still rebuilding refuses a
// change until it finishes, so the change is repeated until the platform takes it.
func restorePortal(ctx context.Context, platform organisationAPI, record portalRestore) error {
	if err := awaitPortalActive(ctx, platform, record.Handle); err != nil {
		return fmt.Errorf("waiting for managed API Portal %q before restoring it: %w", record.Handle, err)
	}
	resp, err := platform.get(ctx, portalPath(record.Handle))
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}
	var current map[string]any
	if err := json.Unmarshal(resp.Body, &current); err != nil {
		return fmt.Errorf("reading managed API Portal %q: %w", record.Handle, err)
	}
	changes := portalChanges(current, record)
	if len(changes) == 0 {
		return nil
	}
	body, err := json.Marshal(changes)
	if err != nil {
		return err
	}
	interval := platform.pollInterval
	if interval <= 0 {
		interval = portalPollInterval
	}
	if err := retry.Await(ctx, retry.Options{Timeout: portalBuildTimeout, Interval: interval},
		func(ctx context.Context) (*httpx.Response, error) {
			resp, err := platform.do(ctx, http.MethodPut, portalPath(record.Handle), body)
			if err != nil {
				return nil, retry.Transient(err)
			}
			switch {
			case resp.Succeeded():
				return resp, nil
			case resp.StatusCode == http.StatusConflict || resp.StatusCode == http.StatusTooManyRequests ||
				resp.StatusCode >= http.StatusInternalServerError:
				return nil, retry.Transient(responseError(resp))
			default:
				return nil, responseError(resp)
			}
		},
		func(resp *httpx.Response) bool { return resp != nil },
		"restoring managed API Portal "+record.Handle); err != nil {
		return err
	}
	return awaitPortalActive(ctx, platform, record.Handle)
}

// organisation returns the platform API client of the scenario's organisation.
func (s *Steps) organisation(ctx context.Context) (organisationAPI, error) {
	base, err := s.topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	if err != nil {
		return organisationAPI{}, err
	}
	bearer, err := bearerToken(ctx)
	if err != nil {
		return organisationAPI{}, err
	}
	return organisationAPI{client: s.client, base: base, token: bearer}, nil
}
