/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package soap is the gateway's internal SOAP system policy: the parts of serving a SOAP
// API the gateway does itself rather than leaving to the backend. Today that is one job,
// pointing the endpoint addresses in a service description (WSDL) at the gateway, so a
// client generated from it (SoapUI, wsimport, svcutil) calls the gateway. It is named for
// the protocol so a later SOAP concern joins it instead of becoming another policy, module
// and build-lock line.
//
// The controller attaches it to the GET route of every SoapApi, never an API author.
package soap

import (
	"context"
	"log/slog"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// Parameter names, as the controller writes them.
const (
	ParamWSDLRewrite      = "wsdlRewrite"
	ParamGatewayPath      = "gatewayPath"
	ParamUpstreamURL      = "upstreamUrl"
	ParamVhost            = "vhost"
	ParamMaxResponseBytes = "maxResponseBytes"
)

const cacheControlNoStore = "no-store"

// staleResponseValidators described the backend's own bytes and cannot describe the
// rewritten ones; a client revalidating with one would get the unrewritten document back.
var staleResponseValidators = []string{
	"etag",
	"last-modified",
	"digest",
	"content-digest",
	"content-md5",
}

// SOAPSystemPolicy rewrites service descriptions.
//
// The rewrite configuration is read when the instance is built, not per request, because
// it decides Mode(): only an instance that rewrites buffers the response body, so no other
// chain carries a response-body requirement it has no use for.
type SOAPSystemPolicy struct {
	// wsdl is non-nil on an instance whose parameters asked for rewriting.
	wsdl *wsdlRewriteConfig
	// configErr records a wsdlRewrite block that was present but unusable. Held rather
	// than returned from the factory, so a defect in one API's parameters fails that
	// route's service description closed instead of failing the whole chain build.
	configErr error
}

var ins = &SOAPSystemPolicy{}

// GetPolicy returns the instance for one attachment of the policy.
func GetPolicy(_ policy.PolicyMetadata, params map[string]any) (policy.Policy, error) {
	cfg, err := parseWSDLRewriteConfig(params)
	if err != nil {
		return &SOAPSystemPolicy{configErr: err}, nil
	}
	if cfg == nil {
		return ins, nil
	}
	return &SOAPSystemPolicy{wsdl: cfg}, nil
}

// GetPolicyV2 is an alias for GetPolicy, provided for compatibility with plugin registries
// that call GetPolicyV2 on all plugins.
func GetPolicyV2(metadata policy.PolicyMetadata, params map[string]any) (policy.Policy, error) {
	return GetPolicy(metadata, params)
}

// Mode declares the response body phase only for an instance that rewrites, or that was
// configured to and cannot (which must fail its response rather than forward it). Neither
// request phase is needed: everything the rewrite depends on is a parameter or is read
// from the downstream request snapshot.
func (p *SOAPSystemPolicy) Mode() policy.ProcessingMode {
	responseBodyMode := policy.BodyModeSkip
	if p.wsdl != nil || p.configErr != nil {
		responseBodyMode = policy.BodyModeBuffer
	}
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeSkip,
		RequestBodyMode:    policy.BodyModeSkip,
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   responseBodyMode,
	}
}

// OnResponseBody rewrites a successful service description.
//
// Anything that is not one — an error status, an empty body, a document of another kind —
// is forwarded untouched. A service description that cannot be rewritten safely fails with
// a 502 rather than being forwarded with the backend's URLs in it.
func (p *SOAPSystemPolicy) OnResponseBody(
	ctx context.Context,
	respCtx *policy.ResponseContext,
	_ map[string]any,
) policy.ResponseAction {
	if respCtx.ResponseStatus < 200 || respCtx.ResponseStatus > 299 {
		return policy.DownstreamResponseModifications{}
	}
	if respCtx.ResponseBody == nil || len(respCtx.ResponseBody.Content) == 0 {
		return policy.DownstreamResponseModifications{}
	}

	apiID := ""
	if respCtx.SharedContext != nil {
		apiID = respCtx.APIId
	}

	if p.configErr != nil {
		slog.Error("SOAP system policy: invalid wsdlRewrite configuration; refusing to serve the service description",
			"api_id", apiID, "path", respCtx.RequestPath, "error", p.configErr)
		return unavailableResponse()
	}
	if p.wsdl == nil {
		// Unreachable: an instance with nothing to do declares ResponseBodyMode: Skip.
		return policy.DownstreamResponseModifications{}
	}

	scheme, authority := downstreamOrigin(respCtx)
	rewritten, err := p.wsdl.rewrite(ctx, respCtx.ResponseBody.Content, scheme, authority)
	if err != nil {
		// Logged, not answered: the reason names the backend's document or the gateway's
		// configuration, neither of which a client should read off a response.
		slog.Error("SOAP system policy: cannot rewrite the service description; refusing to serve it",
			"api_id", apiID, "path", respCtx.RequestPath, "error", err)
		return unavailableResponse()
	}
	if rewritten == nil {
		return policy.DownstreamResponseModifications{}
	}

	// Content-Length is recomputed by the engine when the body is replaced, and the body is
	// re-compressed for the client if the backend compressed it. What is handled here is
	// what described the backend's bytes: validators are dropped, and the response is not
	// cached, since the rewritten URLs depend on how this request reached the gateway.
	return policy.DownstreamResponseModifications{
		Body:            rewritten,
		HeadersToSet:    map[string]string{"cache-control": cacheControlNoStore},
		HeadersToRemove: staleResponseValidators,
	}
}

// downstreamOrigin is the scheme and authority of the client's request, from the snapshot
// the engine captured before any policy mutated it — so a policy that rewrote the host on
// its way upstream cannot change what the description advertises.
func downstreamOrigin(respCtx *policy.ResponseContext) (scheme, authority string) {
	if respCtx.Downstream != nil && respCtx.Downstream.Request != nil {
		return respCtx.Downstream.Request.Scheme, respCtx.Downstream.Request.Authority
	}
	return "", ""
}

// unavailableResponse is the answer for a service description the gateway will not serve.
// It names no reason — that is in the log. It describes the failure, so on a SoapApi route
// the engine renders it as a SOAP Fault; the body is the fallback for a gateway that does
// not render.
func unavailableResponse() policy.ImmediateResponse {
	return policy.ImmediateResponse{
		StatusCode: 502,
		Headers: map[string]string{
			"content-type":  "application/json",
			"cache-control": cacheControlNoStore,
		},
		Body: []byte(`{"error":"bad_gateway","message":"The service description could not be served."}`),
		Fault: &policy.FaultDetails{
			Code:    policy.FaultCodeUpstreamGeneric,
			Type:    policy.FaultTypeUpstream,
			Message: "The service description could not be served.",
		},
	}
}
