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
// The platform calls it once per push, after the gateway has authenticated and
// before anything is written, with only the artifacts the push would create.
// Updates to artifacts the control plane already holds, undeploy pushes, and
// organization-level kinds that are not stored as artifacts (LLM provider
// templates) are never offered, so an admitter cannot block a gateway from
// keeping what it has already synced.
//
// An artifact the admitter rejects is not created. It is reported to the
// gateway as a failure under its dpid, carrying the rejection's Code and
// Message, inside the push's normal 200 response; the rest of the push goes
// ahead. The gateway records the failure and pushes the artifact again on its
// next (re)connect, when it is offered again.
//
// With several admitters, they run in plugin order and an artifact rejected by
// one is not offered to the next.
type GatewayImportAdmitter interface {
	// AdmitGatewayImports returns the rejected candidates, keyed by their DPID.
	// A candidate absent from the map is admitted, so a nil or empty map admits
	// the whole batch. ctx is the push request's context.
	AdmitGatewayImports(ctx context.Context, batch GatewayImportBatch) map[string]GatewayImportRejection
}

// GatewayImportObserver is an OPTIONAL interface a Plugin may implement to learn
// which artifacts a data-plane gateway's push created in the control plane.
//
// It is called once per push, after every artifact in it has been processed,
// and only when at least one artifact was created. Updates are not reported.
// It runs on the request path and cannot change the response, so it must not
// block: hand slow work (publishing an event, say) off to the background.
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

// GatewayImportCandidate is one artifact a push would create.
//
// A handle pushed more than once in the same batch is offered once, as its first
// entry, carrying the latest DeployedAt among its entries; the verdict on it
// applies to every entry with that handle.
type GatewayImportCandidate struct {
	// DPID is the artifact's data-plane UUID, the key its result is reported
	// under and the key a rejection is returned under.
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
