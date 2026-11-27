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

package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/utils"
	"github.com/wso2/api-platform/platform-api/pdk"
)

// The defaults a rejection gets for whatever the admitter left empty, so the gateway is
// always told something it can log.
const (
	defaultImportRejectionCode    = "import_rejected"
	defaultImportRejectionMessage = "The control plane did not accept this artifact."
)

// offeredForAdmission reports whether artifacts of a pushed kind are offered to the plugins'
// admitters and reported to their observers. Every kind stored in the artifacts table is: that
// table is where an existing artifact is found by handle, which is what tells a create from an
// update. An LLM provider template is an organization-level record outside it, so it is
// neither offered nor reported.
func offeredForAdmission(gatewayKind string) bool {
	return gatewayKind != constants.LLMProviderTemplate
}

// admissionKey identifies a possible create by handle and control-plane kind. A handle can
// be new for multiple kinds, but only one can be created.
type admissionKey struct {
	handle string
	kind   string
}

// importGate holds one push's admission decisions and is enforced where the import creates an
// artifact, ensuring no create escapes the admitters.
//
// A nil gate admits every create: no admitter is installed.
type importGate struct {
	admitted map[admissionKey]bool
	refused  map[admissionKey]pdk.GatewayImportRejection
	// undecided names the creates an admitter could not decide (it failed or panicked).
	undecided map[admissionKey]error
}

// importRefusedError is an admitter's refusal, reported to the gateway with its code.
type importRefusedError struct {
	rejection pdk.GatewayImportRejection
}

func (e *importRefusedError) Error() string { return e.rejection.Message }

// importNotAdmittedError is a transient create error when no admitter decided this push.
type importNotAdmittedError struct {
	cause error
}

func (e *importNotAdmittedError) Error() string {
	const msg = "The artifact was not admitted in this push and was not created; it is considered again when it is next pushed."
	if e.cause != nil {
		return msg + " The control plane could not decide: " + e.cause.Error()
	}
	return msg
}

func (e *importNotAdmittedError) Unwrap() error { return e.cause }

// allowCreate reports whether the push may create the artifact named by handle and kind.
func (g *importGate) allowCreate(handle, storedKind string) error {
	if g == nil {
		return nil
	}
	key := admissionKey{handle: handle, kind: storedKind}
	if rejection, ok := g.refused[key]; ok {
		return &importRefusedError{rejection: rejection}
	}
	if cause, ok := g.undecided[key]; ok {
		return &importNotAdmittedError{cause: cause}
	}
	if g.admitted[key] {
		return nil
	}
	return &importNotAdmittedError{}
}

// admitImports asks plugin admitters which artifacts may be created. It returns nil when no
// admitters are configured. The gateway must already be validated for the org.
//
// Admitters receive a non-cancellable context so processing continues after the gateway
// disconnects; decisions must not fail, and fail-open admitters must allow everything through.
func (s *ArtifactImportService) admitImports(ctx context.Context, orgID, gatewayID string, ordered []dto.ImportGatewayArtifactRequest) *importGate {
	if len(s.importAdmitters) == 0 {
		return nil
	}
	gate := &importGate{
		admitted:  make(map[admissionKey]bool),
		refused:   make(map[admissionKey]pdk.GatewayImportRejection),
		undecided: make(map[admissionKey]error),
	}
	candidates := s.importCandidates(orgID, ordered)
	if len(candidates) == 0 {
		return gate
	}
	// DPIDs should be unique, but a malformed push can repeat one across candidates.
	keysByDPID := make(map[string][]admissionKey, len(candidates))
	for _, c := range candidates {
		keysByDPID[c.DPID] = append(keysByDPID[c.DPID], candidateKey(c))
	}

	hookCtx := context.WithoutCancel(ctx)
	for i, admitter := range s.importAdmitters {
		// An artifact one admitter refused is not offered to the next.
		remaining := make([]pdk.GatewayImportCandidate, 0, len(candidates))
		for _, c := range candidates {
			if _, refused := gate.refused[candidateKey(c)]; !refused {
				remaining = append(remaining, c)
			}
		}
		if len(remaining) == 0 {
			break
		}
		verdicts, err := s.callAdmitter(hookCtx, admitter, pdk.GatewayImportBatch{
			OrgID:      orgID,
			GatewayID:  gatewayID,
			Candidates: remaining,
		})
		if err != nil {
			// Nothing this admitter was offered can be decided, and the admitters after it
			// cannot overrule it: none of these creates happens in this push.
			s.slogger.Warn("Import admitter could not decide; the artifacts it was offered are not created in this push",
				"admitter", i, "gatewayId", gatewayID, "orgId", orgID, "candidates", len(remaining), "error", err)
			for _, c := range remaining {
				gate.undecided[candidateKey(c)] = err
			}
			return gate
		}
		for dpid, rejection := range verdicts {
			keys, offered := keysByDPID[dpid]
			if !offered {
				// Only a candidate can be refused: anything else is an update or not in the
				// push at all, and an admitter must not be able to block either.
				s.slogger.Warn("Import admitter refused an artifact that was not offered to it; ignoring",
					"dpid", dpid, "gatewayId", gatewayID, "orgId", orgID)
				continue
			}
			if len(keys) > 1 {
				// The refusal cannot say which one it meant, so it holds for all of them.
				s.slogger.Warn("Import admitter refused a DPID the push repeats; refusing every artifact under it",
					"dpid", dpid, "artifacts", len(keys), "gatewayId", gatewayID, "orgId", orgID)
			}
			for _, key := range keys {
				if _, refused := gate.refused[key]; !refused {
					gate.refused[key] = normalizeImportRejection(rejection)
				}
			}
		}
	}
	for _, c := range candidates {
		key := candidateKey(c)
		if _, refused := gate.refused[key]; !refused {
			gate.admitted[key] = true
		}
	}
	return gate
}

