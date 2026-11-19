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
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

// The block's gateway loads the integration-test policy definitions under
// core/catalog/platformgateway/resources/custom-policies (customPolicies: true) and reports all but
// it-builtin as custom in the manifest it pushes through the cloud console's connect route. These
// steps read that manifest and sync the custom policies into the organisation.
//
// The definitions have fixed names, and a synced policy is an organisation-wide record that
// outlives the gateway, so each scenario first removes any record of its policy an earlier run
// left, and registers the record it syncs for cleanup.
//
// The cloud routes only what the console's gateway Policies tab uses: the manifest, the list, sync
// and delete. A synced policy is never read by version (GET .../versions/{version}), which the
// cloud's gateway answers with its own 404, so its presence and absence are read from the list.

// kindGatewayCustomPolicy is a custom policy synced into the organisation. Its cleanup ID is
// "<uuid>/<version>", the two parts of its route.
var kindGatewayCustomPolicy = cleanup.Kind{Name: "gateway-custom-policy", Order: cleanup.KindPolicy.Order}

const (
	gatewayCustomPoliciesPath = "/gateway-custom-policies"

	// manifestTimeout bounds how long the manifest a gateway pushes on connect takes to be readable.
	manifestTimeout = 2 * time.Minute

	// customPolicyListPage is the page size used to read the organisation's custom policies.
	customPolicyListPage = 100

	// customPolicyRollbackTimeout bounds the delete of a synced policy the step could not register
	// for cleanup.
	customPolicyRollbackTimeout = 30 * time.Second

	// keyGatewayManifest holds the policies of the self-hosted gateway's manifest, as JSON.
	keyGatewayManifest = "cloudSelfHostedGatewayManifest"
	// keyCustomPolicyID and keyCustomPolicyVersion identify the custom policy the scenario synced.
	keyCustomPolicyID      = "cloudCustomPolicyID"
	keyCustomPolicyVersion = "cloudCustomPolicyVersion"
	keyCustomPolicyName    = "cloudCustomPolicyName"
)

// manifestPolicy is one policy of a gateway manifest as the platform reports it.
type manifestPolicy struct {
	Name             string          `json:"name"`
	Version          string          `json:"version"`
	IsCustomPolicy   bool            `json:"isCustomPolicy"`
	PolicyDefinition json.RawMessage `json:"policyDefinition"`
}

// registerCustomPolicySteps binds the steps that read a self-hosted gateway's manifest and sync
// its custom policies.
func (s *Steps) registerCustomPolicySteps(sc stepRegistrar) {
	sc.Step(`^the self-hosted APIP gateway's manifest should eventually list the policy "([^"]*)"$`, s.awaitManifestPolicy)
	sc.Step(`^the self-hosted APIP gateway's manifest should report "([^"]*)" version "([^"]*)" as a (custom|built-in) policy$`, s.assertManifestPolicy)
	sc.Step(`^I remove any custom policy "([^"]*)" an earlier run left in the organisation$`, s.removeLeftoverCustomPolicy)
	sc.Step(`^I sync the custom policy "([^"]*)" version "([^"]*)" from the self-hosted APIP gateway$`, s.syncCustomPolicy)
	sc.Step(`^the synced custom policy should be listed in the organisation$`, s.assertCustomPolicyListed)
	sc.Step(`^the organisation should list the synced custom policy and no custom policy "([^"]*)"$`, s.assertCustomPolicyListedWithout)
	sc.Step(`^I delete the synced custom policy$`, s.deleteCustomPolicy)
	sc.Step(`^the synced custom policy should eventually be absent$`, s.verifyCustomPolicyDeleted)
	sc.Step(`^the organisation should hold no custom policy "([^"]*)"$`, s.assertNoCustomPolicy)
}

func gatewayManifestPath(gatewayID string) string {
	return "/gateways/" + url.PathEscape(gatewayID) + "/manifest"
}

func customPolicyVersionPath(id, version string) string {
	return gatewayCustomPoliciesPath + "/" + url.PathEscape(id) + "/versions/" + url.PathEscape(version)
}

