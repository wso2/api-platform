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
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

// BedrockPort is the Bedrock service's container port. The gateway runtime reaches it through
// AWS_ENDPOINT_URL_BEDROCK_RUNTIME, as http://testbench:3006.
const BedrockPort = 3006

const (
	serviceBedrock = "bedrock"
	// bedrockRegion is the region the Bedrock endpoint plays, so the region a request must be
	// signed for.
	bedrockRegion = "us-east-1"
)

// Bedrock is a deterministic AWS Bedrock ApplyGuardrail-compatible service, for the AWS Bedrock
// Guardrail policy. It listens on its own port, BedrockPort, beside the AWS service on Port. Like
// Bedrock, it verifies a request's signature, then looks up the guardrail, then decides.
type Bedrock struct {
	verifier verifier
	// hold is how long a request asking Bedrock to be unresponsive is held.
	hold time.Duration
}

// NewBedrock returns a new Bedrock service.
func NewBedrock() *Bedrock { return newBedrock(time.Now) }

func newBedrock(now func() time.Time) *Bedrock {
	return &Bedrock{verifier: verifier{lookup: lookupBedrockCredential, now: now}, hold: BedrockUnresponsiveHold}
}

// BedrockUnresponsiveHold is how long Bedrock holds a request asking it to be unresponsive before
// answering 503. It must stay under the testbench server's write timeout, after which the caller
// would get a closed connection instead of the answer.
const BedrockUnresponsiveHold = 25 * time.Second

// Name returns the service registration name.
func (s *Bedrock) Name() string { return "bedrock" }

// Port returns the service's listening port.
func (s *Bedrock) Port() int { return BedrockPort }

// Stateful reports whether the service keeps request-specific state.
func (s *Bedrock) Stateful() bool { return false }

// bedrockCredentials are accepted by Bedrock only, beside the fixture and STS-issued credentials:
// suites signed their Bedrock requests with them before the service verified signatures.
var bedrockCredentials = map[string]credential{
	"AKIAIOSFODNN7TEST": {secretAccessKey: "testsecretaccesskeytestsecretaccesskey1"},
	"test-access-key":   {secretAccessKey: "test-secret-key"},
}

// lookupBedrockCredential returns the credential for an access key ID Bedrock accepts.
func lookupBedrockCredential(accessKeyID string) (credential, bool) {
	if c, ok := lookupCredential(accessKeyID); ok {
		return c, true
	}
	c, ok := bedrockCredentials[accessKeyID]
	return c, ok
}

// applyGuardrailRequest is the subset of the ApplyGuardrail request the decisions read.
type applyGuardrailRequest struct {
	Source  string `json:"source"`
	Content []struct {
		Text *struct {
			Text string `json:"text"`
		} `json:"text,omitempty"`
	} `json:"content"`
}

type applyGuardrailResponse struct {
	Action      string           `json:"action"`
	Outputs     []map[string]any `json:"outputs"`
	Assessments []assessment     `json:"assessments"`
}

type assessment struct {
	ContentPolicy              *contentPolicyAssessment              `json:"contentPolicy,omitempty"`
	TopicPolicy                *topicPolicyAssessment                `json:"topicPolicy,omitempty"`
	WordPolicy                 *wordPolicyAssessment                 `json:"wordPolicy,omitempty"`
	SensitiveInformationPolicy *sensitiveInformationPolicyAssessment `json:"sensitiveInformationPolicy,omitempty"`
}

type contentPolicyAssessment struct {
	Filters []contentFilter `json:"filters"`
}

type contentFilter struct {
	Type       string `json:"type"`
	Confidence string `json:"confidence"`
	Action     string `json:"action"`
}

type topicPolicyAssessment struct {
	Topics []deniedTopic `json:"topics"`
}

type deniedTopic struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Action string `json:"action"`
}

type wordPolicyAssessment struct {
	CustomWords []customWord `json:"customWords"`
}

type customWord struct {
	Match  string `json:"match"`
	Action string `json:"action"`
}

type sensitiveInformationPolicyAssessment struct {
	PiiEntities []piiEntity `json:"piiEntities,omitempty"`
}

type piiEntity struct {
	Type   string `json:"type"`
	Match  string `json:"match"`
	Action string `json:"action"`
}

// The fixture guardrail's denied topic. It is detected from its trigger phrase, ignoring case.
const (
	deniedTopicName    = "Investment Advice"
	deniedTopicTrigger = "investment advice"
	// blockedOutput is the guardrail's configured message for blocked content.
	blockedOutput = "Content blocked due to policy violation"
)

