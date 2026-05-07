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
	"errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/gatewayclient"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/secretsource"
)

// classifySecretSourceResolveError maps secretsource.Resolve / ResolveOptional
// failures from reading SecretValueSource: terminal Kubernetes API failures and
// invalid specs use NonRetryableError; transient Get failures propagate as-is so
// the generic reconciler retries them.
func classifySecretSourceResolveError(err error) error {
	if err == nil {
		return nil
	}
	if apierrors.IsNotFound(err) || apierrors.IsForbidden(err) || apierrors.IsInvalid(err) {
		return &gatewayclient.NonRetryableError{Err: err}
	}
	var missing *secretsource.ErrSecretMissing
	if errors.As(err, &missing) && missing.Cause != nil {
		c := missing.Cause
		if apierrors.IsNotFound(c) || apierrors.IsForbidden(c) || apierrors.IsInvalid(c) {
			return &gatewayclient.NonRetryableError{Err: err}
		}
	}

	var ambiguous *secretsource.ErrAmbiguous
	var notCfg *secretsource.ErrNotConfigured
	var keyMiss *secretsource.ErrKeyMissing
	if errors.As(err, &ambiguous) || errors.As(err, &notCfg) || errors.As(err, &keyMiss) {
		return &gatewayclient.NonRetryableError{Err: err}
	}

	return err
}
