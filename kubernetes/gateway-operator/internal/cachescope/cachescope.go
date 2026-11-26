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
// When the Gateway API controllers are off, the operator reads Services and Deployments
// only to find a gateway release's controller and gateway-runtime workloads, so those
// caches hold only objects labeled with one of those two components rather than every
// Service and Deployment in the cluster. HTTPRoute backends can be any Service, so this
// narrowing applies only when the Gateway API controllers are off.
//
// A Get on a filtered-out object returns NotFound, so a reference to one of these
// objects fails loudly rather than reading stale data.
package cachescope

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
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

	// GatewayChartComponentLabel is the label the gateway chart puts on its Services and
	// Deployments to say which workload they belong to. Unlike app.kubernetes.io/name it
	// does not follow the chart's nameOverride, and it is the label the operator's own
	// Service lookups match on.
	GatewayChartComponentLabel = "app.kubernetes.io/component"
)

// GatewayChartComponents are the GatewayChartComponentLabel values of the gateway
// chart's Services and Deployments.
var GatewayChartComponents = []string{"controller", "gateway-runtime"}

// Options returns the manager's cache options: the watched namespaces, managed fields
// stripped from every cached object, the exclusions cfg.Cache enables, and, when the
// Gateway API controllers are off, Services and Deployments narrowed to the gateway
// chart's workloads.
func Options(defaultNamespaces map[string]cache.Config, cfg *config.OperatorConfig) (cache.Options, error) {
	opts := cache.Options{
		DefaultNamespaces: defaultNamespaces,
		// Nothing in the operator reads managedFields; they can be as large as the object.
		DefaultTransform: cache.TransformStripManagedFields(),
	}

	byObject := map[client.Object]cache.ByObject{}
	add := func(obj client.Object, key string, op selection.Operator, values ...string) error {
		req, err := labels.NewRequirement(key, op, values)
		if err != nil {
			return fmt.Errorf("build cache selector for %T: %w", obj, err)
		}
		byObject[obj] = cache.ByObject{Label: labels.NewSelector().Add(*req)}
		return nil
	}

	// NotEquals also selects objects that do not carry the label at all.
	if cfg.Cache.ExcludeHelmReleaseSecrets {
		if err := add(&corev1.Secret{}, HelmReleaseOwnerLabel, selection.NotEquals, HelmReleaseOwnerValue); err != nil {
			return cache.Options{}, err
		}
	}
	if cfg.Cache.ExcludeGatewayChartConfigMaps {
		if err := add(&corev1.ConfigMap{}, GatewayChartNameLabel, selection.NotEquals, GatewayChartNameValue); err != nil {
			return cache.Options{}, err
		}
	}
	if !cfg.GatewayAPI.Enabled {
		for _, obj := range []client.Object{&corev1.Service{}, &appsv1.Deployment{}} {
			if err := add(obj, GatewayChartComponentLabel, selection.In, GatewayChartComponents...); err != nil {
				return cache.Options{}, err
			}
		}
	}
	if len(byObject) > 0 {
		opts.ByObject = byObject
	}
	return opts, nil
}
