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

package valuespolicy

import (
	"errors"
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func defaultPolicy() Policy {
	return Policy{Enabled: true, AllowedImageRegistries: DefaultAllowedImageRegistries}
}

func TestAllowedValues(t *testing.T) {
	cases := map[string]string{
		"empty": "",
		"labels and replicas": `
commonLabels: {team: a}
gateway:
  controller:
    deployment:
      replicaCount: 2
`,
		"default image registries": `
gateway:
  controller:
    image:
      repository: ghcr.io/wso2/api-platform/gateway-controller
      tag: "1.2.0"
  gatewayRuntime:
    image:
      repository: registry.wso2.com/wso2-api-platform/gateway-runtime
`,
		"tag only override": `
gateway:
  gatewayRuntime:
    image:
      tag: "1.3.0"
`,
		"env, envFrom and volumes without secrets or host paths": `
gateway:
  gatewayRuntime:
    deployment:
      extraEnv:
        - name: A
          value: b
        - name: C
          valueFrom:
            configMapKeyRef: {name: cm, key: k}
        - name: POD
          valueFrom:
            fieldRef: {fieldPath: metadata.name}
      extraEnvFrom:
        - configMapRef: {name: cm}
      extraVolumes:
        - name: scratch
          emptyDir: {}
        - name: cfg
          configMap: {name: cm}
        - name: info
          downwardAPI: {items: []}
        - name: proj
          projected:
            sources:
              - configMap: {name: cm}
              - downwardAPI: {items: []}
      extraVolumeMounts:
        - name: cfg
          mountPath: /etc/extra
          mountPropagation: None
`,
		"hardened security contexts": `
gateway:
  controller:
    deployment:
      podSecurityContext:
        runAsNonRoot: true
        runAsUser: 10001
        fsGroup: 0
        seccompProfile: {type: RuntimeDefault}
      securityContext:
        allowPrivilegeEscalation: false
        privileged: false
        readOnlyRootFilesystem: true
        capabilities:
          drop: [ALL]
          add: [NET_BIND_SERVICE]
`,
		"host settings explicitly off": `
gateway:
  controller:
    deployment:
      hostNetwork: false
      hostPID: false
`,
		"chart secret settings": `
gateway:
  controller:
    controlPlane:
      token: {secretName: cp-token, key: token}
    tls:
      certificateProvider: secret
      secret: {name: gateway-tls}
    encryptionKeys:
      enabled: true
      secretName: enc-keys
`,
	}
	for name, values := range cases {
		t.Run(name, func(t *testing.T) {
			if err := defaultPolicy().Check(values); err != nil {
				t.Fatalf("expected values to be allowed, got %v", err)
			}
		})
	}
}

func TestRefusedValues(t *testing.T) {
	cases := []struct {
		name    string
		values  string
		wantKey string
	}{
		{
			name: "image repository outside allowed registries",
			values: `
gateway:
  controller:
    image:
      repository: docker.io/attacker/gateway-controller
`,
			wantKey: "gateway.controller.image.repository",
		},
		{
			name: "image registry prefix without path boundary",
			values: `
gateway:
  gatewayRuntime:
    image:
      repository: ghcr.io/wso2/api-platform-evil/gateway-runtime
`,
			wantKey: "gateway.gatewayRuntime.image.repository",
		},
		{
			name: "plain image string",
			values: `
sidecar:
  image: quay.io/someone/tool:latest
`,
			wantKey: "sidecar.image",
		},
		{
			name: "extraEnv reads a Secret",
			values: `
gateway:
  gatewayRuntime:
    deployment:
      extraEnv:
        - name: OK
          value: x
        - name: STOLEN
          valueFrom:
            secretKeyRef: {name: other, key: password}
`,
			wantKey: "gateway.gatewayRuntime.deployment.extraEnv[1].valueFrom.secretKeyRef",
		},
		{
			name: "extraEnvFrom reads a Secret",
			values: `
gateway:
  controller:
    deployment:
      extraEnvFrom:
        - secretRef: {name: other}
`,
			wantKey: "gateway.controller.deployment.extraEnvFrom[0].secretRef",
		},
		{
			name: "secret volume",
			values: `
gateway:
  controller:
    deployment:
      extraVolumes:
        - name: s
          secret: {secretName: other}
`,
			wantKey: "gateway.controller.deployment.extraVolumes[0].secret",
		},
		{
			name: "hostPath volume",
			values: `
gateway:
  gatewayRuntime:
    deployment:
      extraVolumes:
        - name: root
          hostPath: {path: /}
`,
			wantKey: "gateway.gatewayRuntime.deployment.extraVolumes[0].hostPath",
		},
		{
			name: "projected secret source",
			values: `
gateway:
  gatewayRuntime:
    deployment:
      extraVolumes:
        - name: p
          projected:
            sources:
              - secret: {name: other}
`,
			wantKey: "gateway.gatewayRuntime.deployment.extraVolumes[0].projected.sources[0].secret",
		},
		{
			name: "bidirectional mount propagation",
			values: `
gateway:
  gatewayRuntime:
    deployment:
      extraVolumeMounts:
        - name: x
          mountPath: /x
          mountPropagation: Bidirectional
`,
			wantKey: "gateway.gatewayRuntime.deployment.extraVolumeMounts[0].mountPropagation",
		},
		{
			name: "privileged container",
			values: `
gateway:
  gatewayRuntime:
    deployment:
      securityContext:
        privileged: true
`,
			wantKey: "gateway.gatewayRuntime.deployment.securityContext.privileged",
		},
		{
			name: "privilege escalation",
			values: `
gateway:
  controller:
    deployment:
      securityContext:
        allowPrivilegeEscalation: true
`,
			wantKey: "gateway.controller.deployment.securityContext.allowPrivilegeEscalation",
		},
		{
			name: "run as root",
			values: `
gateway:
  controller:
    deployment:
      podSecurityContext:
        runAsUser: 0
`,
			wantKey: "gateway.controller.deployment.podSecurityContext.runAsUser",
		},
		{
			name: "runAsNonRoot false",
			values: `
gateway:
  controller:
    deployment:
      podSecurityContext:
        runAsNonRoot: false
`,
			wantKey: "gateway.controller.deployment.podSecurityContext.runAsNonRoot",
		},
		{
			name: "added capability",
			values: `
gateway:
  gatewayRuntime:
    deployment:
      securityContext:
        capabilities:
          add: [SYS_ADMIN]
`,
			wantKey: "gateway.gatewayRuntime.deployment.securityContext.capabilities.add[0]",
		},
		{
			name: "unconfined seccomp",
			values: `
gateway:
  gatewayRuntime:
    deployment:
      podSecurityContext:
        seccompProfile: {type: Unconfined}
`,
			wantKey: "gateway.gatewayRuntime.deployment.podSecurityContext.seccompProfile.type",
		},
		{
			name: "SELinux type",
			values: `
gateway:
  gatewayRuntime:
    deployment:
      securityContext:
        seLinuxOptions: {type: spc_t}
`,
			wantKey: "gateway.gatewayRuntime.deployment.securityContext.seLinuxOptions.type",
		},
		{
			name: "security context erased with null",
			values: `
gateway:
  controller:
    deployment:
      securityContext: null
`,
			wantKey: "gateway.controller.deployment.securityContext",
		},
		{
			name: "hardening field erased with null",
			values: `
gateway:
  controller:
    deployment:
      securityContext:
        runAsNonRoot: null
`,
			wantKey: "gateway.controller.deployment.securityContext.runAsNonRoot",
		},
		{
			name: "hostNetwork",
			values: `
gateway:
  gatewayRuntime:
    deployment:
      hostNetwork: true
`,
			wantKey: "gateway.gatewayRuntime.deployment.hostNetwork",
		},
		{
			name: "quoted boolean",
			values: `
gateway:
  gatewayRuntime:
    deployment:
      securityContext:
        privileged: "yes"
`,
			wantKey: "gateway.gatewayRuntime.deployment.securityContext.privileged",
		},
		{
			name: "map with a non-string key is still walked",
			values: `
gateway:
  gatewayRuntime:
    deployment:
      1: one
      hostIPC: true
`,
			wantKey: "gateway.gatewayRuntime.deployment.hostIPC",
		},
		{
			name: "hostPID",
			values: `
gateway:
  controller:
    deployment:
      hostPID: true
`,
			wantKey: "gateway.controller.deployment.hostPID",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := defaultPolicy().Check(tc.values)
			var violation *ViolationError
			if !errors.As(err, &violation) {
				t.Fatalf("expected a *ViolationError, got %v", err)
			}
			keys := violation.Keys()
			if len(keys) != 1 || keys[0] != tc.wantKey {
				t.Fatalf("expected exactly key %q, got %v", tc.wantKey, keys)
			}
			if !strings.Contains(err.Error(), tc.wantKey) {
				t.Fatalf("error %q does not name key %q", err.Error(), tc.wantKey)
			}
		})
	}
}

