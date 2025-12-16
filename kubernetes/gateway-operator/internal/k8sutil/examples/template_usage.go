/*
Copyright 2025.

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

package examples

import (
	"context"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/k8sutil"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Example: How to use the templated manifest applier in a Gateway controller

func ApplyGatewayInfrastructure(
	ctx context.Context,
	client client.Client,
	scheme *runtime.Scheme,
	gatewayConfig *apiv1.Gateway,
	templatePath string,
) error {
	// Create template data from the Gateway spec
	data := k8sutil.NewGatewayManifestTemplateData(gatewayConfig.Name)

	// Populate from spec
	if gatewayConfig.Spec.Infrastructure != nil {
		infra := gatewayConfig.Spec.Infrastructure

		if infra.Replicas != nil {
			data.Replicas = *infra.Replicas
		}

		if infra.Image != "" {
			data.GatewayImage = infra.Image
		}

		if infra.RouterImage != "" {
			data.RouterImage = infra.RouterImage
		}

		if infra.Resources != nil {
			data.Resources = &k8sutil.ResourceRequirements{}

			if infra.Resources.Requests != nil {
				data.Resources.Requests = &k8sutil.ResourceList{}
				if cpu := infra.Resources.Requests.Cpu(); cpu != nil {
					data.Resources.Requests.CPU = cpu.String()
				}
				if mem := infra.Resources.Requests.Memory(); mem != nil {
					data.Resources.Requests.Memory = mem.String()
				}
			}

			if infra.Resources.Limits != nil {
				data.Resources.Limits = &k8sutil.ResourceList{}
				if cpu := infra.Resources.Limits.Cpu(); cpu != nil {
					data.Resources.Limits.CPU = cpu.String()
				}
				if mem := infra.Resources.Limits.Memory(); mem != nil {
					data.Resources.Limits.Memory = mem.String()
				}
			}
		}

		if infra.NodeSelector != nil {
			data.NodeSelector = infra.NodeSelector
		}

		if infra.Tolerations != nil {
			data.Tolerations = infra.Tolerations
		}

		if infra.Affinity != nil {
			data.Affinity = infra.Affinity
		}
	}

	// Populate control plane configuration
	if gatewayConfig.Spec.ControlPlane != nil {
		cp := gatewayConfig.Spec.ControlPlane

		if cp.Host != "" {
			data.ControlPlaneHost = cp.Host
		}

		if cp.TokenSecretRef != nil {
			data.ControlPlaneTokenSecret = &k8sutil.SecretReference{
				Name: cp.TokenSecretRef.Name,
				Key:  cp.TokenSecretRef.Key,
			}
		}
	}

	// Populate storage configuration
	if gatewayConfig.Spec.Storage != nil {
		storage := gatewayConfig.Spec.Storage

		if storage.Type != "" {
			data.StorageType = storage.Type
		}

		if storage.SQLitePath != "" {
			data.StorageSQLitePath = storage.SQLitePath
		}
	}

	// Apply the templated manifest
	return k8sutil.ApplyManifestTemplate(
		ctx,
		client,
		scheme,
		templatePath,
		gatewayConfig.Namespace,
		gatewayConfig, // Set the Gateway as the owner
		data,
	)
}

// Example usage in reconciler:
//
// func (r *GatewayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
//     gatewayConfig := &apiv1.Gateway{}
//     if err := r.Get(ctx, req.NamespacedName, gatewayConfig); err != nil {
//         return ctrl.Result{}, client.IgnoreNotFound(err)
//     }
//
//     // Apply the gateway infrastructure using the template
//     templatePath := r.Config.GatewayConfig.ManifestTemplatePath // e.g., "path/to/api-platform-gateway-k8s-manifests.yaml.tmpl"
//     if err := ApplyGatewayInfrastructure(ctx, r.Client, r.Scheme, gatewayConfig, templatePath); err != nil {
//         return ctrl.Result{}, fmt.Errorf("failed to apply gateway infrastructure: %w", err)
//     }
//
//     return ctrl.Result{}, nil
// }
