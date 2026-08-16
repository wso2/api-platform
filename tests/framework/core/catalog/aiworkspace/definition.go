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

package aiworkspace

import (
	"time"

	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/components"
)

// EnvImageAIWorkspace overrides the ai-workspace image, read by the compose file as
// ${AIW_IMAGE}.
const EnvImageAIWorkspace = "AIW_IMAGE"

const svcAIWorkspace = "ai-workspace"

// AIWorkspace is the AI portal: the REAL product, built from source — a React SPA served by
// a Go BFF that owns authentication (basic mode against platform-api's local login) and
// reverse-proxies the platform API. Stateless: in-memory sessions, no runtime.
//
// It is the system under test for the UI suite; a browser drives it over this block's own
// network by alias, which is why the alias is fixed and the domain the BFF advertises is the
// alias form.
func AIWorkspace() *components.Definition {
	return &components.Definition{
		Name:         svcAIWorkspace,
		Alias:        svcAIWorkspace,
		AliasIsFixed: true,

		Compose: &components.ComposeSpec{
			ComposeFile: "tests/framework/core/catalog/aiworkspace/docker-compose.yaml",

			Env:            map[string]string{EnvImageAIWorkspace: shared.Image(EnvImageAIWorkspace, shared.AIWorkspaceImage()).Ref},
			PrimaryService: svcAIWorkspace,
			Services:       []string{svcAIWorkspace},

			// Its OWN serving pair plus the control plane's public certificate — never the
			// control plane's private key. The overlay points ca_file at cp-cert.pem.
			GeneratedFiles: aiWorkspaceCryptoFiles(),

			// The same role->scope vocabulary platform-api enforces, read by the BFF for
			// authorization decisions.
			StagedFiles: map[string]string{
				"role-to-scope-mapping.yaml": "platform-api/resources/role-to-scope-mapping.yaml",
			},
		},

		Endpoints: []components.Endpoint{
			// The BFF serves HTTPS with its generated self-signed pair; probers and the
			// browser both tolerate that (the health prober skips verification, the browser
			// runs with ignoreHTTPSErrors).
			{Name: "https", Port: 9643, Scheme: "https", AwaitListening: true},
		},

		Health: &components.HealthCheck{
			Endpoint: "https", Path: "/healthz", ExpectStatus: 200,
			Timeout: 180 * time.Second, Interval: 2 * time.Second,
		},

		Config: &components.ConfigInjection{
			BaseConfigPath:    "portals/ai-workspace/configs/config.toml",
			SharedOverlayPath: "tests/framework/core/catalog/overlays/ai-workspace-cp-trust.toml",
			ContainerPath:     "/config.toml",
			Format:            components.TOML,
		},

		// Boots AFTER the control plane: the BFF authenticates against it and proxies to it,
		// so starting first fails for a reason unrelated to any test.
		DependsOn: []string{"platform-api"},

		Limits: components.ResourceLimits{CPUs: 1, MemoryMB: 512},
	}
}

// aiWorkspaceCryptoFiles stages the workspace's TLS material: a serving pair of its own and
// the control plane's PUBLIC certificate for upstream trust. Deliberately not the product
// quickstart's single shared pair — handing this container the control plane's private key
// would let a portal bug impersonate the control plane, and nothing here needs it.
func aiWorkspaceCryptoFiles() map[string][]byte {
	serving, err := shared.SelfSignedCert(svcAIWorkspace, []string{svcAIWorkspace, "localhost"})
	if err != nil {
		panic(err) // crypto/rand failure while assembling configuration; not recoverable
	}
	cp := shared.ControlPlaneCrypto()
	return map[string][]byte{
		"tls/cert.pem":    serving.CertPEM,
		"tls/key.pem":     serving.PrivateKeyPEM,
		"tls/cp-cert.pem": cp["certs/cert.pem"],
	}
}
