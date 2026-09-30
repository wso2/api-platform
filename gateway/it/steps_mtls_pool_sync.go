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
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// clientAuthorityPoolTimeout and clientAuthorityPoolPollInterval bound
// waitForClientAuthorityPool.
const (
	clientAuthorityPoolTimeout      = 10 * time.Second
	clientAuthorityPoolPollInterval = 200 * time.Millisecond
)

// clientAuthorityLazyResourceType is the lazy resource type the controller
// publishes each usage: downstream row under.
const clientAuthorityLazyResourceType = "ClientCertificateAuthority"

// downstreamClientCASecret is the SDS secret the HTTPS listener validates
// client certificates against.
const downstreamClientCASecret = "downstream_client_ca"

// clientAuthority is one pool entry as one component sees it.
type clientAuthority struct {
	role  string
	count int
	// thumbprints is known only from the policy engine, which holds the PEMs.
	thumbprints []string
}

// waitForClientAuthorityPool polls until the policy engine's published pool
// and Envoy's downstream client-CA secret both match the controller's usage:
// downstream rows, or fails after clientAuthorityPoolTimeout.
func waitForClientAuthorityPool(state *TestState) error {
	deadline := time.Now().Add(clientAuthorityPoolTimeout)
	for {
		mismatch, err := clientAuthorityPoolMismatch(state)
		if err == nil && mismatch == "" {
			state.DeleteContextValue(clientAuthorityPoolChangedContextKey)
			return nil
		}
		if time.Now().After(deadline) {
			if err != nil {
				return fmt.Errorf("the gateway did not apply the client authority pool within %s: %w", clientAuthorityPoolTimeout, err)
			}
			return fmt.Errorf("the gateway did not apply the client authority pool within %s: %s", clientAuthorityPoolTimeout, mismatch)
		}
		time.Sleep(clientAuthorityPoolPollInterval)
	}
}

// clientAuthorityPoolMismatch describes the first difference between the
// controller's pool and what the policy engine and Envoy hold, or returns ""
// when they agree. Envoy is compared only while an HTTPS listener names the
// downstream client-CA secret, since only then does Envoy use it.
func clientAuthorityPoolMismatch(state *TestState) (string, error) {
	want, err := controllerClientAuthorities(state)
	if err != nil {
		return "", err
	}
	published, err := policyEngineClientAuthorities(state)
	if err != nil {
		return "", err
	}
	for name, w := range want {
		p, ok := published[name]
		if !ok {
			return fmt.Sprintf("the policy engine does not hold %q", name), nil
		}
		if p.role != w.role || p.count != w.count {
			return fmt.Sprintf("the policy engine holds %q with role %q and %d certificates, the controller has role %q and %d",
				name, p.role, p.count, w.role, w.count), nil
		}
	}
	for name := range published {
		if _, ok := want[name]; !ok {
			return fmt.Sprintf("the policy engine still holds %q", name), nil
		}
	}

	referenced, warming, err := envoyListenerNamesClientCASecret(state)
	if err != nil {
		return "", err
	}
	if len(warming) > 0 {
		return "an Envoy listener is still warming", nil
	}
	if !referenced {
		return "", nil
	}
	served, present, err := envoyClientCAThumbprints(state)
	if err != nil {
		return "", err
	}
	if !present {
		return fmt.Sprintf("an Envoy listener names %s but Envoy holds no such active secret", downstreamClientCASecret), nil
	}
	expected := map[string]bool{}
	for _, p := range published {
		for _, t := range p.thumbprints {
			expected[t] = true
		}
	}
	if !sameStringSet(expected, served) {
		return fmt.Sprintf("Envoy's %s holds %v, the pool holds %v", downstreamClientCASecret, sortedThumbprints(served), sortedThumbprints(expected)), nil
	}
	return "", nil
}

// controllerClientAuthorities lists the controller's usage: downstream rows as
// admin, keyed by name.
func controllerClientAuthorities(state *TestState) (map[string]clientAuthority, error) {
	var listing struct {
		Certificates []struct {
			Name  string `json:"name"`
			Role  string `json:"role"`
			Count int    `json:"count"`
		} `json:"certificates"`
	}
	if err := getJSONAsAdmin(state, state.Config.GatewayControllerURL+"/certificates?usage=downstream", &listing); err != nil {
		return nil, err
	}
	out := make(map[string]clientAuthority, len(listing.Certificates))
	for _, c := range listing.Certificates {
		out[c.Name] = clientAuthority{role: c.Role, count: c.Count}
	}
	return out, nil
}

