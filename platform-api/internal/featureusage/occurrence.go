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

package featureusage

import (
	"fmt"
	"maps"
	"regexp"
	"time"

	"github.com/google/uuid"
)

// Occurrence is one use of a taxonomy feature through a REST API call —
// successful or not; the call's status says which — as the handler that
// served it describes it: platform identifiers, the call itself, and the
// feature's dimensions. It becomes a
// Moesif API event only after it has been checked against the taxonomy, and
// its identifiers are mapped to Moesif's keys only in the background, so the
// handler never waits on that mapping.
//
// There is deliberately no field for a name, an email, a handle or free text.
type Occurrence struct {
	// Feature is the taxonomy code, e.g. "apip.manage-apis.rest-api-create".
	// It is sent as metadata.growth_action.
	Feature string
	// OrgUUID is the platform organization UUID. It is mapped to Moesif's
	// company_id when the event is sent.
	OrgUUID string
	// ActorSubject is the acting user's identity-provider subject, as the
	// request authenticated it (in WSO2 Cloud, the Thunder user ID). It is
	// sent as Moesif's user_id only if it is a UUID, so a service account
	// ("sa:..."), an email-shaped or a federated numeric subject never is.
	// Empty for a system occurrence.
	ActorSubject string
	// OccurrenceID identifies this occurrence: the UUID of what a successful
	// call created, or a random ID for a failed one. It is never sent as is:
	// it is hashed together with the feature code into transaction_id, which
	// Moesif deduplicates on.
	OccurrenceID string
	// Invocation is the REST API call the occurrence happened in.
	Invocation Invocation
	// Dimensions are the occurrence's taxonomy dimensions, keyed by dimension
	// name. Values must be string (enum or string dimensions) or bool.
	Dimensions map[string]any
}

// RejectionError says why an occurrence was refused before leaving the
// process. Its reason names the feature and the dimension, never a value.
type RejectionError struct {
	Feature string
	Reason  string
}

func (e *RejectionError) Error() string {
	return fmt.Sprintf("feature usage occurrence for %q rejected: %s", e.Feature, e.Reason)
}

// stringDimensionPattern is what a string-typed dimension may carry: a code,
// such as a policy name, never free text a person typed. Anything else is
// refused rather than trimmed, so a value that is not a code never reaches a
// third party in any form.
var stringDimensionPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

// transactionNamespace scopes the derived transaction IDs to this producer.
var transactionNamespace = uuid.NewSHA1(uuid.NameSpaceURL, []byte("https://wso2.com/api-platform/platform-api/feature-usage"))

// validatedEvent is an occurrence that passed validation: its invocation and
// metadata are final, and only the identity mapping is left to do.
type validatedEvent struct {
	feature       *Feature
	invocation    Invocation
	orgUUID       string
	actorSubject  string
	transactionID string
	metadata      map[string]any
}

// validate checks o against the taxonomy and the producer's allowlist, and
// builds the metadata the event will carry. It does no I/O, so it runs on the
// caller's goroutine and an invalid occurrence is never queued.
func validate(tax *Taxonomy, enabled map[string]bool, o Occurrence, now time.Time) (*validatedEvent, error) {
	reject := func(format string, args ...any) error {
		return &RejectionError{Feature: o.Feature, Reason: fmt.Sprintf(format, args...)}
	}

	if !enabled[o.Feature] {
		return nil, reject("the feature is not enabled for reporting")
	}
	feature, ok := tax.Feature(o.Feature)
	if !ok {
		return nil, reject("the feature is not in the taxonomy")
	}
	if feature.EventName != feature.Code {
		return nil, reject("the taxonomy's event_name differs from the feature code")
	}
	if !feature.Platforms[PlatformAPI] {
		return nil, reject("the taxonomy does not track the feature from the API")
	}
	if o.OrgUUID == "" {
		return nil, reject("no organization")
	}
	if o.OccurrenceID == "" {
		return nil, reject("no occurrence ID")
	}
	invocation, err := o.Invocation.normalize(now)
	if err != nil {
		return nil, reject("%s", err.Error())
	}

	dimensions := withOutcome(feature, o.Dimensions, invocation.Status)
	metadata := make(map[string]any, len(dimensions)+3)
	for name, value := range dimensions {
		dim, declared := feature.Dimensions[name]
		if !declared {
			return nil, reject("dimension %q is not declared by the taxonomy", name)
		}
		switch dim.Type {
		case DimensionBoolean:
			if _, isBool := value.(bool); !isBool {
				return nil, reject("dimension %q must be a boolean", name)
			}
		case DimensionEnum:
			s, isString := value.(string)
			if !isString || !dim.Values[s] {
				return nil, reject("dimension %q has a value outside its enum", name)
			}
		case DimensionString:
			s, isString := value.(string)
			if !isString || !stringDimensionPattern.MatchString(s) {
				return nil, reject("dimension %q is not a code", name)
			}
		default:
			return nil, reject("dimension %q has an unsupported type", name)
		}
		metadata[name] = value
	}
	for name, dim := range feature.Dimensions {
		if _, present := dimensions[name]; dim.Required && !present {
			return nil, reject("required dimension %q is missing", name)
		}
	}

	// Written after the dimensions; the taxonomy guarantees no dimension uses
	// these names, so nothing can shadow them. They are the keys Agent
	// Manager's API events carry: an API event has no action name, so
	// growth_action is what names the feature.
	metadata["growth_action"] = feature.Code
	metadata["platform"] = tax.ProductName
	metadata["source"] = PlatformAPI

	return &validatedEvent{
		feature:       feature,
		invocation:    invocation,
		orgUUID:       o.OrgUUID,
		actorSubject:  o.ActorSubject,
		transactionID: uuid.NewSHA1(transactionNamespace, []byte(feature.Code+"\x00"+o.OccurrenceID)).String(),
		metadata:      metadata,
	}, nil
}

// canonicalUUID reports whether s is a UUID in its canonical 36-character
// form. Moesif keys companies and users by exact string, so a value is passed
// through unchanged (never re-formatted) to stay identical to what WSO2
// Cloud's other publishers send for the same organization or user.
func canonicalUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	_, err := uuid.Parse(s)
	return err == nil
}

// dimOutcome is the dimension the response decides rather than the request.
const dimOutcome = "outcome"

// withOutcome returns dims with the outcome dimension set from the call's
// status when the feature's taxonomy entry declares one: "success" for a 2xx,
// "failure" otherwise. The status is the only source, so outcome can never
// disagree with response.status. dims itself is not modified.
func withOutcome(feature *Feature, dims map[string]any, status int) map[string]any {
	if _, declared := feature.Dimensions[dimOutcome]; !declared {
		return dims
	}
	out := maps.Clone(dims)
	if out == nil {
		out = map[string]any{}
	}
	out[dimOutcome] = "failure"
	if status >= 200 && status <= 299 {
		out[dimOutcome] = "success"
	}
	return out
}
