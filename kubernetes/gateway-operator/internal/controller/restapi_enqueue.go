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

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (r *RestApiReconciler) enqueueRestApisForSecret(ctx context.Context, obj client.Object) []reconcile.Request {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return nil
	}
	return r.enqueueRestApisForValueFrom(ctx, secretKeyRefKey, secret.Namespace, secret.Name,
		client.ObjectKeyFromObject(secret).String())
}

func (r *RestApiReconciler) enqueueRestApisForConfigMap(ctx context.Context, obj client.Object) []reconcile.Request {
	cm, ok := obj.(*corev1.ConfigMap)
	if !ok {
		return nil
	}
	return r.enqueueRestApisForValueFrom(ctx, configMapKeyRefKey, cm.Namespace, cm.Name,
		client.ObjectKeyFromObject(cm).String())
}

func (r *RestApiReconciler) enqueueRestApisForValueFrom(
	ctx context.Context,
	kind, targetNS, targetName, sourceKey string,
) []reconcile.Request {
	if indexedReqs, ok := r.enqueueRestApisFromValueFromIndex(kind, targetNS, targetName); ok {
		if r.Logger != nil && len(indexedReqs) > 0 {
			names := make([]string, 0, len(indexedReqs))
			for _, q := range indexedReqs {
				names = append(names, q.NamespacedName.String())
			}
			r.Logger.Info("watch: valueFrom source changed; enqueue RestApis",
				zap.String("controller", "RestApi"),
				zap.String("kind", kind),
				zap.String("source", sourceKey),
				zap.Strings("restApis", names))
		}
		return indexedReqs
	}

	list := &apiv1.RestApiList{}
	if err := r.List(ctx, list); err != nil {
		if r.Logger != nil {
			r.Logger.Error("watch: list RestApis for valueFrom enqueue",
				zap.Error(err),
				zap.String("kind", kind),
				zap.String("source", sourceKey))
		}
		return nil
	}
	seen := make(map[types.NamespacedName]struct{})
	var reqs []reconcile.Request
	for i := range list.Items {
		api := &list.Items[i]
		r.upsertRestAPIValueFromIndex(api)
		if !restAPISpecReferencesValueFrom(&api.Spec, api.Namespace, kind, targetNS, targetName) {
			continue
		}
		nn := types.NamespacedName{Namespace: api.Namespace, Name: api.Name}
		if _, dup := seen[nn]; dup {
			continue
		}
		seen[nn] = struct{}{}
		reqs = append(reqs, reconcile.Request{NamespacedName: nn})
	}
	if r.Logger != nil && len(reqs) > 0 {
		names := make([]string, 0, len(reqs))
		for _, q := range reqs {
			names = append(names, q.NamespacedName.String())
		}
		r.Logger.Info("watch: valueFrom source changed; enqueue RestApis",
			zap.String("controller", "RestApi"),
			zap.String("kind", kind),
			zap.String("source", sourceKey),
			zap.Strings("restApis", names))
	}
	return reqs
}

func valueFromRefIndexKey(kind, targetNS, targetName string) string {
	return kind + ":" + (types.NamespacedName{Namespace: targetNS, Name: targetName}).String()
}

func (r *RestApiReconciler) enqueueRestApisFromValueFromIndex(kind, targetNS, targetName string) ([]reconcile.Request, bool) {
	key := valueFromRefIndexKey(kind, targetNS, targetName)
	r.valueFromRefIndexMu.RLock()
	bucket, ok := r.valueFromRefIndex[key]
	if !ok {
		r.valueFromRefIndexMu.RUnlock()
		return nil, false
	}
	seen := make(map[types.NamespacedName]struct{}, len(bucket))
	reqs := make([]reconcile.Request, 0, len(bucket))
	for nn := range bucket {
		if _, dup := seen[nn]; dup {
			continue
		}
		seen[nn] = struct{}{}
		reqs = append(reqs, reconcile.Request{NamespacedName: nn})
	}
	r.valueFromRefIndexMu.RUnlock()
	return reqs, true
}

