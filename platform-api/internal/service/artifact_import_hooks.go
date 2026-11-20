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

// admitImports asks the plugins' admitters which of the artifacts in a push may be created,
// and returns the refused ones keyed by dpid. It is nil when there are no admitters or nothing
// in the push would be created. The gateway must already be validated for the org.
//
// A refusal on a handle applies to every entry in the push with that handle, since letting a
// second entry through would create the artifact the first was refused.
func (s *ArtifactImportService) admitImports(ctx context.Context, orgID, gatewayID string, ordered []dto.ImportGatewayArtifactRequest) map[string]pdk.GatewayImportRejection {
	if len(s.importAdmitters) == 0 {
		return nil
	}
	candidates, dpidsByHandle := s.importCandidates(orgID, ordered)
	if len(candidates) == 0 {
		return nil
	}
	handleByDPID := make(map[string]string, len(candidates))
	for _, c := range candidates {
		handleByDPID[c.DPID] = c.Handle
	}

	refusedHandles := make(map[string]pdk.GatewayImportRejection)
	for _, admitter := range s.importAdmitters {
		// An artifact one admitter refused is not offered to the next.
		remaining := make([]pdk.GatewayImportCandidate, 0, len(candidates))
		for _, c := range candidates {
			if _, refused := refusedHandles[c.Handle]; !refused {
				remaining = append(remaining, c)
			}
		}
		if len(remaining) == 0 {
			break
		}
		verdicts := admitter.AdmitGatewayImports(ctx, pdk.GatewayImportBatch{
			OrgID:      orgID,
			GatewayID:  gatewayID,
			Candidates: remaining,
		})
		for dpid, rejection := range verdicts {
			handle, offered := handleByDPID[dpid]
			if !offered {
				// Only a candidate can be refused: anything else is an update or not in the
				// push at all, and an admitter must not be able to block either.
				s.slogger.Warn("Import admitter refused an artifact that was not offered to it; ignoring",
					"dpid", dpid, "gatewayId", gatewayID, "orgId", orgID)
				continue
			}
			if _, refused := refusedHandles[handle]; refused {
				continue
			}
			refusedHandles[handle] = normalizeImportRejection(rejection)
		}
	}
	if len(refusedHandles) == 0 {
		return nil
	}

	rejected := make(map[string]pdk.GatewayImportRejection)
	for handle, rejection := range refusedHandles {
		for _, dpid := range dpidsByHandle[handle] {
			rejected[dpid] = rejection
		}
	}
	return rejected
}

// importCandidates picks out the entries of a push that would create an artifact: a deployed
// (not undeployed) push of an offered kind with a registered importer, whose handle the
// organization does not hold yet. A handle pushed more than once yields one candidate — its
// first entry, carrying the latest DeployedAt among them — and dpidsByHandle lists every
// entry with that handle so a verdict can be applied to all of them.
//
// A handle whose lookup fails is left out: the import itself is very likely to fail on the
// same lookup, and offering it could get an update to an existing artifact refused.
func (s *ArtifactImportService) importCandidates(orgID string, ordered []dto.ImportGatewayArtifactRequest) (candidates []pdk.GatewayImportCandidate, dpidsByHandle map[string][]string) {
	dpidsByHandle = make(map[string][]string)
	candidateIndex := make(map[string]int) // handle -> index in candidates
	known := make(map[string]bool)         // handle -> already held by the organization

	for _, req := range ordered {
		kind := req.Configuration.Kind
		if req.DPID == "" || req.Status == utils.ImportStatusUndeployed || !offeredForAdmission(kind) {
			continue
		}
		if _, ok := s.importers[kind]; !ok {
			continue
		}
		handle := utils.ImportHandle(req.Configuration)
		if strings.TrimSpace(handle) == "" {
			continue
		}

		if i, seen := candidateIndex[handle]; seen {
			dpidsByHandle[handle] = append(dpidsByHandle[handle], req.DPID)
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
				s.slogger.Warn("Could not tell whether a pushed artifact is new; not offering it for admission",
					"dpid", req.DPID, "kind", kind, "handle", handle, "orgId", orgID, "error", err)
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
		candidateIndex[handle] = len(candidates)
		dpidsByHandle[handle] = append(dpidsByHandle[handle], req.DPID)
		candidates = append(candidates, pdk.GatewayImportCandidate{
			DPID:          req.DPID,
			Kind:          storedArtifactKind(kind),
			Handle:        handle,
			DisplayName:   utils.ImportDisplayName(req.Configuration),
			Version:       utils.SpecString(req.Configuration.Spec, "version"),
			ProjectHandle: utils.ResolveImportProject(req.Configuration.Metadata),
			DeployedAt:    deployedAt,
		})
	}
	return candidates, dpidsByHandle
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
func (s *ArtifactImportService) notifyImported(ctx context.Context, orgID, gatewayID string, created []pdk.GatewayImportedArtifact) {
	if len(created) == 0 || len(s.importObservers) == 0 {
		return
	}
	batch := pdk.GatewayImportedBatch{OrgID: orgID, GatewayID: gatewayID, Created: created}
	for _, observer := range s.importObservers {
		observer.GatewayArtifactsImported(ctx, batch)
	}
}