// fixtureGuardrailID identifies the only guardrail the account has, at the versions customWords
// lists. Bedrock answers any other identifier or version with 404 ResourceNotFoundException.
const fixtureGuardrailID = "test-guardrail-id"

// customWords are the prohibited words and phrases of each version of the fixture guardrail,
// matched anywhere in the text, ignoring case. The published version 1 is a snapshot of the draft
// taken before "secret recipe" was added to it; the versions differ in nothing else.
var customWords = map[string][]string{
	"DRAFT": {"contraband", "secret recipe"},
	"1":     {"contraband"},
}

// piiFilters are the fixture guardrail's sensitive-information filters, named by Bedrock PII
// entity type: email addresses at example.com, phone numbers and US social security numbers.
var piiFilters = []struct {
	entityType string
	pattern    *regexp.Regexp
}{
	{"EMAIL", regexp.MustCompile(`(?i)[a-z0-9._%+-]+@example\.com\b`)},
	{"PHONE", regexp.MustCompile(`\b\d{3}-\d{3}-\d{4}\b`)},
	{"US_SOCIAL_SECURITY_NUMBER", regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)},
}

// The SDK supplies the guardrail ID and version in the request path.
var guardrailPathRegex = regexp.MustCompile(`^/guardrail/([^/]+)/version/([^/]+)/apply$`)

// bedrockCodes maps three of the shared verifier's codes to the ones Bedrock Runtime answers with;
// the verifier's other codes pass through unchanged. Bedrock's API reference documents
// UnrecognizedClientException and ExpiredTokenException. It names no code for a signature that
// does not match, so InvalidSignatureException is assumed: not documented and not verified. A
// temporary key sent without its session token is assumed to get the unknown-key code too.
var bedrockCodes = map[string]string{
	codeInvalidClientTokenID:  "UnrecognizedClientException",
	codeExpiredToken:          "ExpiredTokenException",
	codeSignatureDoesNotMatch: "InvalidSignatureException",
}

// bedrockRejection returns rejected with the code Bedrock Runtime uses for it.
func bedrockRejection(rejected *awsError) *awsError {
	code, ok := bedrockCodes[rejected.code]
	if !ok {
		return rejected
	}
	translated := *rejected
	translated.code = code
	return &translated
}

// errBedrockInternal is Bedrock Runtime's answer to a request it fails to process.
var errBedrockInternal = &awsError{status: http.StatusInternalServerError, code: "InternalServerException",
	message: "Simulated AWS Bedrock Guardrail error"}

// errBedrockUnavailable is Bedrock Runtime's answer while it is unavailable.
var errBedrockUnavailable = &awsError{status: http.StatusServiceUnavailable, code: "ServiceUnavailableException",
	message: "The service is temporarily unavailable. Try again later."}

// Handler returns the ApplyGuardrail endpoint.
func (s *Bedrock) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.health)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		match := guardrailPathRegex.FindStringSubmatch(r.URL.Path)
		if match == nil {
			http.NotFound(w, r)
			return
		}
		if strings.ToUpper(r.Method) != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.apply(w, r, match[1], match[2])
	})
	return mux
}

func (s *Bedrock) health(w http.ResponseWriter, r *http.Request) {
	if strings.ToUpper(r.Method) != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, map[string]string{
		"status":  "ok",
		"service": "bedrock",
	})
}