// awaitManifestPolicy waits until the platform holds a manifest for the scenario's self-hosted
// gateway that lists the policy, and keeps the manifest's policies for the assertions after it.
// The gateway pushes its manifest when it connects.
func (s *Steps) awaitManifestPolicy(ctx context.Context, name string) error {
	id, err := requiredValue(ctx, keySelfHostedGatewayID)
	if err != nil {
		return err
	}
	var policies []manifestPolicy
	err = retry.Await(ctx, retry.Options{Timeout: manifestTimeout, Interval: 3 * time.Second},
		func(ctx context.Context) (bool, error) {
			resp, err := s.doAuthenticatedRaw(ctx, http.MethodGet, gatewayManifestPath(id), nil)
			if err != nil {
				return false, retry.Transient(err)
			}
			if resp.StatusCode != http.StatusOK {
				return false, retry.Transient(responseError(resp))
			}
			read, err := manifestPolicies(resp.Body)
			if err != nil {
				return false, retry.Transient(err)
			}
			policies = read
			_, found := findManifestPolicy(read, name, "")
			return found, nil
		}, func(found bool) bool { return found },
		fmt.Sprintf("waiting for self-hosted gateway %q's manifest to list %q", id, name))
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(policies)
	if err != nil {
		return err
	}
	return tcontext.Set(ctx, keyGatewayManifest, string(encoded))
}

