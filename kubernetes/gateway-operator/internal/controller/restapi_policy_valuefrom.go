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

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const annRestApiPolicyValueFromFingerprint = "gateway.api-platform.wso2.com/policy-valuefrom-fingerprint"

type valueFromBackingID struct {
	secret bool // false => ConfigMap
	ns     string
	name   string
}

func (id valueFromBackingID) key() string {
	k := "cm"
	if id.secret {
		k = "sec"
	}
	return k + ":" + id.ns + "/" + id.name
}

// computeRestApiPolicyValueFromFingerprint returns a deterministic summary of referenced
// Secret / ConfigMap resourceVersions so reconcile can redeploy after backing data rotates
// without a RestApi spec generation bump.
func computeRestApiPolicyValueFromFingerprint(
	ctx context.Context,
	c client.Client,
	defaultNS string,
	spec *apiv1.APIConfigData,
) (string, error) {
	backing := map[string]valueFromBackingID{}
	for i := range spec.Policies {
		jsonTreeAccumulateBackingIDsFromPolicyParams(&spec.Policies[i], defaultNS, backing)
	}
	for i := range spec.Operations {
		for j := range spec.Operations[i].Policies {
			jsonTreeAccumulateBackingIDsFromPolicyParams(&spec.Operations[i].Policies[j], defaultNS, backing)
		}
	}
	var lines []string
	for _, id := range backing {
		rv, err := fetchBackingResourceVersion(ctx, c, id)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return "", fmt.Errorf("policy valueFrom backing %s missing: %w", id.key(), err)
			}
			return "", err
		}
		tag := "configmap"
		if id.secret {
			tag = "secret"
		}
		lines = append(lines, fmt.Sprintf("%s:%s/%s@%s", tag, id.ns, id.name, rv))
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n"), nil
}

func fetchBackingResourceVersion(ctx context.Context, c client.Client, id valueFromBackingID) (string, error) {
	nn := types.NamespacedName{Namespace: id.ns, Name: id.name}
	if id.secret {
		var sec corev1.Secret
		if err := c.Get(ctx, nn, &sec); err != nil {
			return "", err
		}
		return sec.ResourceVersion, nil
	}
	var cm corev1.ConfigMap
	if err := c.Get(ctx, nn, &cm); err != nil {
		return "", err
	}
	return cm.ResourceVersion, nil
}

func jsonTreeAccumulateBackingIDsFromPolicyParams(p *apiv1.Policy, defaultNS string, dst map[string]valueFromBackingID) {
	if p.Params == nil || len(p.Params.Raw) == 0 {
		return
	}
	var root interface{}
	if err := json.Unmarshal(p.Params.Raw, &root); err != nil {
		return
	}
	jsonTreeAccumulateValueFromBackingIDs(root, defaultNS, dst)
}

func jsonTreeAccumulateValueFromBackingIDs(v interface{}, defaultNS string, dst map[string]valueFromBackingID) {
	switch x := v.(type) {
	case map[string]interface{}:
		if vf, ok := x[valueFromKey]; ok {
			if inner, ok := vf.(map[string]interface{}); ok {
				if ref, ok := inner[secretKeyRefKey].(map[string]interface{}); ok {
					if nm, ns, ok2 := normalizedKeyRefNameNamespace(ref, defaultNS); ok2 {
						id := valueFromBackingID{secret: true, ns: ns, name: nm}
						dst[id.key()] = id
					}
				}
				if ref, ok := inner[configMapKeyRefKey].(map[string]interface{}); ok {
					if nm, ns, ok2 := normalizedKeyRefNameNamespace(ref, defaultNS); ok2 {
						id := valueFromBackingID{secret: false, ns: ns, name: nm}
						dst[id.key()] = id
					}
				}
			}
		}
		for _, child := range x {
			jsonTreeAccumulateValueFromBackingIDs(child, defaultNS, dst)
		}
	case []interface{}:
		for _, el := range x {
			jsonTreeAccumulateValueFromBackingIDs(el, defaultNS, dst)
		}
	}
}

func normalizedKeyRefNameNamespace(ref map[string]interface{}, defaultNS string) (name, ns string, ok bool) {
	rawName, _ := ref[keyRefFieldName].(string)
	name = strings.TrimSpace(rawName)
	if name == "" {
		return "", "", false
	}
	ns = strings.TrimSpace(defaultNS)
	if n, exists := ref[keyRefFieldNamespac].(string); exists && strings.TrimSpace(n) != "" {
		ns = strings.TrimSpace(n)
	}
	if ns == "" {
		return "", "", false
	}
	return name, ns, true
}

func restApiPolicyValueFromAnnotation(api *apiv1.RestApi) string {
	if api.Annotations == nil {
		return ""
	}
	return strings.TrimSpace(api.Annotations[annRestApiPolicyValueFromFingerprint])
}

// patchRestApiPolicyValueFromFingerprintAnnotation updates the fingerprint annotation with
// the exact fingerprint captured during the deployed valueFrom resolution pass.
func patchRestApiPolicyValueFromFingerprintAnnotation(
	ctx context.Context,
	c client.Client,
	apiNN types.NamespacedName,
	fp string,
) error {
	api := &apiv1.RestApi{}
	if err := c.Get(ctx, apiNN, api); err != nil {
		return err
	}
	base := api.DeepCopy()
	if api.Annotations == nil {
		api.Annotations = map[string]string{}
	}
	prev := strings.TrimSpace(api.Annotations[annRestApiPolicyValueFromFingerprint])
	if prev == fp {
		return nil
	}
	api.Annotations[annRestApiPolicyValueFromFingerprint] = fp
	return c.Patch(ctx, api, client.MergeFrom(base))
}

func programmedTrueForGeneration(apiConfig *apiv1.RestApi, generation int64) bool {
	c := meta.FindStatusCondition(apiConfig.Status.Conditions, apiv1.APIConditionProgrammed)
	return c != nil && c.Status == metav1.ConditionTrue && c.ObservedGeneration == generation
}