func TestMultipleViolationsAreAllReported(t *testing.T) {
	err := defaultPolicy().Check(`
gateway:
  controller:
    image: {repository: evil.example/controller}
    deployment:
      hostNetwork: true
      extraVolumes:
        - name: root
          hostPath: {path: /}
`)
	var violation *ViolationError
	if !errors.As(err, &violation) {
		t.Fatalf("expected a *ViolationError, got %v", err)
	}
	want := []string{
		"gateway.controller.deployment.extraVolumes[0].hostPath",
		"gateway.controller.deployment.hostNetwork",
		"gateway.controller.image.repository",
	}
	if got := violation.Keys(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got keys %v, want %v", got, want)
	}
}

func TestDisabledPolicyAllowsEverything(t *testing.T) {
	p := Policy{Enabled: false}
	if err := p.Check("gateway:\n  controller:\n    deployment:\n      hostPID: true\n"); err != nil {
		t.Fatalf("disabled policy must allow everything, got %v", err)
	}
}

func TestCustomAndEmptyRegistryLists(t *testing.T) {
	values := "gateway:\n  controller:\n    image:\n      repository: mirror.internal:5000/platform/gateway-controller\n"

	custom := Policy{Enabled: true, AllowedImageRegistries: []string{"mirror.internal:5000/platform"}}
	if err := custom.Check(values); err != nil {
		t.Fatalf("mirror registry should be allowed, got %v", err)
	}

	empty := Policy{Enabled: true}
	if err := empty.Check(values); err == nil {
		t.Fatal("an empty registry list must refuse every image override")
	}
}

