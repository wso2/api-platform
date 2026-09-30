/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package it

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"time"
)

// cleanupReferenceTimeout and cleanupReferencePollInterval bound each of
// cleanupTrackedCertificates' waits for references to clear.
// certificateDeleteTimeout bounds the retries of a delete refused as
// referenced (409).
const (
	cleanupReferenceTimeout      = 5 * time.Second
	cleanupReferencePollInterval = 25 * time.Millisecond
	certificateDeleteTimeout     = 10 * time.Second
)

// waitForDeletedConfigsToLeaveController polls the controller's config dump
// until none of the named configurations is still deployed there, or
// cleanupReferenceTimeout passes. The controller checks certificate
// references against that store, which converges asynchronously after a
// delete, so a certificate deleted sooner can still be refused as referenced.
func (m *mtlsSteps) waitForDeletedConfigsToLeaveController(names []string) {
	if len(names) == 0 {
		return
	}
	deleted := make(map[string]bool, len(names))
	for _, name := range names {
		deleted[name] = true
	}
	deadline := time.Now().Add(cleanupReferenceTimeout)
	for {
		var dump struct {
			Apis []struct {
				Configuration struct {
					Metadata struct {
						Name string `json:"name"`
					} `json:"metadata"`
				} `json:"configuration"`
				Metadata struct {
					Status string `json:"status"`
				} `json:"metadata"`
			} `json:"apis"`
		}
		err := getJSONAsAdmin(m.state, m.state.Config.GatewayControllerAdminURL+"/config_dump", &dump)
		if err == nil {
			stillDeployed := false
			for _, item := range dump.Apis {
				if deleted[item.Configuration.Metadata.Name] && item.Metadata.Status != "undeployed" {
					stillDeployed = true
					break
				}
			}
			if !stillDeployed {
				return
			}
		}
		if time.Now().After(deadline) {
			return
		}
		time.Sleep(cleanupReferencePollInterval)
	}
}

// deletedRouteTimeout bounds, per configuration, the wait for a deleted
// configuration's route to leave the gateway's plain HTTP listener.
const deletedRouteTimeout = 10 * time.Second

// waitForDeployedRoutesRemoved waits, up to deletedRouteTimeout each, until
// the router answers the route of every API and Agent this scenario deployed
// with 404, which it does once the gateway has applied the delete.
func waitForDeployedRoutesRemoved(state *TestState) error {
	routes := deployedRoutes(state)
	names := make([]string, 0, len(routes))
	for name := range routes {
		names = append(names, name)
	}
	sort.Strings(names)
	client := &http.Client{Timeout: 2 * time.Second}
	for _, name := range names {
		r := routes[name]
		url := state.Config.RouterURL + r.path
		deadline := time.Now().Add(deletedRouteTimeout)
		lastStatus := 0
		for {
			req, err := http.NewRequest(r.method, url, nil)
			if err != nil {
				return fmt.Errorf("probing the route of deleted configuration %q: %w", name, err)
			}
			resp, err := client.Do(req)
			if err == nil {
				lastStatus = resp.StatusCode
				resp.Body.Close()
				if lastStatus == http.StatusNotFound {
					break
				}
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("deleted configuration %q: %s %s still answered %d after %s, expected 404", name, r.method, url, lastStatus, deletedRouteTimeout)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	return nil
}

// scenarioConfigNames returns the API and Agent names this scenario deployed.
func scenarioConfigNames(state *TestState) []string {
	var names []string
	for _, key := range []string{deployedAPINamesContextKey, deployedAgentNamesContextKey} {
		if raw, ok := state.GetContextValue(key); ok {
			if recorded, ok := raw.([]string); ok {
				names = append(names, recorded...)
			}
		}
	}
	return names
}

// cleanupTrackedCertificates deletes, as admin, every gateway identity and
// certificate this scenario attempted to upload. It first waits, within
// cleanupReferenceTimeout, for the listing to show none of them referenced
// by an API. A delete the controller still refuses as referenced, which a
// reference the listing does not count can cause, is retried until
// certificateDeleteTimeout, first after the scenario's configurations have
// left the controller's store; a certificate still refused then is logged
// as leaked. Names that were never stored are skipped, and the previous auth
// header is not restored.
func (m *mtlsSteps) cleanupTrackedCertificates() {
	if len(m.uploadedIdentityNames) == 0 && len(m.uploadedNames) == 0 {
		return
	}

	admin, ok := m.state.Config.Users["admin"]
	if !ok {
		return
	}
	creds := base64.StdEncoding.EncodeToString([]byte(admin.Username + ":" + admin.Password))
	m.httpSteps.SetHeader("Authorization", "Basic "+creds)

	tracked := make(map[string]bool, len(m.uploadedIdentityNames)+len(m.uploadedNames))
	for _, name := range append(append([]string{}, m.uploadedIdentityNames...), m.uploadedNames...) {
		tracked[name] = true
	}

	var idByName map[string]string
	deadline := time.Now().Add(cleanupReferenceTimeout)
	for {
		if err := m.httpSteps.SendGETToService("gateway-controller", "/certificates"); err != nil {
			return
		}
		var parsed struct {
			Certificates []map[string]any `json:"certificates"`
		}
		if err := json.Unmarshal(m.httpSteps.LastBody(), &parsed); err != nil {
			return
		}

		idByName = make(map[string]string, len(parsed.Certificates))
		referenced := false
		for _, item := range parsed.Certificates {
			name := fmt.Sprint(item["name"])
			idByName[name] = fmt.Sprint(item["id"])
			if refs, ok := item["referencedByApis"].(float64); ok && refs > 0 && tracked[name] {
				referenced = true
			}
		}
		if !referenced || time.Now().After(deadline) {
			break
		}
		time.Sleep(cleanupReferencePollInterval)
	}

	storeConverged := false
	deleteTracked := func(names []string) {
		for _, name := range names {
			id, ok := idByName[name]
			if !ok {
				continue
			}
			deadline := time.Now().Add(certificateDeleteTimeout)
			for {
				_ = m.httpSteps.SendDELETEToService("gateway-controller", "/certificates/"+id)
				resp := m.httpSteps.LastResponse()
				if resp == nil || resp.StatusCode != http.StatusConflict {
					break
				}
				if time.Now().After(deadline) {
					log.Printf("ERROR: cleanup could not delete certificate %q (id %s): still refused as referenced after %s", name, id, certificateDeleteTimeout)
					break
				}
				if !storeConverged {
					m.waitForDeletedConfigsToLeaveController(scenarioConfigNames(m.state))
					storeConverged = true
				} else {
					time.Sleep(cleanupReferencePollInterval * 4)
				}
			}
		}
	}
	deleteTracked(m.uploadedIdentityNames)
	deleteTracked(m.uploadedNames)
}
