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

package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
)

// k8sGatewayReconcilePredicate drops status-only updates so status patches do not
// re-run Helm on every Gateway condition change.
func k8sGatewayReconcilePredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool { return true },
		DeleteFunc: func(e event.DeleteEvent) bool { return true },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldG, ok1 := e.ObjectOld.(*gatewayv1.Gateway)
			newG, ok2 := e.ObjectNew.(*gatewayv1.Gateway)
			if !ok1 || !ok2 {
				return true
			}
			if oldG.Generation != newG.Generation {
				return true
			}
			return k8sGatewayHelmDrivingMetaChanged(oldG, newG)
		},
		GenericFunc: func(ge event.GenericEvent) bool { return true },
	}
}

func k8sGatewayHelmDrivingMetaChanged(oldG, newG *gatewayv1.Gateway) bool {
	a, b := oldG.Annotations, newG.Annotations
	if a == nil {
		a = map[string]string{}
	}
	if b == nil {
		b = map[string]string{}
	}
	return a[AnnK8sGatewayHelmValuesConfigMap] != b[AnnK8sGatewayHelmValuesConfigMap] ||
		a[AnnK8sGatewayControlPlaneHost] != b[AnnK8sGatewayControlPlaneHost] ||
		a[AnnK8sGatewayAPISelector] != b[AnnK8sGatewayAPISelector] ||
		a[AnnK8sGatewayHelmValuesHash] != b[AnnK8sGatewayHelmValuesHash]
}

func helmInstallSignature(valuesYAML, valuesFilePath string, cfg *config.OperatorConfig, gw *gatewayv1.Gateway) (string, error) {
	h := sha256.New()
	h.Write([]byte(cfg.Gateway.HelmChartName))
	h.Write([]byte{0})
	h.Write([]byte(cfg.Gateway.HelmChartVersion))
	h.Write([]byte{0})
	h.Write([]byte(strconv.FormatInt(gw.Generation, 10)))
	h.Write([]byte{0})
	cmRef := ""
	if gw.Annotations != nil {
		cmRef = gw.Annotations[AnnK8sGatewayHelmValuesConfigMap]
	}
	h.Write([]byte(cmRef))
	h.Write([]byte{0})

	// Include spec.infrastructure labels/annotations in the signature so changes trigger reconciliation.
	if gw.Spec.Infrastructure != nil {
		if len(gw.Spec.Infrastructure.Labels) > 0 {
			data, err := json.Marshal(gw.Spec.Infrastructure.Labels)
			if err != nil {
				return "", fmt.Errorf("marshal infra labels for signature: %w", err)
			}
			h.Write([]byte("infra-labels\x00"))
			h.Write(data)
		}
		if len(gw.Spec.Infrastructure.Annotations) > 0 {
			data, err := json.Marshal(gw.Spec.Infrastructure.Annotations)
			if err != nil {
				return "", fmt.Errorf("marshal infra annotations for signature: %w", err)
			}
			h.Write([]byte("infra-annotations\x00"))
			h.Write(data)
		}
	}

	hasValues := false
	if valuesFilePath != "" {
		data, err := os.ReadFile(valuesFilePath)
		if err != nil {
			return "", fmt.Errorf("read helm values file for signature: %w", err)
		}
		h.Write([]byte("file\x00"))
		h.Write(data)
		hasValues = true
	}
	if valuesYAML != "" {
		h.Write([]byte("yaml\x00"))
		h.Write([]byte(valuesYAML))
		hasValues = true
	}
	if !hasValues {
		h.Write([]byte("<no-values>"))
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func (r *K8sGatewayReconciler) patchHelmValuesHash(ctx context.Context, gw *gatewayv1.Gateway, hash string) error {
	latest := &gatewayv1.Gateway{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(gw), latest); err != nil {
		return err
	}
	base := latest.DeepCopy()
	if latest.Annotations == nil {
		latest.Annotations = map[string]string{}
	}
	if latest.Annotations[AnnK8sGatewayHelmValuesHash] == hash {
		return nil
	}
	latest.Annotations[AnnK8sGatewayHelmValuesHash] = hash
	return r.Patch(ctx, latest, client.MergeFrom(base))
}
