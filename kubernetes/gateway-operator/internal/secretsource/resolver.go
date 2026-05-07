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

// Package secretsource resolves SecretValueSource fields used by the new
// management-API CRDs. Inline values are returned verbatim; valueFrom
// selectors are read from a core/v1 Secret in the owning CR's namespace.
package secretsource

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
)

// ErrNotConfigured indicates that neither value nor valueFrom is set.
type ErrNotConfigured struct{ Field string }

func (e *ErrNotConfigured) Error() string {
	return fmt.Sprintf("%s: neither value nor valueFrom is set", e.Field)
}

// ErrAmbiguous indicates that both value and valueFrom are set.
type ErrAmbiguous struct{ Field string }

func (e *ErrAmbiguous) Error() string {
	return fmt.Sprintf("%s: both value and valueFrom are set", e.Field)
}

// ErrSecretMissing indicates a referenced Secret could not be located.
type ErrSecretMissing struct {
	Field     string
	Namespace string
	Name      string
	Cause     error
}

func (e *ErrSecretMissing) Error() string {
	return fmt.Sprintf("%s: get secret %s/%s: %v", e.Field, e.Namespace, e.Name, e.Cause)
}

// ErrKeyMissing indicates a key is missing from the referenced Secret.
type ErrKeyMissing struct {
	Field     string
	Namespace string
	Name      string
	Key       string
}

func (e *ErrKeyMissing) Error() string {
	return fmt.Sprintf("%s: secret %s/%s does not contain key %q", e.Field, e.Namespace, e.Name, e.Key)
}

// Resolve returns the plaintext value for src.
//
// fallbackNamespace is used when src.ValueFrom omits the namespace; pass the
// owning CR's namespace.
//
// fieldPath is included in error messages to make missing/ambiguous fields
// easier to diagnose.
func Resolve(ctx context.Context, k8sClient client.Client, fieldPath string, src apiv1.SecretValueSource, fallbackNamespace string) (string, error) {
	hasInline := src.Value != nil
	hasRef := src.ValueFrom != nil

	switch {
	case hasInline && hasRef:
		return "", &ErrAmbiguous{Field: fieldPath}
	case hasInline:
		return *src.Value, nil
	case hasRef:
		return resolveSecretRef(ctx, k8sClient, fieldPath, src.ValueFrom, fallbackNamespace)
	default:
		return "", &ErrNotConfigured{Field: fieldPath}
	}
}

// ResolveOptional behaves like Resolve but returns ("", nil) when src is nil
// or has neither value nor valueFrom set. Callers use it for optional
// SecretValueSource fields (e.g. ApiKey.Spec.ApiKey).
func ResolveOptional(ctx context.Context, k8sClient client.Client, fieldPath string, src *apiv1.SecretValueSource, fallbackNamespace string) (string, bool, error) {
	if src == nil {
		return "", false, nil
	}
	if src.Value == nil && src.ValueFrom == nil {
		return "", false, nil
	}
	val, err := Resolve(ctx, k8sClient, fieldPath, *src, fallbackNamespace)
	if err != nil {
		return "", false, err
	}
	return val, true, nil
}

func resolveSecretRef(ctx context.Context, k8sClient client.Client, fieldPath string, sel *corev1.SecretKeySelector, fallbackNamespace string) (string, error) {
	if sel.Name == "" {
		return "", &ErrNotConfigured{Field: fieldPath + ".valueFrom.secretKeyRef.name"}
	}
	if sel.Key == "" {
		return "", &ErrNotConfigured{Field: fieldPath + ".valueFrom.secretKeyRef.key"}
	}
	ns := fallbackNamespace
	// SecretKeySelector embeds LocalObjectReference; namespace is implicit
	// (same namespace as the owning CR). The fallback covers that case
	// directly without requiring callers to mutate the selector.
	secret := &corev1.Secret{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: sel.Name}, secret); err != nil {
		if apierrors.IsNotFound(err) {
			if sel.Optional != nil && *sel.Optional {
				return "", nil
			}
			return "", &ErrSecretMissing{Field: fieldPath, Namespace: ns, Name: sel.Name, Cause: err}
		}
		return "", err
	}
	if v, ok := secret.Data[sel.Key]; ok {
		return string(v), nil
	}
	if v, ok := secret.StringData[sel.Key]; ok {
		return v, nil
	}
	if sel.Optional != nil && *sel.Optional {
		return "", nil
	}
	return "", &ErrKeyMissing{Field: fieldPath, Namespace: ns, Name: sel.Name, Key: sel.Key}
}
