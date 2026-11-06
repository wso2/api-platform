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

package aws

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// The upstream plays an AWS-hosted backend, such as an API Gateway API, that the gateway forwards
// API traffic to: the AWS Authentication policy's upstream. Features address it as
// http://testbench/<service>/<region>, for example http://testbench/execute-api/us-east-1.

const (
	// HeaderAccessKeyID carries the access key ID of a request whose signature verified.
	HeaderAccessKeyID = "X-Aws-Access-Key-Id"
	// HeaderRevokedAccessKeys lists access key IDs the upstream treats as deactivated.
	HeaderRevokedAccessKeys = "X-Testbench-Revoked-Access-Keys"
	// HeaderSignerClockSkew is a Go duration by which the upstream takes the signer's clock to run
	// ahead of its own (negative when behind).
	HeaderSignerClockSkew = "X-Testbench-Signer-Clock-Skew"
)

// nonexistentResource is the resource path, below /<service>/<region>, the upstream does not have.
const nonexistentResource = "/nonexistent"

// upstream is an AWS-hosted backend that accepts only correctly SigV4-signed requests.
type upstream struct {
	verifier verifier
}

// reflection is the JSON body returned for a verified request.
type reflection struct {
	Method      string              `json:"method"`
	Path        string              `json:"path"`
	Query       string              `json:"query,omitempty"`
	Headers     map[string][]string `json:"headers"`
	Body        string              `json:"body,omitempty"`
	AccessKeyID string              `json:"accessKeyId"`
	Service     string              `json:"service"`
	Region      string              `json:"region"`
}

// serve verifies a request signed for the endpoint's own service and region, authorizes it, and
// reflects it.
func (u *upstream) serve(w http.ResponseWriter, r *http.Request, ep endpoint) {
	body, ok := readBody(w, r, writeJSONError)
	if !ok {
		return
	}

	// Like AWS, the upstream authenticates a request before anything else, so a malformed test
	// control is reported only for a request whose signature verifies.
	ctl, controlErr := requestControls(r)
	id, rejected := u.verifier.verify(r, body, scope{service: ep.service, region: ep.region}, ctl)
	if rejected != nil {
		writeJSONError(w, rejected)
		return
	}
	if controlErr != nil {
		writeJSONError(w, controlErr)
		return
	}
	// As AWS does, the upstream authorizes an authenticated caller before it looks up the resource.
	resource := ep.rest
	if resource == "" {
		resource = "/"
	}
	if cred, _ := u.verifier.lookup(id.accessKeyID); !cred.permits(resource) {
		writeJSONError(w, &awsError{status: http.StatusForbidden, code: codeAccessDenied,
			message: fmt.Sprintf("User is not authorized to perform: %s:Invoke on resource: %s %s",
				id.service, r.Method, resource)})
		return
	}
	if underPath(resource, nonexistentResource) {
		writeJSONError(w, &awsError{status: http.StatusNotFound, code: codeResourceNotFound,
			message: "Requested resource not found"})
		return
	}

	headers := r.Header.Clone()
	headers.Set("Host", r.Host)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set(HeaderAccessKeyID, id.accessKeyID)
	_ = json.NewEncoder(w).Encode(reflection{
		Method:      r.Method,
		Path:        r.URL.Path,
		Query:       r.URL.RawQuery,
		Headers:     headers,
		Body:        string(body),
		AccessKeyID: id.accessKeyID,
		Service:     id.service,
		Region:      id.region,
	})
}

// requestControls parses the request's test-control headers. A malformed header is returned as an
// error, with the controls that did parse.
func requestControls(r *http.Request) (controls, *awsError) {
	ctl := controls{revoked: revokedKeys(r)}
	values := r.Header.Values(HeaderSignerClockSkew)
	if len(values) == 0 {
		return ctl, nil
	}
	// One request has one signer clock, so the header must carry exactly one duration.
	malformed := &awsError{status: http.StatusBadRequest, code: "InvalidRequest",
		message: HeaderSignerClockSkew + " must be a single Go duration"}
	if len(values) > 1 {
		return ctl, malformed
	}
	skew, err := time.ParseDuration(values[0])
	if err != nil {
		return ctl, malformed
	}
	ctl.signerClockSkew = skew
	return ctl, nil
}

// revokedKeys parses the X-Testbench-Revoked-Access-Keys request header.
func revokedKeys(r *http.Request) map[string]bool {
	revoked := map[string]bool{}
	for _, value := range r.Header.Values(HeaderRevokedAccessKeys) {
		for _, key := range strings.Split(value, ",") {
			if key = strings.TrimSpace(key); key != "" {
				revoked[key] = true
			}
		}
	}
	return revoked
}