// policyEngineClientAuthorities reads the client authorities the policy
// engine holds from its config dump, keyed by name.
func policyEngineClientAuthorities(state *TestState) (map[string]clientAuthority, error) {
	var dump struct {
		LazyResources struct {
			ResourcesByType map[string][]struct {
				ID       string `json:"id"`
				Resource struct {
					Certificates []string `json:"certificates"`
					Role         string   `json:"role"`
				} `json:"resource"`
			} `json:"resources_by_type"`
		} `json:"lazy_resources"`
	}
	if err := getJSON(state, state.Config.PolicyEngineURL+"/config_dump", &dump); err != nil {
		return nil, err
	}
	resources := dump.LazyResources.ResourcesByType[clientAuthorityLazyResourceType]
	out := make(map[string]clientAuthority, len(resources))
	for _, r := range resources {
		thumbprints, err := pemThumbprints(strings.Join(r.Resource.Certificates, "\n"))
		if err != nil {
			return nil, fmt.Errorf("policy engine client authority %q: %w", r.ID, err)
		}
		out[r.ID] = clientAuthority{role: r.Resource.Role, count: len(r.Resource.Certificates), thumbprints: thumbprints}
	}
	return out, nil
}

// envoyListenerNamesClientCASecret reports whether any active Envoy listener
// names the downstream client-CA secret, and the names of the listeners that
// are still warming.
func envoyListenerNamesClientCASecret(state *TestState) (referenced bool, warming []string, err error) {
	var dump struct {
		Configs []struct {
			Name         string          `json:"name"`
			ActiveState  json.RawMessage `json:"active_state"`
			WarmingState json.RawMessage `json:"warming_state"`
		} `json:"configs"`
	}
	if err := getJSON(state, envoyAdminURL()+"/config_dump?resource=dynamic_listeners", &dump); err != nil {
		return false, nil, err
	}
	for _, l := range dump.Configs {
		if len(l.WarmingState) > 0 && string(l.WarmingState) != "null" {
			warming = append(warming, l.Name)
		}
		if strings.Contains(string(l.ActiveState), `"`+downstreamClientCASecret+`"`) {
			referenced = true
		}
	}
	return referenced, warming, nil
}

// waitForEnvoyListenersActive waits until no Envoy listener is warming. While
// a replacement listener warms, Envoy serves new connections on the previous
// instance, so a request sent then meets the configuration being replaced.
func waitForEnvoyListenersActive(state *TestState) error {
	const interval = 100 * time.Millisecond
	timeout := envoySettleTimeout
	deadline := time.Now().Add(timeout)
	for {
		_, warming, err := envoyListenerNamesClientCASecret(state)
		if err == nil && len(warming) == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			if err != nil {
				return fmt.Errorf("reading Envoy's listeners: %w", err)
			}
			return fmt.Errorf("Envoy listeners still warming after %s: %s", timeout, strings.Join(warming, ", "))
		}
		time.Sleep(interval)
	}
}

// listenerMoveGrace bounds how long waitForEnvoySettled waits for Envoy's
// listener version to move past a pre-mutation baseline. A mutation that
// leaves the HTTPS listener unchanged, such as a second mtls-auth API, never
// moves it.
const listenerMoveGrace = 2 * time.Second

// envoySettleTimeout bounds each of waitForEnvoySettled's waits on warming
// resources.
const envoySettleTimeout = 10 * time.Second

// envoyListenersVersion returns the version_info of Envoy's listeners dump,
// the version of the last listener update Envoy accepted.
func envoyListenersVersion(state *TestState) (string, error) {
	var dump struct {
		Configs []struct {
			Type        string `json:"@type"`
			VersionInfo string `json:"version_info"`
		} `json:"configs"`
	}
	if err := getJSON(state, envoyAdminURL()+"/config_dump?mask=version_info", &dump); err != nil {
		return "", err
	}
	for _, c := range dump.Configs {
		if strings.HasSuffix(c.Type, ".ListenersConfigDump") {
			return c.VersionInfo, nil
		}
	}
	return "", fmt.Errorf("Envoy's config dump has no listeners dump")
}

