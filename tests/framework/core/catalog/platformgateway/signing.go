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

package platformgateway

import (
	_ "embed"
	"fmt"
	"maps"
	"slices"

	"github.com/wso2/api-platform/tests/framework/core/components"
)

const (
	signingKeysComposeFile = "tests/framework/core/catalog/platformgateway/docker-compose.signing-keys.yaml"

	// envSigningKeyActive and envSigningKeyPrevious carry the PEM keys into the Compose project
	// environment, from which the override's configs copy them into the runtime. They are
	// project variables only: a multi-line value cannot be written to the services' env file,
	// and the keys stay out of every container's environment.
	envSigningKeyActive   = "GATEWAY_SIGNING_KEY_ACTIVE"
	envSigningKeyPrevious = "GATEWAY_SIGNING_KEY_PREVIOUS"
)

// The gateway signing keys are fixed integration-test keys, so a scenario can assert the
// published key IDs. They sign nothing outside a test gateway.
var (
	//go:embed signing-keys/active.pem
	signingKeyActivePEM string
	//go:embed signing-keys/previous.pem
	signingKeyPreviousPEM string
)

// withSigningKeys returns a copy of def whose runtime holds the gateway signing keys at
// /secrets/gateway-runtime/signing. The policy engine rejects a key file that is group-writable
// or readable by others, so the keys are copied in as Compose configs owned by the runtime user
// with mode 0400 rather than bind-mounted.
func withSigningKeys(def *components.Definition) (*components.Definition, error) {
	if def == nil || def.Compose == nil {
		return nil, fmt.Errorf("the gateway signing keys need a compose-backed definition")
	}
	if slices.Contains(def.Compose.ComposeOverrideFiles, signingKeysComposeFile) {
		return nil, fmt.Errorf("%s already holds the gateway signing keys", def)
	}
	out := *def

	compose := *def.Compose
	compose.ComposeOverrideFiles = append(append([]string(nil), def.Compose.ComposeOverrideFiles...), signingKeysComposeFile)
	compose.ProjectEnv = maps.Clone(def.Compose.ProjectEnv)
	if compose.ProjectEnv == nil {
		compose.ProjectEnv = map[string]string{}
	}
	compose.ProjectEnv[envSigningKeyActive] = signingKeyActivePEM
	compose.ProjectEnv[envSigningKeyPrevious] = signingKeyPreviousPEM
	out.Compose = &compose

	return &out, nil
}
