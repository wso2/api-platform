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
	"errors"
	"sort"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
	"github.com/wso2/api-platform/platform-api/pdk"
)

// recordingAdmitter records every batch it is offered and refuses what refuse returns. err
// makes it fail to decide, panicValue makes it panic, and ctxErrs records the context error it
// saw on each call.
type recordingAdmitter struct {
	batches    []pdk.GatewayImportBatch
	refuse     func(batch pdk.GatewayImportBatch) map[string]pdk.GatewayImportRejection
	err        error
	panicValue any
	ctxErrs    []error
}

func (a *recordingAdmitter) AdmitGatewayImports(ctx context.Context, batch pdk.GatewayImportBatch) (map[string]pdk.GatewayImportRejection, error) {
	a.batches = append(a.batches, batch)
	a.ctxErrs = append(a.ctxErrs, ctx.Err())
	if a.panicValue != nil {
		panic(a.panicValue)
	}
	if a.err != nil {
		return nil, a.err
	}
	if a.refuse == nil {
		return nil, nil
	}
	return a.refuse(batch), nil
}

// offeredDPIDs is every candidate dpid across the batches the admitter saw, sorted.
func (a *recordingAdmitter) offeredDPIDs() []string {
	var ids []string
	for _, b := range a.batches {
		for _, c := range b.Candidates {
			ids = append(ids, c.DPID)
		}
	}
	sort.Strings(ids)
	return ids
}

// refuseDPIDs refuses exactly the given dpids with a quota rejection.
func refuseDPIDs(dpids ...string) func(pdk.GatewayImportBatch) map[string]pdk.GatewayImportRejection {
	return func(pdk.GatewayImportBatch) map[string]pdk.GatewayImportRejection {
		out := make(map[string]pdk.GatewayImportRejection, len(dpids))
		for _, id := range dpids {
			out[id] = pdk.GatewayImportRejection{Code: "quota_exceeded", Message: "API limit reached (apis 3/3)."}
		}
		return out
	}
}

// recordingObserver records every batch of created artifacts it is told about, and the context
// error it saw. panicValue makes it panic after recording.
type recordingObserver struct {
	batches    []pdk.GatewayImportedBatch
	ctxErrs    []error
	panicValue any
}

func (o *recordingObserver) GatewayArtifactsImported(ctx context.Context, b pdk.GatewayImportedBatch) {
	o.batches = append(o.batches, b)
	o.ctxErrs = append(o.ctxErrs, ctx.Err())
	if o.panicValue != nil {
		panic(o.panicValue)
	}
}

func (o *recordingObserver) createdDPIDs() []string {
	var ids []string
	for _, b := range o.batches {
		for _, c := range b.Created {
			ids = append(ids, c.DPID)
		}
	}
	sort.Strings(ids)
	return ids
}