// manifestPolicies reads the policies of a manifest response.
func manifestPolicies(body []byte) ([]manifestPolicy, error) {
	var document struct {
		Policies []manifestPolicy `json:"policies"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return nil, fmt.Errorf("decoding the gateway manifest: %w", err)
	}
	return document.Policies, nil
}

// findManifestPolicy finds a policy by name and, when version is set, version.
func findManifestPolicy(policies []manifestPolicy, name, version string) (manifestPolicy, bool) {
	for _, policy := range policies {
		if policy.Name == name && (version == "" || policy.Version == version) {
			return policy, true
		}
	}
	return manifestPolicy{}, false
}

// assertManifestPolicy checks how the manifest reports a policy: a custom policy carries its
// definition, which the organisation syncs; a built-in one does not.
func (s *Steps) assertManifestPolicy(ctx context.Context, name, version, kind string) error {
	encoded, err := requiredValue(ctx, keyGatewayManifest)
	if err != nil {
		return fmt.Errorf("wait for the gateway's manifest first: %w", err)
	}
	var policies []manifestPolicy
	if err := json.Unmarshal([]byte(encoded), &policies); err != nil {
		return err
	}
	return checkManifestPolicy(policies, name, version, kind == "custom")
}

func checkManifestPolicy(policies []manifestPolicy, name, version string, custom bool) error {
	policy, found := findManifestPolicy(policies, name, version)
	if !found {
		return fmt.Errorf("the gateway manifest does not list %s %s", name, version)
	}
	if policy.IsCustomPolicy != custom {
		return fmt.Errorf("the gateway manifest reports %s %s with isCustomPolicy=%t, want %t", name, version, policy.IsCustomPolicy, custom)
	}
	hasDefinition := hasJSONValue(policy.PolicyDefinition)
	if custom && !isJSONObject(policy.PolicyDefinition) {
		return fmt.Errorf("the gateway manifest reports custom policy %s %s without a definition object", name, version)
	}
	if !custom && hasDefinition {
		return fmt.Errorf("the gateway manifest reports built-in policy %s %s with a definition", name, version)
	}
	return nil
}

// hasJSONValue reports whether raw holds a value other than null.
func hasJSONValue(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed != "" && trimmed != "null"
}

// isJSONObject reports whether raw holds a JSON object with at least one field.
func isJSONObject(raw json.RawMessage) bool {
	var object map[string]any
	return json.Unmarshal(raw, &object) == nil && len(object) > 0
}

// listCustomPolicies reads every custom policy the organisation holds.
func (s *Steps) listCustomPolicies(ctx context.Context) ([]map[string]any, error) {
	var all []map[string]any
	for offset := 0; ; offset += customPolicyListPage {
		path := gatewayCustomPoliciesPath + "?limit=" + strconv.Itoa(customPolicyListPage) + "&offset=" + strconv.Itoa(offset)
		resp, err := s.doAuthenticatedRaw(ctx, http.MethodGet, path, nil)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			return nil, responseError(resp)
		}
		items, err := customPolicyPage(resp.Body)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if len(items) < customPolicyListPage {
			return all, nil
		}
	}
}

// customPolicyPage reads one page of the organisation's custom policies. A response without a
// "list" field, or with an entry that is null or has no name, is an error, not an empty page:
// absence is only ever read from a list the platform returned, by name. The platform answers an organisation that holds none with
// "list": null, which is an empty page.
func customPolicyPage(body []byte) ([]map[string]any, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("decoding the organisation's custom policies: %w", err)
	}
	raw, present := envelope["list"]
	if !present {
		return nil, fmt.Errorf("the organisation's custom policies response has no list")
	}
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("decoding the organisation's custom policies: %w", err)
	}
	for i, item := range items {
		if item == nil {
			return nil, fmt.Errorf("the organisation's custom policies list has a null entry at %d", i)
		}
		if name, _ := item["name"].(string); name == "" {
			return nil, fmt.Errorf("the organisation's custom policies list has an entry without a name at %d: %v", i, item)
		}
	}
	return items, nil
}

// customPolicyRecords returns the "<uuid>/<version>" of every record of the named policy.
func customPolicyRecords(items []map[string]any, name string) []string {
	var records []string
	for _, item := range items {
		if itemName, _ := item["name"].(string); itemName != name {
			continue
		}
		id, _ := item["uuid"].(string)
		version, _ := item["version"].(string)
		if id != "" && version != "" {
			records = append(records, id+"/"+version)
		}
	}
	return records
}

// removeLeftoverCustomPolicy deletes every record of the named policy, so the scenario's sync
// starts from an organisation that does not hold it.
func (s *Steps) removeLeftoverCustomPolicy(ctx context.Context, name string) error {
	items, err := s.listCustomPolicies(ctx)
	if err != nil {
		return err
	}
	for _, record := range customPolicyRecords(items, name) {
		if err := s.removeCustomPolicyRecord(ctx, record); err != nil {
			return fmt.Errorf("removing custom policy %s left by an earlier run: %w", name, err)
		}
	}
	return nil
}

// removeCustomPolicyRecord deletes the custom policy "<uuid>/<version>"; one already gone counts
// as deleted.
func (s *Steps) removeCustomPolicyRecord(ctx context.Context, record string) error {
	_, path, settled, err := deleteCustomPolicyRequest(record)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticatedRaw(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return fmt.Errorf("deleting custom policy %s: %w", record, err)
	}
	if !settled(resp) {
		return fmt.Errorf("deleting custom policy %s: %w", record, responseError(resp))
	}
	return nil
}

// syncCustomPolicy asks the organisation to sync a policy from the self-hosted gateway's manifest
// and publishes the response without requiring success, so a scenario can assert a refusal. A
// synced policy is registered for cleanup.
func (s *Steps) syncCustomPolicy(ctx context.Context, name, version string) error {
	gatewayID, err := requiredValue(ctx, keySelfHostedGatewayID)
	if err != nil {
		return err
	}
	query := url.Values{"gatewayId": {gatewayID}, "policyName": {name}, "policyVersion": {version}}
	resp, err := s.doAuthenticated(ctx, http.MethodPost, gatewayCustomPoliciesPath+"/sync?"+query.Encode(), nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	id, err := stringField(resp, "uuid")
	if err != nil {
		return fmt.Errorf("reading the synced custom policy: %w (%s)", err, resp.Describe())
	}
	synced, err := stringField(resp, "version")
	if err != nil {
		return fmt.Errorf("reading the synced custom policy: %w (%s)", err, resp.Describe())
	}
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: kindGatewayCustomPolicy, ID: id + "/" + synced, Actor: resourceActor,
		Description: "custom policy " + name + " " + synced,
	}); err != nil {
		err = fmt.Errorf("registering custom policy %s %s for cleanup: %w", name, synced, err)
		// Nothing else will delete the policy, so the delete must still go out when the
		// scenario's deadline has passed.
		rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), customPolicyRollbackTimeout)
		defer cancel()
		return errors.Join(err, s.removeCustomPolicyRecord(rollbackCtx, id+"/"+synced))
	}
	for key, value := range map[string]string{keyCustomPolicyID: id, keyCustomPolicyVersion: synced, keyCustomPolicyName: name} {
		if err := tcontext.Set(ctx, key, value); err != nil {
			return err
		}
	}
	return nil
}

// syncedCustomPolicy returns the id, version and name of the policy the scenario synced.
func syncedCustomPolicy(ctx context.Context) (id, version, name string, err error) {
	if id, err = requiredValue(ctx, keyCustomPolicyID); err != nil {
		return "", "", "", fmt.Errorf("sync a custom policy first: %w", err)
	}
	if version, err = requiredValue(ctx, keyCustomPolicyVersion); err != nil {
		return "", "", "", err
	}
	if name, err = requiredValue(ctx, keyCustomPolicyName); err != nil {
		return "", "", "", err
	}
	return id, version, name, nil
}

// assertCustomPolicyListed checks the organisation lists the synced policy by its id, name and
// version.
func (s *Steps) assertCustomPolicyListed(ctx context.Context) error {
	id, version, name, err := syncedCustomPolicy(ctx)
	if err != nil {
		return err
	}
	items, err := s.listCustomPolicies(ctx)
	if err != nil {
		return err
	}
	return checkCustomPolicyListed(items, id, version, name)
}

// assertCustomPolicyListedWithout checks that one read of the organisation's list shows the synced
// policy and no record of the named one, so an empty list cannot pass the absence check.
func (s *Steps) assertCustomPolicyListedWithout(ctx context.Context, absent string) error {
	id, version, name, err := syncedCustomPolicy(ctx)
	if err != nil {
		return err
	}
	items, err := s.listCustomPolicies(ctx)
	if err != nil {
		return err
	}
	if err := checkCustomPolicyListed(items, id, version, name); err != nil {
		return err
	}
	if matches := customPoliciesNamed(items, absent); len(matches) > 0 {
		return fmt.Errorf("the organisation holds custom policy %s: %v", absent, matches)
	}
	return nil
}

// checkCustomPolicyListed checks items hold the policy with the uuid, under its name and version.
func checkCustomPolicyListed(items []map[string]any, id, version, name string) error {
	for _, item := range items {
		if itemID, _ := item["uuid"].(string); itemID != id {
			continue
		}
		gotName, _ := item["name"].(string)
		gotVersion, _ := item["version"].(string)
		if gotName != name || gotVersion != version {
			return fmt.Errorf("custom policy %s is listed as %s %s, want %s %s", id, gotName, gotVersion, name, version)
		}
		return nil
	}
	return fmt.Errorf("the organisation does not list custom policy %s (%s %s)", id, name, version)
}

// deleteCustomPolicy deletes the synced policy.
func (s *Steps) deleteCustomPolicy(ctx context.Context) error {
	id, version, _, err := syncedCustomPolicy(ctx)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticated(ctx, http.MethodDelete, customPolicyVersionPath(id, version), nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNoContent {
		return responseError(resp)
	}
	deregister(ctx, kindGatewayCustomPolicy, id+"/"+version)
	return nil
}

// verifyCustomPolicyDeleted waits until the organisation no longer lists the synced policy.
func (s *Steps) verifyCustomPolicyDeleted(ctx context.Context) error {
	id, version, name, err := syncedCustomPolicy(ctx)
	if err != nil {
		return err
	}
	return retry.Await(ctx, retry.Options{Timeout: propagationTimeout(s.topo), Interval: 3 * time.Second},
		func(ctx context.Context) (bool, error) {
			items, err := s.listCustomPolicies(ctx)
			if err != nil {
				return false, retry.Transient(err)
			}
			return listsCustomPolicy(items, id), nil
		}, func(listed bool) bool { return !listed },
		"waiting for custom policy "+name+" "+version+" to be absent")
}

// listsCustomPolicy reports whether items hold the custom policy with the uuid.
func listsCustomPolicy(items []map[string]any, id string) bool {
	for _, item := range items {
		if itemID, _ := item["uuid"].(string); itemID == id {
			return true
		}
	}
	return false
}

// assertNoCustomPolicy checks the organisation holds no record of the named policy.
func (s *Steps) assertNoCustomPolicy(ctx context.Context, name string) error {
	items, err := s.listCustomPolicies(ctx)
	if err != nil {
		return err
	}
	if matches := customPoliciesNamed(items, name); len(matches) > 0 {
		return fmt.Errorf("the organisation holds custom policy %s: %v", name, matches)
	}
	return nil
}

// customPoliciesNamed returns every listed item with the name, whatever else it reports, so an
// item without a uuid or version still counts as present.
func customPoliciesNamed(items []map[string]any, name string) []map[string]any {
	var matches []map[string]any
	for _, item := range items {
		if itemName, _ := item["name"].(string); itemName == name {
			matches = append(matches, item)
		}
	}
	return matches
}

// deleteCustomPolicyRequest is the delete of a synced custom policy identified "<uuid>/<version>".
// A policy already gone counts as deleted.
func deleteCustomPolicyRequest(record string) (string, string, func(*httpx.Response) bool, error) {
	id, version, found := strings.Cut(record, "/")
	if !found || id == "" || version == "" {
		return "", "", nil, fmt.Errorf("custom policy %q is not <uuid>/<version>", record)
	}
	return http.MethodDelete, customPolicyVersionPath(id, version), func(resp *httpx.Response) bool {
		return resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound
	}, nil
}