func TestInvalidYAMLIsNotAViolation(t *testing.T) {
	err := defaultPolicy().Check("gateway: [unterminated")
	if err == nil {
		t.Fatal("expected a parse error")
	}
	var violation *ViolationError
	if errors.As(err, &violation) {
		t.Fatal("a parse error must not be reported as a policy violation")
	}
}

// The gateway chart's own defaults and the operator's sample values must pass,
// or enabling the policy by default would break a stock install.
func TestShippedValuesPass(t *testing.T) {
	for _, path := range []string{
		"../../../helm/gateway-helm-chart/values.yaml",
		"../../config/gateway_values.yaml",
	} {
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Skipf("values file not available: %v", err)
			}
			if err := defaultPolicy().Check(string(data)); err != nil {
				t.Fatalf("shipped values refused: %v", err)
			}
		})
	}
}

// The values embedded in the sample APIGateway ConfigMaps must pass too.
func TestSampleConfigMapValuesPass(t *testing.T) {
	for _, path := range []string{
		"../../config/samples/api_v1_apigateway.yaml",
		"../../config/samples/gateway-custom-config.yaml",
		"../../config/samples/api_v1_apigateway_multiversion.yaml",
	} {
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Skipf("sample not available: %v", err)
			}
			dec := yaml.NewDecoder(strings.NewReader(string(data)))
			found := 0
			for {
				var doc map[string]interface{}
				if err := dec.Decode(&doc); err != nil {
					break
				}
				if doc["kind"] != "ConfigMap" {
					continue
				}
				d, _ := doc["data"].(map[string]interface{})
				values, _ := d["values.yaml"].(string)
				if values == "" {
					continue
				}
				found++
				if err := defaultPolicy().Check(values); err != nil {
					t.Fatalf("sample values refused: %v", err)
				}
			}
			if found == 0 {
				t.Skip("no ConfigMap values in sample")
			}
		})
	}
}