// waitForEnvoySettled waits for Envoy to apply the scenario's latest changes.
// After a mutation that can change the HTTPS listener it first waits, within
// listenerMoveGrace, for the listener version to move past the pre-mutation
// baseline. It then waits until no listener and no cluster is warming: a new
// connection is served by the previous listener until its replacement is
// active, and a warming cluster answers 503.
func waitForEnvoySettled(state *TestState) error {
	if raw, ok := state.GetContextValue(listenerBaselineContextKey); ok {
		baseline, _ := raw.(string)
		deadline := time.Now().Add(listenerMoveGrace)
		for time.Now().Before(deadline) {
			if version, err := envoyListenersVersion(state); err == nil && version != baseline {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		state.DeleteContextValue(listenerBaselineContextKey)
	}
	if err := waitForEnvoyListenersActive(state); err != nil {
		return err
	}
	return waitForEnvoyClustersActive(state)
}

// waitForEnvoyClustersActive waits until Envoy lists no warming cluster.
func waitForEnvoyClustersActive(state *TestState) error {
	deadline := time.Now().Add(envoySettleTimeout)
	for {
		var dump struct {
			Configs []struct {
				Cluster struct {
					Name string `json:"name"`
				} `json:"cluster"`
			} `json:"configs"`
		}
		err := getJSON(state, envoyAdminURL()+"/config_dump?resource=dynamic_warming_clusters", &dump)
		if err == nil && len(dump.Configs) == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			if err != nil {
				return fmt.Errorf("reading Envoy's warming clusters: %w", err)
			}
			names := make([]string, 0, len(dump.Configs))
			for _, c := range dump.Configs {
				names = append(names, c.Cluster.Name)
			}
			return fmt.Errorf("Envoy clusters still warming after %s: %s", envoySettleTimeout, strings.Join(names, ", "))
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// envoyClientCAThumbprints returns the thumbprints of the certificates in
// Envoy's active downstream client-CA secret, and whether it holds one.
func envoyClientCAThumbprints(state *TestState) (map[string]bool, bool, error) {
	var dump struct {
		Configs []struct {
			Name   string `json:"name"`
			Secret struct {
				ValidationContext struct {
					TrustedCA struct {
						InlineBytes string `json:"inline_bytes"`
					} `json:"trusted_ca"`
				} `json:"validation_context"`
			} `json:"secret"`
		} `json:"configs"`
	}
	if err := getJSON(state, envoyAdminURL()+"/config_dump?resource=dynamic_active_secrets", &dump); err != nil {
		return nil, false, err
	}
	for _, c := range dump.Configs {
		if c.Name != downstreamClientCASecret {
			continue
		}
		bundle, err := base64.StdEncoding.DecodeString(c.Secret.ValidationContext.TrustedCA.InlineBytes)
		if err != nil {
			return nil, false, fmt.Errorf("Envoy's %s trusted_ca is not base64: %w", downstreamClientCASecret, err)
		}
		thumbprints, err := pemThumbprints(string(bundle))
		if err != nil {
			return nil, false, fmt.Errorf("Envoy's %s: %w", downstreamClientCASecret, err)
		}
		set := make(map[string]bool, len(thumbprints))
		for _, t := range thumbprints {
			set[t] = true
		}
		return set, true, nil
	}
	return nil, false, nil
}

// pemThumbprints returns the lowercase-hex SHA-256 thumbprint of each
// CERTIFICATE block in data, in order.
func pemThumbprints(data string) ([]string, error) {
	var out []string
	rest := []byte(data)
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return out, nil
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		sum := sha256.Sum256(block.Bytes)
		out = append(out, hex.EncodeToString(sum[:]))
	}
}

func envoyAdminURL() string {
	return fmt.Sprintf("http://localhost:%s", EnvoyAdminPort)
}

// getJSONAsAdmin GETs url with the admin user's basic credentials and
// decodes the JSON body into out.
func getJSONAsAdmin(state *TestState, url string, out any) error {
	admin, ok := state.Config.Users["admin"]
	if !ok {
		return fmt.Errorf("no admin user configured")
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(admin.Username, admin.Password)
	return doJSON(state.HTTPClient, req, out)
}

// getJSON GETs url and decodes the JSON body into out.
func getJSON(state *TestState, url string, out any) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	return doJSON(state.HTTPClient, req, out)
}

func doJSON(client *http.Client, req *http.Request, out any) error {
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s returned status %d", req.URL, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("GET %s returned an unreadable body: %w", req.URL, err)
	}
	return nil
}

func sameStringSet(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func sortedThumbprints(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
