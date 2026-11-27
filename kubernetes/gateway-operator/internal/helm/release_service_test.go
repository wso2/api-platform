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

package helm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"
)

// renderReleaseManifest renders the repository's gateway chart the way `helm template`
// does, so the parser is tested against the real manifest rather than a sample.
func renderReleaseManifest(t *testing.T, releaseName, namespace string, values map[string]interface{}) string {
	t.Helper()
	chrt, err := loader.Load(gatewayHelmChartPath)
	require.NoError(t, err)
	install := action.NewInstall(&action.Configuration{Log: func(string, ...interface{}) {}})
	install.ClientOnly = true
	install.DryRun = true
	install.ReleaseName = releaseName
	install.Namespace = namespace
	// The chart requires kubeVersion >= 1.24; client-only rendering otherwise assumes 1.20.
	install.KubeVersion = &chartutil.KubeVersion{Version: "v1.30.0", Major: "1", Minor: "30"}
	rel, err := install.Run(chrt, values)
	require.NoError(t, err)
	return rel.Manifest
}

func TestFindManifestService_RealChart(t *testing.T) {
	// The controller Deployment refuses to render without encryption keys.
	manifest := renderReleaseManifest(t, "gw-gateway", "infra", map[string]interface{}{
		"gateway": map[string]interface{}{
			"controller": map[string]interface{}{
				"encryptionKeys": map[string]interface{}{"enabled": true, "secretName": "dummy-secret"},
			},
		},
	})
	require.Contains(t, manifest, "kind: Deployment", "the rendered chart should include the Deployments the old parser choked on")

	controller, err := FindManifestService(manifest, "controller")
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(controller.Name, "-controller"), "got %q", controller.Name)
	require.Equal(t, "controller", controller.Selector["app.kubernetes.io/component"])

	runtime, err := FindManifestService(manifest, "gateway-runtime")
	require.NoError(t, err)
	require.NotEqual(t, controller.Name, runtime.Name)
}

func TestDeployedReleaseService(t *testing.T) {
	service := func(name string) string {
		return "apiVersion: v1\nkind: Service\nmetadata:\n  name: " + name +
			"\n  labels:\n    app.kubernetes.io/component: controller\n"
	}
	rel := func(version int, status release.Status, manifest string) *release.Release {
		return &release.Release{
			Name: "gw-gateway", Namespace: "infra", Version: version,
			Info: &release.Info{Status: status}, Manifest: manifest,
		}
	}
	config := func(t *testing.T, rels ...*release.Release) *action.Configuration {
		t.Helper()
		store := storage.Init(driver.NewMemory())
		for _, r := range rels {
			require.NoError(t, store.Create(r))
		}
		return &action.Configuration{Releases: store, Log: func(string, ...interface{}) {}}
	}

	t.Run("a failed upgrade does not change the Service name", func(t *testing.T) {
		cfg := config(t,
			rel(1, release.StatusDeployed, service("gw-old-controller")),
			rel(2, release.StatusFailed, service("gw-new-controller")),
		)
		got, err := deployedReleaseService(cfg, "infra", "gw-gateway", "controller")
		require.NoError(t, err)
		require.Equal(t, "gw-old-controller", got.Name)
	})

	t.Run("no deployed revision names the latest status", func(t *testing.T) {
		cfg := config(t, rel(1, release.StatusFailed, service("gw-new-controller")))
		_, err := deployedReleaseService(cfg, "infra", "gw-gateway", "controller")
		require.ErrorIs(t, err, ErrReleaseNotFound)
		require.ErrorContains(t, err, "latest revision 1 is failed")
	})

	t.Run("no release at all", func(t *testing.T) {
		_, err := deployedReleaseService(config(t), "infra", "gw-gateway", "controller")
		require.ErrorIs(t, err, ErrReleaseNotFound)
	})
}

const releaseManifest = `---
# Source: gateway/templates/gateway/controller/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: gw-gateway-controller
  labels:
    app.kubernetes.io/instance: gw-gateway
    app.kubernetes.io/component: controller
spec:
  type: ClusterIP
  selector:
    app.kubernetes.io/instance: gw-gateway
    app.kubernetes.io/component: controller
  ports:
    - name: rest
      port: 9090
---
# Source: gateway/templates/gateway/runtime/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: gw-gateway-runtime
  labels:
    app.kubernetes.io/component: gateway-runtime
spec:
  selector:
    app.kubernetes.io/component: gateway-runtime
---
# Source: gateway/templates/gateway/controller/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gw-gateway-controller
  labels:
    app.kubernetes.io/component: controller
spec:
  selector:
    matchLabels:
      app.kubernetes.io/instance: gw-gateway
      app.kubernetes.io/component: controller
  template:
    metadata:
      labels:
        app.kubernetes.io/component: controller
    spec:
      containers:
        - name: controller
          ports:
            - containerPort: 9090
---
# Source: gateway/templates/gateway/controller/networkpolicy.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: gw-gateway-controller
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/component: controller
`

func TestFindManifestService(t *testing.T) {
	got, err := FindManifestService(releaseManifest, "controller")
	require.NoError(t, err)
	require.Equal(t, "gw-gateway-controller", got.Name)
	require.Equal(t, map[string]string{
		"app.kubernetes.io/instance":  "gw-gateway",
		"app.kubernetes.io/component": "controller",
	}, got.Selector)

	runtime, err := FindManifestService(releaseManifest, "gateway-runtime")
	require.NoError(t, err)
	require.Equal(t, "gw-gateway-runtime", runtime.Name)
}

func TestFindManifestService_NoMatch(t *testing.T) {
	_, err := FindManifestService(releaseManifest, "policy-engine")
	require.ErrorContains(t, err, "no policy-engine Service")

	_, err = FindManifestService("", "controller")
	require.ErrorContains(t, err, "no controller Service")
}

func TestFindManifestService_MoreThanOne(t *testing.T) {
	twice := releaseManifest + `---
apiVersion: v1
kind: Service
metadata:
  name: second-controller
  labels:
    app.kubernetes.io/component: controller
`
	_, err := FindManifestService(twice, "controller")
	require.ErrorContains(t, err, "2 controller Services")
}
