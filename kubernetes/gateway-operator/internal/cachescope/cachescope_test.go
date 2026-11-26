/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cachescope

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
)

// selectorFor returns the ByObject label selector for obj's type, or nil.
func selectorFor(t *testing.T, opts cache.Options, obj client.Object) labels.Selector {
	t.Helper()
	for o, by := range opts.ByObject {
		if reflect.TypeOf(o) == reflect.TypeOf(obj) {
			return by.Label
		}
	}
	return nil
}

func TestOptionsExcludesHelmReleaseSecrets(t *testing.T) {
	opts, err := Options(nil, config.CacheConfig{ExcludeHelmReleaseSecrets: true})
	if err != nil {
		t.Fatalf("Options: %v", err)
	}
	sel := selectorFor(t, opts, &corev1.Secret{})
	if sel == nil {
		t.Fatal("no selector for Secret")
	}

	cases := []struct {
		name   string
		labels map[string]string
		cached bool
	}{
		// Labels Helm puts on a gateway release revision the operator installed.
		{"helm release", map[string]string{"owner": "helm", "name": "gw-34e37ec4", "status": "deployed", "version": "1"}, false},
		{"superseded helm release", map[string]string{"owner": "helm", "name": "gw-34e37ec4", "status": "superseded", "version": "2"}, false},
		// Positive controls: Secrets a resource may reference stay cached.
		{"unlabeled user Secret", nil, true},
		{"chart-rendered Secret", map[string]string{"app.kubernetes.io/name": "gateway", "app.kubernetes.io/managed-by": "Helm"}, true},
		{"owner label with another value", map[string]string{"owner": "team-a"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sel.Matches(labels.Set(tc.labels)); got != tc.cached {
				t.Fatalf("cached = %v, want %v (selector %q)", got, tc.cached, sel)
			}
		})
	}

	if selectorFor(t, opts, &corev1.ConfigMap{}) != nil {
		t.Fatal("ConfigMap selector set although its exclusion is off")
	}
}

func TestOptionsExcludesGatewayChartConfigMaps(t *testing.T) {
	opts, err := Options(nil, config.CacheConfig{ExcludeGatewayChartConfigMaps: true})
	if err != nil {
		t.Fatalf("Options: %v", err)
	}
	sel := selectorFor(t, opts, &corev1.ConfigMap{})
	if sel == nil {
		t.Fatal("no selector for ConfigMap")
	}

	cases := []struct {
		name   string
		labels map[string]string
		cached bool
	}{
		// Labels the gateway chart puts on gw-*-gateway-llm-pricing and gw-*-gateway-config.
		{"gateway chart ConfigMap", map[string]string{
			"app.kubernetes.io/instance":   "gw-db18c288",
			"app.kubernetes.io/managed-by": "Helm",
			"app.kubernetes.io/name":       "gateway",
			"app.kubernetes.io/version":    "1.2.0",
			"helm.sh/chart":                "gateway-1.2.5",
		}, false},
		// Positive controls: values ConfigMaps an APIGateway references stay cached.
		{"cloud values ConfigMap", map[string]string{
			"openchoreo.dev/managed-by":                   "renderedrelease-controller",
			"openchoreo.dev/rendered-release-resource-id": "gateway-values",
		}, true},
		{"unlabeled values ConfigMap", nil, true},
		{"operator chart ConfigMap", map[string]string{"app.kubernetes.io/name": "gateway-operator"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sel.Matches(labels.Set(tc.labels)); got != tc.cached {
				t.Fatalf("cached = %v, want %v (selector %q)", got, tc.cached, sel)
			}
		})
	}

	if selectorFor(t, opts, &corev1.Secret{}) != nil {
		t.Fatal("Secret selector set although its exclusion is off")
	}
}

func TestOptionsWithExclusionsOff(t *testing.T) {
	ns := map[string]cache.Config{"team-a": {}}
	opts, err := Options(ns, config.CacheConfig{})
	if err != nil {
		t.Fatalf("Options: %v", err)
	}
	if opts.ByObject != nil {
		t.Fatalf("ByObject = %v, want nil", opts.ByObject)
	}
	if _, ok := opts.DefaultNamespaces["team-a"]; !ok || len(opts.DefaultNamespaces) != 1 {
		t.Fatalf("DefaultNamespaces = %v, want only team-a", opts.DefaultNamespaces)
	}
}

func TestOptionsStripsManagedFields(t *testing.T) {
	opts, err := Options(nil, config.CacheConfig{})
	if err != nil {
		t.Fatalf("Options: %v", err)
	}
	if opts.DefaultTransform == nil {
		t.Fatal("DefaultTransform not set")
	}
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name:          "values",
		ManagedFields: []metav1.ManagedFieldsEntry{{Manager: "kubectl"}},
	}}
	out, err := opts.DefaultTransform(cm)
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	got := out.(*corev1.ConfigMap)
	if got.ManagedFields != nil {
		t.Fatalf("ManagedFields = %v, want nil", got.ManagedFields)
	}
	if got.Name != "values" {
		t.Fatalf("Name = %q, want values", got.Name)
	}
}
