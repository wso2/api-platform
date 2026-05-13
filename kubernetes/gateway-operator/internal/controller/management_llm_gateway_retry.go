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
	"net/http"
	"strings"

	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/gatewayclient"
)

// rewriteLLMDeployDependencyErrors turns selected gateway "client" errors into
// RetryableError when failure is due to another management resource not yet
// visible on the gateway (parallel reconcile / apply order).
func rewriteLLMDeployDependencyErrors(err error) error {
	var nr *gatewayclient.NonRetryableError
	if !errors.As(err, &nr) {
		return err
	}
	msg := nr.Err.Error()
	switch nr.StatusCode {
	case http.StatusBadRequest:
		// LlmProvider create/update when LlmProviderTemplate is still deploying.
		if strings.Contains(msg, "failed to retrieve template") {
			return &gatewayclient.RetryableError{Err: nr.Err, StatusCode: nr.StatusCode}
		}
	case http.StatusNotFound:
		// LLM proxy create: referenced provider missing — same HTTP body as a
		// missing proxy on GET is distinguished by the "with handle" suffix.
		if strings.Contains(msg, "LLM proxy configuration not found") &&
			!strings.Contains(msg, "with handle") {
			return &gatewayclient.RetryableError{Err: nr.Err, StatusCode: nr.StatusCode}
		}
	}
	return err
}
