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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/gatewayclient"
)

func TestRewriteLLMDeployDependencyErrors(t *testing.T) {
	tmplMiss := &gatewayclient.NonRetryableError{
		StatusCode: http.StatusBadRequest,
		Err:        errors.New(`gateway returned status 400: {"message":"failed to retrieve template 'openai-test': configuration not found"}`),
	}
	var retry *gatewayclient.RetryableError
	require.True(t, errors.As(rewriteLLMDeployDependencyErrors(tmplMiss), &retry))

	proxyProvMiss := &gatewayclient.NonRetryableError{
		StatusCode: http.StatusNotFound,
		Err:        errors.New(`gateway returned status 404: {"message":"LLM proxy configuration not found","status":"error"}`),
	}
	require.True(t, errors.As(rewriteLLMDeployDependencyErrors(proxyProvMiss), &retry))

	getByHandle := &gatewayclient.NonRetryableError{
		StatusCode: http.StatusNotFound,
		Err:        errors.New(`gateway returned status 404: {"message":"LLM proxy configuration with handle 'x' not found"}`),
	}
	require.False(t, errors.As(rewriteLLMDeployDependencyErrors(getByHandle), &retry))

	other400 := &gatewayclient.NonRetryableError{
		StatusCode: http.StatusBadRequest,
		Err:        errors.New(`gateway returned status 400: {"message":"invalid yaml"}`),
	}
	require.Same(t, other400, rewriteLLMDeployDependencyErrors(other400))
}