// apply verifies an ApplyGuardrail request for the guardrail and version in its path, looks the
// guardrail up and returns a deterministic decision for the supplied text.
func (s *Bedrock) apply(w http.ResponseWriter, r *http.Request, guardrailID, version string) {
	body, ok := readBody(w, r, writePlainError)
	if !ok {
		return
	}
	if _, rejected := s.verifier.verify(r, body, scope{service: serviceBedrock, region: bedrockRegion}, controls{}); rejected != nil {
		writeJSONError(w, bedrockRejection(rejected))
		return
	}
	words, versionExists := customWords[version]
	if guardrailID != fixtureGuardrailID || !versionExists {
		writeJSONError(w, &awsError{status: http.StatusNotFound, code: codeResourceNotFound,
			message: "The guardrail with the given identifier and version could not be found"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	var req applyGuardrailRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON request", http.StatusBadRequest)
		return
	}

	var content []string
	for _, item := range req.Content {
		if item.Text != nil {
			content = append(content, item.Text.Text)
		}
	}
	text := strings.Join(content, " ")
	if text == "" {
		writeJSON(w, none(text))
		return
	}

	lower := strings.ToLower(text)

	if strings.Contains(lower, "error") && strings.Contains(lower, "simulate") {
		writeJSONError(w, errBedrockInternal)
		return
	}
	// A text asking for an outage finds Bedrock unavailable. One asking for it to be unresponsive
	// gets no answer until the caller gives up or the hold elapses, and then finds it unavailable.
	if strings.Contains(lower, "simulate") && strings.Contains(lower, "outage") {
		writeJSONError(w, errBedrockUnavailable)
		return
	}
	if strings.Contains(lower, "simulate") && strings.Contains(lower, "unresponsive") {
		timer := time.NewTimer(s.hold)
		defer timer.Stop()
		select {
		case <-r.Context().Done():
		case <-timer.C:
		}
		writeJSONError(w, errBedrockUnavailable)
		return
	}

	// Like Bedrock, every policy that matches is reported in the one assessment, and any blocking
	// match blocks the text. Keyword matching intentionally uses substrings to preserve the legacy
	// mock's behavior.
	var a assessment
	block := false
	if strings.Contains(lower, "violence") || strings.Contains(lower, "hate") ||
		strings.Contains(lower, "illegal") {
		a.ContentPolicy = &contentPolicyAssessment{
			Filters: []contentFilter{{Type: "VIOLENCE", Confidence: "HIGH", Action: "BLOCKED"}},
		}
		block = true
	}
	if strings.Contains(lower, deniedTopicTrigger) {
		a.TopicPolicy = &topicPolicyAssessment{
			Topics: []deniedTopic{{Name: deniedTopicName, Type: "DENY", Action: "BLOCKED"}},
		}
		block = true
	}
	for _, word := range words {
		if strings.Contains(lower, word) {
			if a.WordPolicy == nil {
				a.WordPolicy = &wordPolicyAssessment{}
			}
			a.WordPolicy.CustomWords = append(a.WordPolicy.CustomWords, customWord{Match: word, Action: "BLOCKED"})
			block = true
		}
	}
	if entities := detectPII(text); len(entities) > 0 {
		a.SensitiveInformationPolicy = &sensitiveInformationPolicyAssessment{PiiEntities: entities}
	}

	switch {
	case block:
		writeJSON(w, blocked(a))
	case a.SensitiveInformationPolicy != nil:
		// Check masking before redaction because both match example.com addresses.
		writeJSON(w, piiIntervened(text, a, strings.Contains(lower, "mask-")))
	default:
		writeJSON(w, none(text))
	}
}

// none is the "no violation" decision. Real Bedrock always echoes the (possibly unmodified)
// text back in outputs[0] even when action is NONE; an empty outputs array here previously
// caused the caller to dereference a non-existent first element.
func none(text string) applyGuardrailResponse {
	return applyGuardrailResponse{
		Action:      "NONE",
		Outputs:     []map[string]any{{"text": text}},
		Assessments: []assessment{},
	}
}

// blocked is an intervention that blocks the text, reported by the given assessment.
func blocked(a assessment) applyGuardrailResponse {
	return applyGuardrailResponse{
		Action:      "GUARDRAIL_INTERVENED",
		Outputs:     []map[string]any{{"text": blockedOutput}},
		Assessments: []assessment{a},
	}
}

// detectPII returns every match of the PII filters, in the order they appear in the text.
func detectPII(text string) []piiEntity {
	type found struct {
		at     int
		entity piiEntity
	}
	var all []found
	for _, filter := range piiFilters {
		for _, loc := range filter.pattern.FindAllStringIndex(text, -1) {
			all = append(all, found{at: loc[0], entity: piiEntity{
				Type: filter.entityType, Match: text[loc[0]:loc[1]], Action: "ANONYMIZED",
			}})
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].at < all[j].at })
	entities := make([]piiEntity, 0, len(all))
	for _, f := range all {
		entities = append(entities, f.entity)
	}
	return entities
}

// piiIntervened anonymizes the assessment's PII entities. The output text replaces each match
// with an anonymized placeholder when the text asks for masking ("mask-"), else "*****".
func piiIntervened(text string, a assessment, mask bool) applyGuardrailResponse {
	for _, e := range a.SensitiveInformationPolicy.PiiEntities {
		replacement := "*****"
		if mask {
			replacement = "$ANONYMIZED_" + e.Type + "$"
		}
		text = strings.ReplaceAll(text, e.Match, replacement)
	}
	return applyGuardrailResponse{
		Action:      "GUARDRAIL_INTERVENED",
		Outputs:     []map[string]any{{"text": text}},
		Assessments: []assessment{a},
	}
}

// writePlainError writes a failure to read the request body as plain text, as the ApplyGuardrail
// endpoint here always has.
func writePlainError(w http.ResponseWriter, e *awsError) {
	http.Error(w, e.message, e.status)
}

func writeJSON(w http.ResponseWriter, v any) {
	_ = json.NewEncoder(w).Encode(v)
}