func (r *RestApiReconciler) upsertRestAPIValueFromIndex(api *apiv1.RestApi) {
	if api == nil {
		return
	}
	nn := types.NamespacedName{Namespace: api.Namespace, Name: api.Name}
	newRefs := restAPISpecValueFromRefKeys(&api.Spec, api.Namespace)

	r.valueFromRefIndexMu.Lock()
	defer r.valueFromRefIndexMu.Unlock()
	if r.valueFromRefIndex == nil {
		r.valueFromRefIndex = make(map[string]map[types.NamespacedName]struct{})
	}
	if r.restAPIValueFromRef == nil {
		r.restAPIValueFromRef = make(map[types.NamespacedName]map[string]struct{})
	}

	if oldRefs, ok := r.restAPIValueFromRef[nn]; ok {
		for key := range oldRefs {
			if _, still := newRefs[key]; still {
				continue
			}
			if bucket, found := r.valueFromRefIndex[key]; found {
				delete(bucket, nn)
				if len(bucket) == 0 {
					delete(r.valueFromRefIndex, key)
				}
			}
		}
	}

	for key := range newRefs {
		bucket, ok := r.valueFromRefIndex[key]
		if !ok {
			bucket = make(map[types.NamespacedName]struct{})
			r.valueFromRefIndex[key] = bucket
		}
		bucket[nn] = struct{}{}
	}
	r.restAPIValueFromRef[nn] = newRefs
}

func (r *RestApiReconciler) removeRestAPIFromValueFromIndex(nn types.NamespacedName) {
	r.valueFromRefIndexMu.Lock()
	defer r.valueFromRefIndexMu.Unlock()
	if r.valueFromRefIndex == nil || r.restAPIValueFromRef == nil {
		return
	}
	refs, ok := r.restAPIValueFromRef[nn]
	if !ok {
		return
	}
	for key := range refs {
		if bucket, found := r.valueFromRefIndex[key]; found {
			delete(bucket, nn)
			if len(bucket) == 0 {
				delete(r.valueFromRefIndex, key)
			}
		}
	}
	delete(r.restAPIValueFromRef, nn)
}

func restAPISpecValueFromRefKeys(spec *apiv1.APIConfigData, defaultNS string) map[string]struct{} {
	refs := make(map[string]struct{})
	if spec == nil {
		return refs
	}
	for i := range spec.Policies {
		policyParamsValueFromRefKeys(&spec.Policies[i], defaultNS, refs)
	}
	for i := range spec.Operations {
		for j := range spec.Operations[i].Policies {
			policyParamsValueFromRefKeys(&spec.Operations[i].Policies[j], defaultNS, refs)
		}
	}
	return refs
}

func policyParamsValueFromRefKeys(p *apiv1.Policy, defaultNS string, refs map[string]struct{}) {
	if p == nil || p.Params == nil || len(p.Params.Raw) == 0 {
		return
	}
	var root interface{}
	if err := json.Unmarshal(p.Params.Raw, &root); err != nil {
		return
	}
	collectValueFromRefKeys(root, defaultNS, refs)
}

func collectValueFromRefKeys(v interface{}, defaultNS string, refs map[string]struct{}) {
	switch x := v.(type) {
	case map[string]interface{}:
		if rawVF, ok := x[valueFromKey]; ok {
			if inner, ok := rawVF.(map[string]interface{}); ok {
				if kind, ref, err := selectValueFromRef(inner, "RestApi"); err == nil {
					name, _, ns, err := readKeyRef(ref, defaultNS, "RestApi", kind)
					if err == nil {
						refs[valueFromRefIndexKey(kind, ns, name)] = struct{}{}
					}
				}
			}
		}
		for _, child := range x {
			collectValueFromRefKeys(child, defaultNS, refs)
		}
	case []interface{}:
		for i := range x {
			collectValueFromRefKeys(x[i], defaultNS, refs)
		}
	}
}

func restAPISpecReferencesValueFrom(spec *apiv1.APIConfigData, defaultNS, kind, targetNS, targetName string) bool {
	for i := range spec.Policies {
		if policyParamsReferencesValueFrom(&spec.Policies[i], defaultNS, kind, targetNS, targetName) {
			return true
		}
	}
	for i := range spec.Operations {
		for j := range spec.Operations[i].Policies {
			if policyParamsReferencesValueFrom(&spec.Operations[i].Policies[j], defaultNS, kind, targetNS, targetName) {
				return true
			}
		}
	}
	return false
}

func policyParamsReferencesValueFrom(p *apiv1.Policy, defaultNS, kind, targetNS, targetName string) bool {
	if p.Params == nil || len(p.Params.Raw) == 0 {
		return false
	}
	var root interface{}
	if err := json.Unmarshal(p.Params.Raw, &root); err != nil {
		return false
	}
	return jsonTreeReferencesValueFrom(root, kind, targetNS, targetName, defaultNS)
}
