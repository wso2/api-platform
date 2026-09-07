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

// EnvImageAIWorkspace names the environment variable used to override the AI Workspace image.
const EnvImageAIWorkspace = "AIW_IMAGE"

const svcAIWorkspace = "ai-workspace"

// AIWorkspace returns the AI Workspace component definition.
func AIWorkspace() *components.Definition {
	env := map[string]string{EnvImageAIWorkspace: shared.Image(EnvImageAIWorkspace, shared.AIWorkspaceImage()).Ref}
	for key, value := range runtimeCoverageEnvironment() {
		env[key] = value
	}
	return &components.Definition{
		Name:         svcAIWorkspace,
		Alias:        svcAIWorkspace,
		AliasIsFixed: true,

		Compose: &components.ComposeSpec{
			ComposeFile: "tests/framework/core/catalog/aiworkspace/docker-compose.yaml",

			Env:            env,
			PrimaryService: svcAIWorkspace,
			Services:       []string{svcAIWorkspace},
			CoverageServices: []components.CoverageService{{
				Name: svcAIWorkspace, Types: []string{"go"},
			}},

			GeneratedFiles: aiWorkspaceCryptoFiles(),

			StagedFiles: map[string]string{
				"role-to-scope-mapping.yaml": "platform-api/resources/role-to-scope-mapping.yaml",
			},
		},

		Endpoints: []components.Endpoint{
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

		DependsOn: []string{"platform-api"},

		Limits: components.ResourceLimits{CPUs: 1, MemoryMB: 512},
	}
}

func runtimeCoverageEnvironment() map[string]string {
	if !shared.CoverageMode() {
		return nil
	}
	spec, _ := BuildSpec("")
	env := make(map[string]string, len(spec.Coverage.Environment))
	for key, value := range spec.Coverage.Environment {
		env[key] = value
	}
	return env
}

// aiWorkspaceCryptoFiles returns the workspace serving certificate and control-plane CA files.
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