// candidateKey is the create a candidate stands for.
func candidateKey(c pdk.GatewayImportCandidate) admissionKey {
	return admissionKey{handle: c.Handle, kind: c.Kind}
}

// callAdmitter runs one admitter and limits panic failures to its requested creates.
func (s *ArtifactImportService) callAdmitter(ctx context.Context, admitter pdk.GatewayImportAdmitter, batch pdk.GatewayImportBatch) (verdicts map[string]pdk.GatewayImportRejection, err error) {
	defer func() {
		if r := recover(); r != nil {
			verdicts, err = nil, fmt.Errorf("admitter panicked: %v", r)
		}
	}()
	return admitter.AdmitGatewayImports(ctx, batch)
}

// importCandidates picks out the entries of a push that would create an artifact: a deployed
// (not undeployed) push of an offered kind with a registered importer and a non-blank handle,
// whose handle the organization does not hold yet. A handle and kind pushed more than once
// yield one candidate — the first entry, carrying the latest DeployedAt among them.
//
// An entry whose handle could not be looked up is not offered. That does not let it through:
// the gate refuses any create it was not asked about, so it is created only if the import's
// own lookup finds it to be an update after all.
func (s *ArtifactImportService) importCandidates(orgID string, ordered []dto.ImportGatewayArtifactRequest) []pdk.GatewayImportCandidate {
	var candidates []pdk.GatewayImportCandidate
	candidateIndex := make(map[admissionKey]int) // key -> index in candidates
	known := make(map[string]bool)               // handle -> already held by the organization
	lookupFailed := make(map[string]bool)        // handle -> could not be looked up

	for _, req := range ordered {
		kind := req.Configuration.Kind
		if req.DPID == "" || req.Status == utils.ImportStatusUndeployed || !offeredForAdmission(kind) {
			continue
		}
		if _, ok := s.importers[kind]; !ok {
			continue
		}
		handle := utils.ImportHandle(req.Configuration)
		if strings.TrimSpace(handle) == "" || lookupFailed[handle] {
			continue
		}
		key := admissionKey{handle: handle, kind: storedArtifactKind(kind)}

		if i, seen := candidateIndex[key]; seen {
			if req.DeployedAt != nil && (candidates[i].DeployedAt == nil || req.DeployedAt.After(*candidates[i].DeployedAt)) {
				deployedAt := *req.DeployedAt
				candidates[i].DeployedAt = &deployedAt
			}
			continue
		}

		exists, looked := known[handle]
		if !looked {
			existing, err := s.artifactRepo.GetByHandle(handle, orgID)
			if err != nil {
				s.slogger.Warn("Could not tell whether a pushed artifact is new; it is not offered for admission",
					"dpid", req.DPID, "kind", kind, "handle", handle, "orgId", orgID, "error", err)
				lookupFailed[handle] = true
				continue
			}
			exists = existing != nil
			known[handle] = exists
		}
		if exists {
			// An update, or a handle held by another kind, which the import refuses itself.
			continue
		}

		var deployedAt *time.Time
		if req.DeployedAt != nil {
			t := *req.DeployedAt
			deployedAt = &t
		}
		candidateIndex[key] = len(candidates)
		candidates = append(candidates, pdk.GatewayImportCandidate{
			DPID:          req.DPID,
			Kind:          key.kind,
			Handle:        handle,
			DisplayName:   utils.ImportDisplayName(req.Configuration),
			Version:       utils.SpecString(req.Configuration.Spec, "version"),
			ProjectHandle: utils.ResolveImportProject(req.Configuration.Metadata),
			DeployedAt:    deployedAt,
		})
	}
	return candidates
}

// normalizeImportRejection fills in what an admitter left empty.
func normalizeImportRejection(r pdk.GatewayImportRejection) pdk.GatewayImportRejection {
	r.Code = strings.TrimSpace(r.Code)
	r.Message = strings.TrimSpace(r.Message)
	if r.Code == "" {
		r.Code = defaultImportRejectionCode
	}
	if r.Message == "" {
		r.Message = defaultImportRejectionMessage
	}
	return r
}

// notifyImported tells the plugins' observers which artifacts a push created. It is a no-op
// when nothing was created or no plugin observes.
//
// The observers get ctx without its cancellation: the creates have already happened, and the
// next push only updates them, so a report lost because the gateway stopped waiting would be
// lost for good. A panicking observer is logged and the rest are still told.
func (s *ArtifactImportService) notifyImported(ctx context.Context, orgID, gatewayID string, created []pdk.GatewayImportedArtifact) {
	if len(created) == 0 || len(s.importObservers) == 0 {
		return
	}
	hookCtx := context.WithoutCancel(ctx)
	for i, observer := range s.importObservers {
		// Each gets its own copy, so one observer cannot change what the next is told.
		batch := pdk.GatewayImportedBatch{OrgID: orgID, GatewayID: gatewayID,
			Created: append([]pdk.GatewayImportedArtifact(nil), created...)}
		s.callObserver(hookCtx, i, observer, batch)
	}
}

// callObserver runs one observer, recovering a panic: the push's creates are already done and
// its response must still reach the gateway.
func (s *ArtifactImportService) callObserver(ctx context.Context, i int, observer pdk.GatewayImportObserver, batch pdk.GatewayImportedBatch) {
	defer func() {
		if r := recover(); r != nil {
			s.slogger.Error("Import observer panicked; the artifacts it was told about may not be recorded by it",
				"observer", i, "gatewayId", batch.GatewayID, "orgId", batch.OrgID, "created", len(batch.Created), "panic", r)
		}
	}()
	observer.GatewayArtifactsImported(ctx, batch)
}
