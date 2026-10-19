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

package testbench

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/wso2/api-platform/tests/framework/core/components"
)

const (
	// EnvURL points every testbench endpoint at one hosted testbench base URL.
	EnvURL = "TESTBENCH_URL"
	// EnvEndpointURLPrefix, followed by an upper-cased endpoint name with "-" replaced by "_"
	// (TESTBENCH_URL_CONTENT_SAFETY), points one endpoint at its own URL and wins over EnvURL.
	EnvEndpointURLPrefix = "TESTBENCH_URL_"
	// EnvAccessToken is the bearer token sent to a hosted testbench.
	EnvAccessToken = "TESTBENCH_ACCESS_TOKEN"

	componentName = "testbench"
)

// Topology is the part of a booted block topology the resolver reads.
type Topology interface {
	// URL returns the address of a component endpoint reachable from the test process.
	URL(component, endpoint string) (string, error)
	// Component returns a declared component instance.
	Component(name string) (*components.Instance, error)
}

// Configured reports whether a hosted testbench is configured for endpoint through the
// environment. A malformed value is reported as an error rather than ignored.
func Configured(endpoint string) (bool, error) {
	_, ok, err := override(endpoint)
	return ok, err
}

// HostURL returns the testbench endpoint address the test process calls. A configured hosted
// testbench wins; otherwise the endpoint is the mapped address of the testbench container.
func HostURL(topo Topology, endpoint string) (string, error) {
	if hosted, ok, err := override(endpoint); err != nil || ok {
		return hosted, err
	}
	if topo == nil {
		return "", fmt.Errorf("testbench: no topology to resolve endpoint %q", endpoint)
	}
	return topo.URL(componentName, endpoint)
}

// UpstreamURL returns the testbench endpoint address that components under test call, for
// example a gateway upstream. A configured hosted testbench wins; otherwise the endpoint is
// the address the testbench container has on the block network.
func UpstreamURL(topo Topology, endpoint string) (string, error) {
	if hosted, ok, err := override(endpoint); err != nil || ok {
		return hosted, err
	}
	if topo == nil {
		return "", fmt.Errorf("testbench: no topology to resolve endpoint %q", endpoint)
	}
	inst, err := topo.Component(componentName)
	if err != nil {
		return "", err
	}
	return inst.InternalURL(endpoint)
}

// AuthHeaders returns the headers a request to a hosted testbench carries, or nil when no
// access token is configured. The token is read from the environment on every call.
func AuthHeaders() map[string]string {
	token := strings.TrimSpace(os.Getenv(EnvAccessToken))
	if token == "" {
		return nil
	}
	return map[string]string{"Authorization": "Bearer " + token}
}

// override returns the hosted URL configured for endpoint, if any. The configured value is
// never echoed in an error, since a hosted URL may itself be a secret.
func override(endpoint string) (string, bool, error) {
	if suffix := endpointEnvSuffix(endpoint); suffix != "" {
		if name := EnvEndpointURLPrefix + suffix; strings.TrimSpace(os.Getenv(name)) != "" {
			return normalize(name)
		}
	}
	if strings.TrimSpace(os.Getenv(EnvURL)) != "" {
		return normalize(EnvURL)
	}
	return "", false, nil
}

func endpointEnvSuffix(endpoint string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(endpoint), "-", "_"))
}

// normalize validates the value of the named variable and returns it without a trailing slash.
func normalize(name string) (string, bool, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	u, err := url.Parse(raw)
	switch {
	case err != nil:
		return "", false, fmt.Errorf("testbench: %s is not a valid URL", name)
	case u.Scheme != "http" && u.Scheme != "https":
		return "", false, fmt.Errorf("testbench: %s must use http or https", name)
	case u.Host == "":
		return "", false, fmt.Errorf("testbench: %s has no host", name)
	case u.User != nil:
		return "", false, fmt.Errorf("testbench: %s must not carry credentials; use %s", name, EnvAccessToken)
	case u.RawQuery != "" || u.Fragment != "":
		return "", false, fmt.Errorf("testbench: %s must not carry a query or fragment", name)
	}
	return strings.TrimRight(raw, "/"), true, nil
}
