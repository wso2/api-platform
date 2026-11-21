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

package pdk

import (
	"context"
	"time"
)

// GatewayImportAdmitter is an OPTIONAL interface a Plugin may implement to
// decide which NEW artifacts a data-plane gateway's push may create in the
// control plane (POST /api/internal/v1/artifacts/import-gateway-artifacts).
//
// The platform invokes the admitter once per push, after gateway authentication
// and before any writes, offering only artifacts the push would create.
// Updates to existing artifacts, undeploy pushes, and organization-level
// resources not stored as artifacts (e.g., LLM provider templates) are excluded.
// Thus, an admitter cannot prevent the gateway from retaining synced resources.
//
// Admission is enforced at artifact creation, not when artifacts are offered.
// A create is rejected if it was not admitted in the same push, including
// explicit rejections, artifacts never offered, and updates converted to creates
// by concurrent deletions. Rejections are reported as artifact failures under
// their dpid, including the rejection's Code and Message, within the normal
// HTTP 200 response. Other artifacts continue processing. The gateway records
// the failure and retries the artifact on its next (re)connection.
//
// Admission and creation are not atomic. Concurrent pushes from the same
// organization are admitted independently, so shared-count limits may be
// exceeded. Strict limits require the admitter to manage its own reservations.
//
// With several admitters, they run in plugin order and an artifact refused by
// one is not offered to the next.
type GatewayImportAdmitter interface {
	// AdmitGatewayImports returns refused candidates keyed by DPID. Candidates
	// absent from the map are admitted; nil or empty maps admit all candidates.
	//
	// A non-nil error means admission failed: no offered candidates are created,
	// and each is reported as a transient failure (without an errorCode) for retry.
	// Updates and undeploys are unaffected. Return nil, nil to allow the push when
	// admission cannot be determined. Panics are treated as errors.
	//
	// ctx carries the push request's values but is not cancelled on disconnect.
	// The admitter is responsible for bounding its execution time.
	AdmitGatewayImports(ctx context.Context, batch GatewayImportBatch) (map[string]GatewayImportRejection, error)
}

// GatewayImportObserver is an OPTIONAL interface a Plugin may implement to learn
// which artifacts a data-plane gateway's push created in the control plane.
//
// It is called once per push, after every artifact in it has been processed,
// and only when at least one artifact was created. Updates are not reported, so
// an artifact is reported exactly once, by the push that created it. It runs on
// the request path and cannot change the response, so it should be quick. ctx is
// not cancelled when the gateway disconnects, so a report is not lost to a
// gateway that gave up waiting; the observer bounds its own calls. A panic is
// recovered and logged.
type GatewayImportObserver interface {
	GatewayArtifactsImported(ctx context.Context, imported GatewayImportedBatch)
}

// GatewayImportBatch is one push's worth of artifacts that would be created.
type GatewayImportBatch struct {
	// OrgID is the platform's organization UUID the pushing gateway belongs to,
	// resolved from the gateway's own credentials, never from the push body.
	OrgID string
	// GatewayID is the pushing gateway's UUID.
	GatewayID string
	// Candidates are the artifacts the push would create, in the order the
	// platform will import them (dependency order, not deployment order).
	Candidates []GatewayImportCandidate
}

// GatewayImportCandidate represents an artifact that a push would create.
//
// Duplicate entries with the same handle and kind are offered once, using the
// first entry and the latest DeployedAt value. Its admission decision applies
// to all matching entries.
//
// Handle, DisplayName, Version, and ProjectHandle are taken directly from the
// push body and are untrusted, except that Handle must be non-blank.
type GatewayImportCandidate struct {
	// DPID is the artifact's data-plane UUID, the key its result is reported
	// under and the key a rejection is returned under. The gateway assigns it.
	DPID string
	// Kind is the control-plane kind the artifact would be stored as: one of the
	// Kind* constants (a gateway Agent is offered as KindAgentProxy).
	Kind string
	// Handle is the artifact's handle (metadata.name), unique per organization.
	Handle string
	// DisplayName is the artifact's display name, falling back to the handle.
	DisplayName string
	// Version is the artifact's spec.version, empty when it has none.
	Version string
	// ProjectHandle is the project the push names, empty for organization-level
	// kinds. It is as pushed: the platform has not yet checked that it exists.
	ProjectHandle string
	// DeployedAt is when the gateway deployed the artifact, nil when the push
	// carries no deployment time.
	DeployedAt *time.Time
}

// GatewayImportRejection is an admitter's reason for not creating a candidate.
// Both fields reach the gateway, which logs them, so Message should stand on its
// own: what was refused, and what the user can do about it.
type GatewayImportRejection struct {
	// Code is a short machine-readable reason (for example "quota_exceeded"),
	// returned to the gateway as the artifact's errorCode. Empty becomes
	// "import_rejected".
	Code string
	// Message is returned to the gateway as the artifact's error. Empty becomes a
	// generic message.
	Message string
}

// GatewayImportedBatch reports the artifacts one push created.
type GatewayImportedBatch struct {
	// OrgID and GatewayID are as in GatewayImportBatch.
	OrgID     string
	GatewayID string
	Created   []GatewayImportedArtifact
}

// GatewayImportedArtifact is one artifact a push created.
type GatewayImportedArtifact struct {
	// DPID is the artifact's data-plane UUID.
	DPID string
	// ID is the control-plane UUID the platform minted for it.
	ID string
	// Kind is the control-plane kind it was stored as (a Kind* constant).
	Kind string
	// Handle is the artifact's handle.
	Handle string
}
