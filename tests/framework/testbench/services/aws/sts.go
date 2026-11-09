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
	"crypto/rand"
	"encoding/hex"
	"encoding/xml"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// STS plays AWS Security Token Service, which the AWS policies call to assume an IAM role
// (sts-assume-role mode). The gateway runtime reaches it through AWS_ENDPOINT_URL_STS, as
// http://testbench/sts/<region>.

const stsNamespace = "https://sts.amazonaws.com/doc/2011-06-15/"

// Roles whose name selects a failure or a credential lifetime, so that a scenario can make
// credential retrieval fail, or credentials expire sooner, without shared state. Any other role
// is assumed normally.
const (
	// roleUnavailable is never assumed: STS answers 503 ServiceUnavailable.
	roleUnavailable = "testbench-sts-unavailable"
	// roleIntermittent is unavailable for the first outageDuration of every outagePeriod, by
	// the service's clock, and assumed for shortCredentialLifetime for the rest.
	roleIntermittent = "testbench-sts-intermittent"
	// roleShortLived is always assumed, for shortCredentialLifetime.
	roleShortLived = "testbench-sts-short-lived"
	// roleUnresponsive is never answered while the caller waits, up to unresponsiveHold.
	roleUnresponsive = "testbench-sts-unresponsive"
)

const (
	outagePeriod   = 20 * time.Second
	outageDuration = 10 * time.Second

	// shortCredentialLifetime makes a caller that keeps sending requests refresh its credentials
	// every few seconds. For roleIntermittent, the refresh comes within the first 3 seconds of
	// every outage, and the rest of the outage outlasts an AWS SDK's default retries of that
	// refresh (at most 6 seconds of backoff), so every attempt fails.
	shortCredentialLifetime = 3 * time.Second

	// unresponsiveHold bounds how long a request for roleUnresponsive is held before STS answers
	// 503, so that a caller with no timeout of its own does not wait forever.
	unresponsiveHold = 90 * time.Second
)

// sts answers AWS STS AssumeRole with short-lived credentials the upstream accepts. Issued
// credentials are derived from their expiry time, not stored.
type sts struct {
	verifier verifier
	// hold is how long a request for roleUnresponsive is held.
	hold time.Duration
}

// lookupFixtureCredential resolves the base credentials that sign AssumeRole. Only fixture
// credentials are accepted; this service does not support role chaining.
func lookupFixtureCredential(accessKeyID string) (credential, bool) {
	c, ok := fixtureCredentials[accessKeyID]
	return c, ok
}

type assumeRoleResponse struct {
	XMLName  xml.Name `xml:"AssumeRoleResponse"`
	XMLNS    string   `xml:"xmlns,attr"`
	Result   assumeRoleResult
	Metadata responseMetadata `xml:"ResponseMetadata"`
}

type assumeRoleResult struct {
	XMLName     xml.Name `xml:"AssumeRoleResult"`
	Credentials struct {
		AccessKeyID     string `xml:"AccessKeyId"`
		SecretAccessKey string `xml:"SecretAccessKey"`
		SessionToken    string `xml:"SessionToken"`
		Expiration      string `xml:"Expiration"`
	} `xml:"Credentials"`
	AssumedRoleUser struct {
		Arn           string `xml:"Arn"`
		AssumedRoleID string `xml:"AssumedRoleId"`
	} `xml:"AssumedRoleUser"`
}

type responseMetadata struct {
	RequestID string `xml:"RequestId"`
}

type errorResponse struct {
	XMLName xml.Name `xml:"ErrorResponse"`
	XMLNS   string   `xml:"xmlns,attr"`
	Error   struct {
		Type    string `xml:"Type"`
		Code    string `xml:"Code"`
		Message string `xml:"Message"`
	} `xml:"Error"`
	RequestID string `xml:"RequestId"`
}

// serve answers an AssumeRole request. The signature must be scoped to sts, in any region: the
// AWS SDK signs it for whichever region the policy is configured with.
func (s *sts) serve(w http.ResponseWriter, r *http.Request, _ endpoint) {
	if r.Method != http.MethodPost {
		writeXMLError(w, &awsError{status: http.StatusMethodNotAllowed, code: "InvalidAction",
			message: "STS accepts only POST requests"})
		return
	}
	body, ok := readBody(w, r, writeXMLError)
	if !ok {
		return
	}
	if _, rejected := s.verifier.verify(r, body, scope{service: "sts"}, controls{}); rejected != nil {
		writeXMLError(w, rejected)
		return
	}

	form, err := url.ParseQuery(string(body))
	if err != nil || form.Get("Action") != "AssumeRole" {
		writeXMLError(w, &awsError{status: http.StatusBadRequest, code: "InvalidAction",
			message: "Only the AssumeRole action is supported"})
		return
	}
	roleARN, sessionName := form.Get("RoleArn"), form.Get("RoleSessionName")
	if roleARN == "" || sessionName == "" {
		writeXMLError(w, &awsError{status: http.StatusBadRequest, code: "ValidationError",
			message: "RoleArn and RoleSessionName are required"})
		return
	}

	roleName := roleARN[strings.LastIndex(roleARN, "/")+1:]
	if !s.available(r, roleName) {
		writeXMLError(w, &awsError{status: http.StatusServiceUnavailable, code: "ServiceUnavailable",
			message: "Service is unable to handle request."})
		return
	}

	lifetime := issuedCredentialLifetime
	if roleName == roleIntermittent || roleName == roleShortLived {
		lifetime = shortCredentialLifetime
	}
	accessKeyID, issued := issueCredentials(s.verifier.now(), lifetime)
	response := assumeRoleResponse{XMLNS: stsNamespace, Metadata: responseMetadata{RequestID: requestID()}}
	response.Result.Credentials.AccessKeyID = accessKeyID
	response.Result.Credentials.SecretAccessKey = issued.secretAccessKey
	response.Result.Credentials.SessionToken = issued.sessionToken
	response.Result.Credentials.Expiration = issued.expiresAt.Format(time.RFC3339)
	response.Result.AssumedRoleUser.Arn = "arn:aws:sts::123456789012:assumed-role/" + roleName + "/" + sessionName
	response.Result.AssumedRoleUser.AssumedRoleID = "AROATESTASSUMEDROLE0:" + sessionName

	writeXML(w, http.StatusOK, response)
}

// available reports whether STS assumes roleName now. A request for roleUnresponsive is held
// until the caller gives up or the hold elapses, and is then unavailable.
func (s *sts) available(r *http.Request, roleName string) bool {
	switch roleName {
	case roleUnavailable:
		return false
	case roleIntermittent:
		return !inOutage(s.verifier.now())
	case roleUnresponsive:
		timer := time.NewTimer(s.hold)
		defer timer.Stop()
		select {
		case <-r.Context().Done():
		case <-timer.C:
		}
		return false
	default:
		return true
	}
}

// inOutage reports whether now falls in the outage at the start of an outagePeriod.
func inOutage(now time.Time) bool {
	return now.Sub(time.Unix(0, 0))%outagePeriod < outageDuration
}

// writeXMLError writes a rejection in the XML shape of an AWS query-protocol service.
func writeXMLError(w http.ResponseWriter, e *awsError) {
	response := errorResponse{XMLNS: stsNamespace, RequestID: requestID()}
	response.Error.Type = "Sender"
	if e.status >= http.StatusInternalServerError {
		response.Error.Type = "Receiver"
	}
	response.Error.Code = e.code
	response.Error.Message = e.message
	writeXML(w, e.status, response)
}

func writeXML(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(v)
}

func requestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