func importBatch(d *importTestDeps, reqs ...dto.ImportGatewayArtifactRequest) *dto.ImportGatewayArtifactsResponse {
	return d.svc.ImportArtifacts(context.Background(), importTestOrgID, importTestGatewayID, reqs)
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Only artifacts the push would create are offered: an update to an artifact the control
// plane already holds, an undeploy push, and an LLM provider template never are, so an
// admitter cannot keep a gateway from updating what it has already synced.
func TestImportHooks_OnlyCreatesAreOffered(t *testing.T) {
	d := setupImportTest(t)
	mustImport(t, d, restImportRequest("dp-existing", "existing-api", "Existing API"))
	mustImport(t, d, agentImportRequest("dp-existing-agent", "existing-agent", minimalGatewayAgentSpec()))

	admitter := &recordingAdmitter{}
	d.svc.SetGatewayImportHooks([]pdk.GatewayImportAdmitter{admitter}, nil)

	resp := importBatch(d,
		restImportRequest("dp-existing-again", "existing-api", "Existing API v2"),
		agentImportRequest("dp-existing-agent-again", "existing-agent", minimalGatewayAgentSpec()),
		undeployed(restImportRequest("dp-gone", "gone-api", "Gone API")),
		dpTemplateReq("dp-template", "my-template", "My Template"),
		restImportRequest("dp-new-rest", "new-rest-api", "New REST API"),
		graphqlImportRequest("dp-new-gql", "new-gql-api", "New GraphQL API"),
	)

	if len(admitter.batches) != 1 {
		t.Fatalf("admitter called %d times, want once per push", len(admitter.batches))
	}
	if got, want := admitter.offeredDPIDs(), []string{"dp-new-gql", "dp-new-rest"}; !equalStrings(got, want) {
		t.Fatalf("offered = %v, want %v", got, want)
	}
	b := admitter.batches[0]
	if b.OrgID != importTestOrgID || b.GatewayID != importTestGatewayID {
		t.Errorf("batch org/gateway = %q/%q, want %q/%q", b.OrgID, b.GatewayID, importTestOrgID, importTestGatewayID)
	}
	if resp.Failed != 0 {
		t.Errorf("failed = %d, want 0 when nothing is refused (%+v)", resp.Failed, resp.Artifacts)
	}
}

func TestImportHooks_CandidateCarriesTheArtifactsIdentity(t *testing.T) {
	d := setupImportTest(t)
	admitter := &recordingAdmitter{}
	d.svc.SetGatewayImportHooks([]pdk.GatewayImportAdmitter{admitter}, nil)

	importBatch(d,
		withDeployedAt(restImportRequest("dp-rest", "weather-api", "Weather API"), newerDeployedAt),
		agentImportRequest("dp-agent", "my-agent", minimalGatewayAgentSpec()),
	)

	byDPID := map[string]pdk.GatewayImportCandidate{}
	for _, c := range admitter.batches[0].Candidates {
		byDPID[c.DPID] = c
	}
	rest := byDPID["dp-rest"]
	if rest.Kind != pdk.KindRestAPI || rest.Handle != "weather-api" || rest.DisplayName != "Weather API" ||
		rest.Version != "v1.0" || rest.ProjectHandle != "default" {
		t.Errorf("REST candidate = %+v", rest)
	}
	if rest.DeployedAt == nil || !rest.DeployedAt.Equal(newerDeployedAt) {
		t.Errorf("REST candidate DeployedAt = %v, want %v", rest.DeployedAt, newerDeployedAt)
	}
	// A gateway Agent is offered under the control-plane kind it is stored as.
	if agent := byDPID["dp-agent"]; agent.Kind != pdk.KindAgentProxy {
		t.Errorf("Agent candidate kind = %q, want %q", agent.Kind, pdk.KindAgentProxy)
	}
}

func TestImportHooks_RefusedArtifactFailsAndIsNotCreated(t *testing.T) {
	d := setupImportTest(t)
	admitter := &recordingAdmitter{refuse: refuseDPIDs("dp-refused")}
	observer := &recordingObserver{}
	d.svc.SetGatewayImportHooks([]pdk.GatewayImportAdmitter{admitter}, []pdk.GatewayImportObserver{observer})

	resp := importBatch(d,
		restImportRequest("dp-admitted", "admitted-api", "Admitted API"),
		restImportRequest("dp-refused", "refused-api", "Refused API"),
	)

	if resp.Total != 2 || resp.Success != 1 || resp.Failed != 1 {
		t.Fatalf("total/success/failed = %d/%d/%d, want 2/1/1", resp.Total, resp.Success, resp.Failed)
	}
	refused := resp.Artifacts["dp-refused"]
	if refused.Status != utils.ImportStatusFailed || refused.Origin != constants.OriginDP || refused.ID != "" {
		t.Errorf("refused result = %+v, want a failed gateway_api result with no id", refused)
	}
	if refused.ErrorCode != "quota_exceeded" || refused.Error != "API limit reached (apis 3/3)." {
		t.Errorf("refused error/code = %q/%q, want the admitter's", refused.Error, refused.ErrorCode)
	}
	if art := artifactByHandle(t, d, "refused-api"); art != nil {
		t.Errorf("refused artifact was created: %+v", art)
	}
	if art := artifactByHandle(t, d, "admitted-api"); art == nil {
		t.Error("admitted artifact was not created")
	}
	if admitted := resp.Artifacts["dp-admitted"]; admitted.Error != "" || admitted.ErrorCode != "" {
		t.Errorf("admitted result carries an error: %+v", admitted)
	}
	if got := observer.createdDPIDs(); !equalStrings(got, []string{"dp-admitted"}) {
		t.Errorf("observer told of %v, want only the admitted artifact", got)
	}
}

// A refused artifact is offered again on the next push, and created once admitted.
func TestImportHooks_RefusedArtifactIsOfferedAgainOnTheNextPush(t *testing.T) {
	d := setupImportTest(t)
	admitter := &recordingAdmitter{refuse: refuseDPIDs("dp-1")}
	d.svc.SetGatewayImportHooks([]pdk.GatewayImportAdmitter{admitter}, nil)

	if resp := importBatch(d, restImportRequest("dp-1", "later-api", "Later API")); resp.Failed != 1 {
		t.Fatalf("first push: failed = %d, want 1", resp.Failed)
	}
	admitter.refuse = nil
	if resp := importBatch(d, restImportRequest("dp-1", "later-api", "Later API")); resp.Success != 1 {
		t.Fatalf("second push: success = %d, want 1 (%+v)", resp.Success, resp.Artifacts)
	}
	if len(admitter.batches) != 2 {
		t.Errorf("admitter called %d times, want 2", len(admitter.batches))
	}
	if art := artifactByHandle(t, d, "later-api"); art == nil {
		t.Error("artifact was not created once admitted")
	}
}

func TestImportHooks_EmptyRejectionGetsDefaults(t *testing.T) {
	d := setupImportTest(t)
	admitter := &recordingAdmitter{refuse: func(pdk.GatewayImportBatch) map[string]pdk.GatewayImportRejection {
		return map[string]pdk.GatewayImportRejection{"dp-1": {}}
	}}
	d.svc.SetGatewayImportHooks([]pdk.GatewayImportAdmitter{admitter}, nil)

	got := importBatch(d, restImportRequest("dp-1", "an-api", "An API")).Artifacts["dp-1"]
	if got.ErrorCode != defaultImportRejectionCode || got.Error != defaultImportRejectionMessage {
		t.Errorf("error/code = %q/%q, want the defaults", got.Error, got.ErrorCode)
	}
}

// An admitter can only refuse what it was offered: refusing an update, or a dpid not in the
// push, changes nothing.
func TestImportHooks_RefusingSomethingNotOfferedIsIgnored(t *testing.T) {
	d := setupImportTest(t)
	mustImport(t, d, restImportRequest("dp-existing", "existing-api", "Existing API"))
	admitter := &recordingAdmitter{refuse: refuseDPIDs("dp-update", "dp-not-in-push")}
	d.svc.SetGatewayImportHooks([]pdk.GatewayImportAdmitter{admitter}, nil)

	resp := importBatch(d,
		restImportRequest("dp-update", "existing-api", "Existing API v2"),
		restImportRequest("dp-new", "new-api", "New API"),
	)
	if resp.Success != 2 || resp.Failed != 0 {
		t.Fatalf("success/failed = %d/%d, want 2/0 (%+v)", resp.Success, resp.Failed, resp.Artifacts)
	}
}

// A handle pushed twice in one push is offered once, with the latest deployment time, and a
// refusal applies to both entries: letting the second through would create what was refused.
func TestImportHooks_DuplicateHandleIsOfferedOnceAndRefusedTogether(t *testing.T) {
	d := setupImportTest(t)
	admitter := &recordingAdmitter{refuse: refuseDPIDs("dp-first")}
	d.svc.SetGatewayImportHooks([]pdk.GatewayImportAdmitter{admitter}, nil)

	resp := importBatch(d,
		withDeployedAt(restImportRequest("dp-first", "twice-api", "Twice API"), baseDeployedAt),
		withDeployedAt(restImportRequest("dp-second", "twice-api", "Twice API"), evenNewerDeployedAt),
	)

	cands := admitter.batches[0].Candidates
	if len(cands) != 1 || cands[0].DPID != "dp-first" {
		t.Fatalf("candidates = %+v, want only dp-first", cands)
	}
	if cands[0].DeployedAt == nil || !cands[0].DeployedAt.Equal(evenNewerDeployedAt) {
		t.Errorf("candidate DeployedAt = %v, want the latest (%v)", cands[0].DeployedAt, evenNewerDeployedAt)
	}
	if resp.Failed != 2 {
		t.Fatalf("failed = %d, want both entries refused (%+v)", resp.Failed, resp.Artifacts)
	}
	if art := artifactByHandle(t, d, "twice-api"); art != nil {
		t.Errorf("refused handle was created: %+v", art)
	}
}

// Admitters run in plugin order; what one refuses is not offered to the next.
func TestImportHooks_LaterAdmitterIsNotOfferedWhatAnEarlierOneRefused(t *testing.T) {
	d := setupImportTest(t)
	first := &recordingAdmitter{refuse: refuseDPIDs("dp-a")}
	second := &recordingAdmitter{refuse: refuseDPIDs("dp-b")}
	d.svc.SetGatewayImportHooks([]pdk.GatewayImportAdmitter{first, second}, nil)

	resp := importBatch(d,
		restImportRequest("dp-a", "a-api", "A API"),
		restImportRequest("dp-b", "b-api", "B API"),
		restImportRequest("dp-c", "c-api", "C API"),
	)

	if got := second.offeredDPIDs(); !equalStrings(got, []string{"dp-b", "dp-c"}) {
		t.Errorf("second admitter offered %v, want [dp-b dp-c]", got)
	}
	if resp.Success != 1 || resp.Failed != 2 || resp.Artifacts["dp-c"].Error != "" {
		t.Errorf("success/failed = %d/%d, want 1/2 with dp-c imported (%+v)", resp.Success, resp.Failed, resp.Artifacts)
	}
}

// With nothing the push would create, the admitter is not called at all.
func TestImportHooks_AdmitterIsNotCalledWithoutCandidates(t *testing.T) {
	d := setupImportTest(t)
	mustImport(t, d, restImportRequest("dp-existing", "existing-api", "Existing API"))
	admitter := &recordingAdmitter{}
	d.svc.SetGatewayImportHooks([]pdk.GatewayImportAdmitter{admitter}, nil)

	importBatch(d, restImportRequest("dp-existing-again", "existing-api", "Existing API v2"))
	if len(admitter.batches) != 0 {
		t.Errorf("admitter called %d times, want 0", len(admitter.batches))
	}
}

// The observer hears about creates only — with the control-plane id and stored kind — and is
// not called for a push that created nothing.
func TestImportHooks_ObserverIsToldOnlyOfCreates(t *testing.T) {
	d := setupImportTest(t)
	mustImport(t, d, restImportRequest("dp-existing", "existing-api", "Existing API"))
	observer := &recordingObserver{}
	d.svc.SetGatewayImportHooks(nil, []pdk.GatewayImportObserver{observer})

	resp := importBatch(d,
		restImportRequest("dp-existing-again", "existing-api", "Existing API v2"),
		dpTemplateReq("dp-template", "my-template", "My Template"),
		graphqlImportRequest("dp-gql", "new-gql-api", "New GraphQL API"),
	)

	if len(observer.batches) != 1 {
		t.Fatalf("observer called %d times, want once", len(observer.batches))
	}
	created := observer.batches[0].Created
	if len(created) != 1 {
		t.Fatalf("created = %+v, want only the GraphQL API", created)
	}
	c := created[0]
	if c.DPID != "dp-gql" || c.Handle != "new-gql-api" || c.Kind != pdk.KindGraphQLAPI || c.ID != resp.Artifacts["dp-gql"].ID {
		t.Errorf("created = %+v, want dp-gql with CP id %q", c, resp.Artifacts["dp-gql"].ID)
	}

	importBatch(d, restImportRequest("dp-existing-3", "existing-api", "Existing API v3"))
	if len(observer.batches) != 1 {
		t.Errorf("observer called for a push that created nothing")
	}
}

// flakyLookupRepo fails the next GetByHandle calls for one handle, then behaves normally.
type flakyLookupRepo struct {
	repository.ArtifactRepository
	handle   string
	failures int
}

func (r *flakyLookupRepo) GetByHandle(handle, orgUUID string) (*model.Artifact, error) {
	if handle == r.handle && r.failures > 0 {
		r.failures--
		return nil, errors.New("connection reset")
	}
	return r.ArtifactRepository.GetByHandle(handle, orgUUID)
}

// When the lookup that decides whether an artifact is new fails, the artifact is failed for
// this push — never imported — even though the import's own lookup would have succeeded:
// otherwise a create could slip past the admitters. Every entry with that handle goes with it,
// the failure is transient (no errorCode), and the next push decides it normally.
func TestImportHooks_UndecidableArtifactIsFailedNotImported(t *testing.T) {
	d := setupImportTest(t)
	// Only the admission lookup fails; the import's own lookup would succeed.
	d.svc.artifactRepo = &flakyLookupRepo{ArtifactRepository: d.artifactRepo, handle: "flaky-api", failures: 1}
	admitter := &recordingAdmitter{}
	observer := &recordingObserver{}
	d.svc.SetGatewayImportHooks([]pdk.GatewayImportAdmitter{admitter}, []pdk.GatewayImportObserver{observer})

	resp := importBatch(d,
		restImportRequest("dp-flaky", "flaky-api", "Flaky API"),
		restImportRequest("dp-flaky-again", "flaky-api", "Flaky API"),
		restImportRequest("dp-fine", "fine-api", "Fine API"),
	)

	if resp.Success != 1 || resp.Failed != 2 {
		t.Fatalf("success/failed = %d/%d, want 1/2 (%+v)", resp.Success, resp.Failed, resp.Artifacts)
	}
	for _, dpid := range []string{"dp-flaky", "dp-flaky-again"} {
		got := resp.Artifacts[dpid]
		if got.Status != utils.ImportStatusFailed || got.Error == "" || got.ErrorCode != "" {
			t.Errorf("%s = %+v, want a transient failure: an error and no errorCode", dpid, got)
		}
	}
	if art := artifactByHandle(t, d, "flaky-api"); art != nil {
		t.Fatalf("an artifact no admitter saw was created: %+v", art)
	}
	if got := admitter.offeredDPIDs(); !equalStrings(got, []string{"dp-fine"}) {
		t.Errorf("offered = %v, want only dp-fine", got)
	}
	if got := observer.createdDPIDs(); !equalStrings(got, []string{"dp-fine"}) {
		t.Errorf("observer told of %v, want only dp-fine", got)
	}

	// The next push decides it, and creates it once admitted.
	resp = importBatch(d, restImportRequest("dp-flaky", "flaky-api", "Flaky API"))
	if resp.Success != 1 {
		t.Fatalf("next push: success = %d, want 1 (%+v)", resp.Success, resp.Artifacts)
	}
	if got := admitter.offeredDPIDs(); !equalStrings(got, []string{"dp-fine", "dp-flaky"}) {
		t.Errorf("offered across both pushes = %v, want dp-fine then dp-flaky", got)
	}
}

// refuseKind refuses every candidate of one kind, as a limit on that kind with no room would.
func refuseKind(kind string) func(pdk.GatewayImportBatch) map[string]pdk.GatewayImportRejection {
	return func(b pdk.GatewayImportBatch) map[string]pdk.GatewayImportRejection {
		out := map[string]pdk.GatewayImportRejection{}
		for _, c := range b.Candidates {
			if c.Kind == kind {
				out[c.DPID] = pdk.GatewayImportRejection{Code: "quota_exceeded", Message: "no room"}
			}
		}
		return out
	}
}

// H1: one handle pushed under two kinds is offered once per kind, so an admitter limiting one
// kind sees the entry of that kind, and a decoy of another kind that fails its own import
// cannot carry it through.
func TestImportHooks_SameHandleDifferentKindIsOfferedPerKind(t *testing.T) {
	d := setupImportTest(t)
	admitter := &recordingAdmitter{refuse: refuseKind(pdk.KindRestAPI)}
	d.svc.SetGatewayImportHooks([]pdk.GatewayImportAdmitter{admitter}, nil)

	resp := importBatch(d,
		// A decoy that fails on its own: its template does not exist.
		dpProviderReq("dp-decoy", "shared-handle", "Decoy", "no-such-template"),
		restImportRequest("dp-rest", "shared-handle", "Real API"),
	)

	cands := admitter.batches[0].Candidates
	kinds := map[string]string{}
	for _, c := range cands {
		kinds[c.DPID] = c.Kind
	}
	if kinds["dp-decoy"] != pdk.KindLLMProvider || kinds["dp-rest"] != pdk.KindRestAPI {
		t.Fatalf("candidates = %+v, want one per kind", cands)
	}
	if got := resp.Artifacts["dp-rest"]; got.ErrorCode != "quota_exceeded" {
		t.Errorf("REST entry = %+v, want refused", got)
	}
	if art := artifactByHandle(t, d, "shared-handle"); art != nil {
		t.Fatalf("a refused create went through behind a decoy: %+v", art)
	}
}

// H2: a blank handle is refused by the import itself, with or without admitters, so it can
// neither create an unaddressable artifact nor slip past admission.
func TestImportHooks_BlankHandleIsRefused(t *testing.T) {
	for _, withHooks := range []bool{false, true} {
		d := setupImportTest(t)
		admitter := &recordingAdmitter{}
		if withHooks {
			d.svc.SetGatewayImportHooks([]pdk.GatewayImportAdmitter{admitter}, nil)
		}
		resp := importBatch(d,
			restImportRequest("dp-space", " ", "Space"),
			restImportRequest("dp-tab", "\t", "Tab"),
		)
		if resp.Failed != 2 || resp.Success != 0 {
			t.Fatalf("hooks=%v: success/failed = %d/%d, want 0/2 (%+v)", withHooks, resp.Success, resp.Failed, resp.Artifacts)
		}
		for _, h := range []string{" ", "\t"} {
			if art := artifactByHandle(t, d, h); art != nil {
				t.Errorf("hooks=%v: blank handle %q was created", withHooks, h)
			}
		}
		if len(admitter.batches) != 0 {
			t.Errorf("hooks=%v: blank handles were offered: %+v", withHooks, admitter.batches)
		}
	}
}

// phantomRepo reports an artifact for one handle on the next lookups, then nothing: the artifact
// was deleted between the lookup that picks candidates and the one that decides the create.
type phantomRepo struct {
	repository.ArtifactRepository
	handle   string
	phantoms int
}

func (r *phantomRepo) GetByHandle(handle, orgUUID string) (*model.Artifact, error) {
	if handle == r.handle && r.phantoms > 0 {
		r.phantoms--
		return &model.Artifact{UUID: "deleted-meanwhile", Handle: handle, Type: constants.RestApi}, nil
	}
	return r.ArtifactRepository.GetByHandle(handle, orgUUID)
}

// H3: an artifact that was an update when candidates were picked, and is gone when the import
// decides, is not created: no admitter saw it. It fails as transient and is offered next push.
func TestImportHooks_UpdateTurnedCreateIsNotCreatedUnadmitted(t *testing.T) {
	d := setupImportTest(t)
	d.svc.artifactRepo = &phantomRepo{ArtifactRepository: d.artifactRepo, handle: "vanishing-api", phantoms: 1}
	admitter := &recordingAdmitter{}
	d.svc.SetGatewayImportHooks([]pdk.GatewayImportAdmitter{admitter}, nil)

	resp := importBatch(d, restImportRequest("dp-1", "vanishing-api", "Vanishing API"))

	got := resp.Artifacts["dp-1"]
	if got.Status != utils.ImportStatusFailed || got.Error == "" || got.ErrorCode != "" {
		t.Errorf("result = %+v, want a transient failure", got)
	}
	if art := artifactByHandle(t, d, "vanishing-api"); art != nil {
		t.Fatalf("a create no admitter saw went through: %+v", art)
	}
	if len(admitter.batches) != 0 {
		t.Errorf("admitter was offered %+v, want nothing (it looked like an update)", admitter.batches)
	}
}

// H3: a failed first lookup does not hold up an artifact that turns out to be an update: the
// import's own lookup finds it, and updates are never gated.
func TestImportHooks_UndecidableUpdateStillUpdates(t *testing.T) {
	d := setupImportTest(t)
	mustImport(t, d, withDeployedAt(restImportRequest("dp-1", "kept-api", "First"), baseDeployedAt))
	d.svc.artifactRepo = &flakyLookupRepo{ArtifactRepository: d.artifactRepo, handle: "kept-api", failures: 1}
	d.svc.SetGatewayImportHooks([]pdk.GatewayImportAdmitter{&recordingAdmitter{}}, nil)

	resp := importBatch(d, withDeployedAt(restImportRequest("dp-1", "kept-api", "Second"), newerDeployedAt))
	if resp.Success != 1 {
		t.Fatalf("update result = %+v, want success", resp.Artifacts["dp-1"])
	}
	if art := artifactByHandle(t, d, "kept-api"); art == nil || art.Name != "Second" {
		t.Errorf("artifact = %+v, want updated to Second", art)
	}
}

// M2 and M4: an admitter that cannot decide — an error or a panic — costs the creates it was
// offered, as transient failures, and nothing else: updates in the push still go through.
func TestImportHooks_AdmitterThatCannotDecideBlocksOnlyItsCreates(t *testing.T) {
	cases := map[string]*recordingAdmitter{
		"error": {err: errors.New("quota backend unavailable")},
		"panic": {panicValue: "boom"},
	}
	for name, admitter := range cases {
		t.Run(name, func(t *testing.T) {
			d := setupImportTest(t)
			mustImport(t, d, withDeployedAt(restImportRequest("dp-existing", "existing-api", "First"), baseDeployedAt))
			observer := &recordingObserver{}
			d.svc.SetGatewayImportHooks([]pdk.GatewayImportAdmitter{admitter}, []pdk.GatewayImportObserver{observer})

			resp := importBatch(d,
				withDeployedAt(restImportRequest("dp-existing", "existing-api", "Second"), newerDeployedAt),
				restImportRequest("dp-new", "new-api", "New API"),
			)

			if resp.Success != 1 || resp.Failed != 1 {
				t.Fatalf("success/failed = %d/%d, want 1/1 (%+v)", resp.Success, resp.Failed, resp.Artifacts)
			}
			if got := resp.Artifacts["dp-new"]; got.ErrorCode != "" || got.Error == "" {
				t.Errorf("new artifact result = %+v, want a transient failure", got)
			}
			if art := artifactByHandle(t, d, "new-api"); art != nil {
				t.Errorf("an undecided create went through: %+v", art)
			}
			if art := artifactByHandle(t, d, "existing-api"); art == nil || art.Name != "Second" {
				t.Errorf("update = %+v, want applied", art)
			}
			if len(observer.batches) != 0 {
				t.Errorf("observer told of %v, want nothing created", observer.createdDPIDs())
			}
		})
	}
}

// M4: a panicking observer does not lose the push's response, and later observers are still told.
func TestImportHooks_PanickingObserverDoesNotBreakThePush(t *testing.T) {
	d := setupImportTest(t)
	bad := &recordingObserver{panicValue: "boom"}
	good := &recordingObserver{}
	d.svc.SetGatewayImportHooks(nil, []pdk.GatewayImportObserver{bad, good})

	resp := importBatch(d, restImportRequest("dp-1", "an-api", "An API"))
	if resp.Success != 1 {
		t.Fatalf("result = %+v, want success", resp.Artifacts)
	}
	if got := good.createdDPIDs(); !equalStrings(got, []string{"dp-1"}) {
		t.Errorf("second observer told of %v, want dp-1", got)
	}
}

// M3: the hooks get the push's context without its cancellation, so a gateway that stops
// waiting can neither make a fail-open admitter let everything through nor lose a report.
func TestImportHooks_HooksDoNotSeeTheRequestCancelled(t *testing.T) {
	d := setupImportTest(t)
	admitter := &recordingAdmitter{}
	observer := &recordingObserver{}
	d.svc.SetGatewayImportHooks([]pdk.GatewayImportAdmitter{admitter}, []pdk.GatewayImportObserver{observer})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	d.svc.ImportArtifacts(ctx, importTestOrgID, importTestGatewayID,
		[]dto.ImportGatewayArtifactRequest{restImportRequest("dp-1", "an-api", "An API")})

	if len(admitter.ctxErrs) != 1 || admitter.ctxErrs[0] != nil {
		t.Errorf("admitter context errors = %v, want one call with a live context", admitter.ctxErrs)
	}
	if len(observer.ctxErrs) != 1 || observer.ctxErrs[0] != nil {
		t.Errorf("observer context errors = %v, want one call with a live context", observer.ctxErrs)
	}
}

// L1: a refusal is applied to the create it was made about, so an update that shares its DPID
// is not refused with it.
func TestImportHooks_RefusalDoesNotReachAnUpdateSharingItsDPID(t *testing.T) {
	d := setupImportTest(t)
	mustImport(t, d, withDeployedAt(restImportRequest("dp-old", "existing-api", "First"), baseDeployedAt))
	admitter := &recordingAdmitter{refuse: refuseDPIDs("dup")}
	d.svc.SetGatewayImportHooks([]pdk.GatewayImportAdmitter{admitter}, nil)

	importBatch(d,
		withDeployedAt(restImportRequest("dup", "existing-api", "Second"), newerDeployedAt),
		restImportRequest("dup", "new-api", "New API"),
	)

	if art := artifactByHandle(t, d, "existing-api"); art == nil || art.Name != "Second" {
		t.Errorf("update = %+v, want applied despite the shared DPID", art)
	}
	if art := artifactByHandle(t, d, "new-api"); art != nil {
		t.Errorf("refused create went through: %+v", art)
	}
}

// Without hooks, a push behaves exactly as it always has.
func TestImportHooks_NoHooksImportsEverything(t *testing.T) {
	d := setupImportTest(t)
	resp := importBatch(d,
		restImportRequest("dp-1", "one-api", "One API"),
		restImportRequest("dp-2", "two-api", "Two API"),
	)
	if resp.Success != 2 || resp.Failed != 0 {
		t.Fatalf("success/failed = %d/%d, want 2/0", resp.Success, resp.Failed)
	}
}

// A malformed push that repeats a DPID across two new artifacts: admitting it admits both, and
// refusing it refuses both, rather than whichever one the DPID happened to map to last.
func TestImportHooks_RepeatedDPIDIsDecidedForEveryArtifactUnderIt(t *testing.T) {
	t.Run("admitted", func(t *testing.T) {
		d := setupImportTest(t)
		d.svc.SetGatewayImportHooks([]pdk.GatewayImportAdmitter{&recordingAdmitter{}}, nil)
		importBatch(d,
			restImportRequest("dp-shared", "first-api", "First"),
			restImportRequest("dp-shared", "second-api", "Second"),
		)
		for _, h := range []string{"first-api", "second-api"} {
			if art := artifactByHandle(t, d, h); art == nil {
				t.Errorf("%s was admitted but not created", h)
			}
		}
	})
	t.Run("refused", func(t *testing.T) {
		d := setupImportTest(t)
		d.svc.SetGatewayImportHooks([]pdk.GatewayImportAdmitter{&recordingAdmitter{refuse: refuseDPIDs("dp-shared")}}, nil)
		importBatch(d,
			restImportRequest("dp-shared", "first-api", "First"),
			restImportRequest("dp-shared", "second-api", "Second"),
		)
		for _, h := range []string{"first-api", "second-api"} {
			if art := artifactByHandle(t, d, h); art != nil {
				t.Errorf("%s was created under a refused DPID", h)
			}
		}
	})
}
