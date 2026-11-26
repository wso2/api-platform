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

// Package cachescope decides which objects the operator's informer cache holds.
//
// The operator watches Secrets and ConfigMaps in every watched namespace, because
// HTTPRoute policies, management resources and APIGateways may reference any of them.
// Without a selector the cache also holds two kinds of object the operator never reads,
// and on a cluster with many gateways they dominate its memory:
//
//   - Helm release Secrets. Helm stores every revision of every release, including each
//     gateway the operator installs, in a Secret labeled owner=helm. Each holds the
//     gzipped chart, values and manifest (about 175 KB for a gateway release). The
//     operator reads its releases through Helm's own client, never through this cache.
//   - The gateway chart's rendered ConfigMaps, labeled app.kubernetes.io/name=gateway.
//     The LLM pricing table alone is about 300 KB per gateway. The operator reads only
//     the values ConfigMaps an APIGateway or Gateway references, which the chart does
//     not render.
//
// A Get on a filtered-out object returns NotFound, so a reference to one of these
// objects fails loudly rather than reading stale data.
package cachescope

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
)

const (
	// HelmReleaseOwnerLabel is the label Helm puts on its release storage Secrets.
	HelmReleaseOwnerLabel = "owner"
	// HelmReleaseOwnerValue is the value of HelmReleaseOwnerLabel on those Secrets.
	HelmReleaseOwnerValue = "helm"

	// GatewayChartNameLabel is the label the gateway chart puts on every object it renders.
	GatewayChartNameLabel = "app.kubernetes.io/name"
	// GatewayChartNameValue is the value of GatewayChartNameLabel on those objects.
	GatewayChartNameValue = "gateway"
)

// Options returns the manager's cache options: the watched namespaces, managed fields
// stripped from every cached object, and the exclusions cfg enables.
func Options(defaultNamespaces map[string]cache.Config, cfg config.CacheConfig) (cache.Options, error) {
	opts := cache.Options{
		DefaultNamespaces: defaultNamespaces,
		// Nothing in the operator reads managedFields; they can be as large as the object.
		DefaultTransform: cache.TransformStripManagedFields(),
	}

	byObject := map[client.Object]cache.ByObject{}
	if cfg.ExcludeHelmReleaseSecrets {
		sel, err := notEqualSelector(HelmReleaseOwnerLabel, HelmReleaseOwnerValue)
		if err != nil {
			return cache.Options{}, err
		}
		byObject[&corev1.Secret{}] = cache.ByObject{Label: sel}
	}
	if cfg.ExcludeGatewayChartConfigMaps {
		sel, err := notEqualSelector(GatewayChartNameLabel, GatewayChartNameValue)
		if err != nil {
			return cache.Options{}, err
		}
		byObject[&corev1.ConfigMap{}] = cache.ByObject{Label: sel}
	}
	if len(byObject) > 0 {
		opts.ByObject = byObject
	}
	return opts, nil
}

// notEqualSelector selects objects whose key label is not value, including objects
// that do not carry the label at all.
func notEqualSelector(key, value string) (labels.Selector, error) {
	req, err := labels.NewRequirement(key, selection.NotEquals, []string{value})
	if err != nil {
		return nil, fmt.Errorf("build cache selector %s!=%s: %w", key, value, err)
	}
	return labels.NewSelector().Add(*req), nil
}
