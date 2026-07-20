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
	"database/sql"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"

	_ "github.com/mattn/go-sqlite3"
)

// validUpstream returns a minimal API-level upstream that passes validation.
func validUpstream() api.Upstream {
	u := "http://backend"
	return api.Upstream{Main: api.UpstreamDefinition{Url: &u}}
}

// newAPIOperationUpstreamTarget constructs the anonymous ref-only target type
// generated for OperationUpstream.main and OperationUpstream.sandbox.
func newAPIOperationUpstreamTarget(ref string) *struct {
	Ref api.UpstreamReference `json:"ref" yaml:"ref"`
} {
	return &struct {
		Ref api.UpstreamReference `json:"ref" yaml:"ref"`
	}{Ref: ref}
}

// TestValidateUpstreamRefs verifies that API-level and per-operation upstream refs must name a
// declared upstreamDefinition, that the pool itself is well-formed, and that duplicate
// definition names are rejected.
func TestValidateUpstreamRefs(t *testing.T) {
	s := &APIService{}
	mainURL := "http://main:8080"
	validUp := api.Upstream{Main: api.UpstreamDefinition{Url: &mainURL}}
	ru := func(name string) api.ReusableUpstream {
		r := api.ReusableUpstream{Name: name}
		r.Upstreams = append(r.Upstreams, struct {
			Url    string `json:"url" yaml:"url"`
			Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
		}{Url: "http://b:8080"})
		return r
	}
	defs := &[]api.ReusableUpstream{ru("alt-backend"), ru("foo")}
	opWithMainRef := func(ref string) *[]api.Operation {
		return &[]api.Operation{
			{Request: api.OperationRequest{
				Method:   api.OperationRequestMethodGET,
				Path:     "/x",
				Upstream: &api.OperationUpstream{Main: newAPIOperationUpstreamTarget(ref)},
			}},
		}
	}

	t.Run("per-op main ref resolves", func(t *testing.T) {
		if err := s.validateUpstreamRefs(defs, validUp, opWithMainRef("alt-backend")); err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})
	t.Run("per-op sandbox ref resolves", func(t *testing.T) {
		ops := &[]api.Operation{
			{Request: api.OperationRequest{
				Method:   api.OperationRequestMethodGET,
				Path:     "/x",
				Upstream: &api.OperationUpstream{Sandbox: newAPIOperationUpstreamTarget("foo")},
			}},
		}
		if err := s.validateUpstreamRefs(defs, validUp, ops); err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})
	t.Run("per-op ref unresolved", func(t *testing.T) {
		if err := s.validateUpstreamRefs(defs, validUp, opWithMainRef("missing")); err == nil {
			t.Error("expected error for undefined per-op ref")
		}
	})
	t.Run("API-level ref unresolved", func(t *testing.T) {
		ref := "nope"
		up := api.Upstream{Main: api.UpstreamDefinition{Ref: &ref}}
		if err := s.validateUpstreamRefs(defs, up, nil); err == nil {
			t.Error("expected error for undefined API-level ref")
		}
	})
	t.Run("API-level url and ref rejected", func(t *testing.T) {
		urlValue := "http://main:8080"
		ref := "alt-backend"
		up := api.Upstream{Main: api.UpstreamDefinition{Url: &urlValue, Ref: &ref}}
		err := s.validateUpstreamRefs(defs, up, nil)
		if err == nil || !strings.Contains(err.Error(), "upstream.main") {
			t.Fatalf("expected upstream.main url/ref error, got %v", err)
		}
	})
	t.Run("API-level missing url and ref rejected", func(t *testing.T) {
		err := s.validateUpstreamRefs(defs, api.Upstream{}, nil)
		if err == nil || !strings.Contains(err.Error(), "upstream.main") {
			t.Fatalf("expected upstream.main missing target error, got %v", err)
		}
	})
	t.Run("API-level ref name contract enforced", func(t *testing.T) {
		ref := "bad ref!"
		up := api.Upstream{Main: api.UpstreamDefinition{Ref: &ref}}
		err := s.validateUpstreamRefs(defs, up, nil)
		if err == nil || !strings.Contains(err.Error(), "upstream.main.ref") {
			t.Fatalf("expected upstream.main.ref name error, got %v", err)
		}
	})
	t.Run("API-level url with surrounding whitespace rejected", func(t *testing.T) {
		// The gateway parses the original (untrimmed) URL, so the platform must too;
		// validating a trimmed copy would persist a URL the gateway rejects at deploy.
		padded := " http://main:8080 "
		up := api.Upstream{Main: api.UpstreamDefinition{Url: &padded}}
		if err := s.validateUpstreamRefs(nil, up, nil); err == nil {
			t.Error("expected error for API-level url with surrounding whitespace")
		}
	})
	t.Run("duplicate definition names", func(t *testing.T) {
		dup := &[]api.ReusableUpstream{ru("x"), ru("x")}
		if err := s.validateUpstreamRefs(dup, validUp, nil); err == nil {
			t.Error("expected error for duplicate definition names")
		}
	})
	t.Run("definition with zero upstreams rejected", func(t *testing.T) {
		bare := &[]api.ReusableUpstream{{Name: "bare"}}
		if err := s.validateUpstreamRefs(bare, validUp, nil); err == nil {
			t.Error("expected error for definition with no upstreams")
		}
	})
	mkDef := func(name, u string) api.ReusableUpstream {
		d := api.ReusableUpstream{Name: name}
		d.Upstreams = append(d.Upstreams, struct {
			Url    string `json:"url" yaml:"url"`
			Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
		}{Url: u})
		return d
	}
	wantErr := func(t *testing.T, defs []api.ReusableUpstream, ops *[]api.Operation, what string) {
		if err := s.validateUpstreamRefs(&defs, validUp, ops); err == nil {
			t.Errorf("expected error for %s", what)
		}
	}
	t.Run("non-first upstream with empty url rejected", func(t *testing.T) {
		d := mkDef("multi", "http://b1:8080")
		d.Upstreams = append(d.Upstreams, struct {
			Url    string `json:"url" yaml:"url"`
			Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
		}{Url: ""})
		wantErr(t, []api.ReusableUpstream{d}, nil, "empty url on a non-first upstream")
	})
	t.Run("zero weight accepted", func(t *testing.T) {
		zero := 0
		d := mkDef("w", "http://b:8080")
		d.Upstreams[0].Weight = &zero
		if err := s.validateUpstreamRefs(&[]api.ReusableUpstream{d}, validUp, nil); err != nil {
			t.Errorf("zero weight should be valid (0..100), got %v", err)
		}
	})
	t.Run("weight over 100 rejected", func(t *testing.T) {
		over := 105
		d := mkDef("w", "http://b:8080")
		d.Upstreams[0].Weight = &over
		wantErr(t, []api.ReusableUpstream{d}, nil, "weight > 100")
	})
	t.Run("negative weight rejected", func(t *testing.T) {
		neg := -10
		d := mkDef("w", "http://b:8080")
		d.Upstreams[0].Weight = &neg
		wantErr(t, []api.ReusableUpstream{d}, nil, "weight < 0")
	})
	t.Run("weight of exactly 100 accepted", func(t *testing.T) {
		hundred := 100
		d := mkDef("w", "http://b:8080")
		d.Upstreams[0].Weight = &hundred
		if err := s.validateUpstreamRefs(&[]api.ReusableUpstream{d}, validUp, nil); err != nil {
			t.Errorf("weight 100 should be valid (0..100), got %v", err)
		}
	})
	t.Run("def name bad chars rejected", func(t *testing.T) {
		wantErr(t, []api.ReusableUpstream{mkDef("bad name!", "http://b:8080")}, nil, "bad definition name")
	})
	t.Run("def name too long rejected", func(t *testing.T) {
		wantErr(t, []api.ReusableUpstream{mkDef(strings.Repeat("a", 101), "http://b:8080")}, nil, "definition name > 100")
	})
	t.Run("def url with path rejected", func(t *testing.T) {
		wantErr(t, []api.ReusableUpstream{mkDef("d", "http://b:8080/foo")}, nil, "url with path")
	})
	t.Run("def url with query rejected", func(t *testing.T) {
		wantErr(t, []api.ReusableUpstream{mkDef("d", "http://b:8080?debug=true")}, nil, "url with query")
	})
	t.Run("def url with fragment rejected", func(t *testing.T) {
		wantErr(t, []api.ReusableUpstream{mkDef("d", "http://b:8080#section")}, nil, "url with fragment")
	})
	t.Run("def url bad scheme rejected", func(t *testing.T) {
		wantErr(t, []api.ReusableUpstream{mkDef("d", "ftp://b:8080")}, nil, "url bad scheme")
	})
	t.Run("def url no host rejected", func(t *testing.T) {
		wantErr(t, []api.ReusableUpstream{mkDef("d", "http://")}, nil, "url no host")
	})
	mkDefWithBasePath := func(basePath string) api.ReusableUpstream {
		d := mkDef("d", "http://b:8080")
		d.BasePath = &basePath
		return d
	}
	t.Run("def basePath without leading slash rejected", func(t *testing.T) {
		wantErr(t, []api.ReusableUpstream{mkDefWithBasePath("api/v2")}, nil, "basePath without leading '/'")
	})
	t.Run("def basePath with trailing slash rejected", func(t *testing.T) {
		wantErr(t, []api.ReusableUpstream{mkDefWithBasePath("/api/v2/")}, nil, "basePath with trailing '/'")
	})
	t.Run("def basePath bare root slash rejected", func(t *testing.T) {
		wantErr(t, []api.ReusableUpstream{mkDefWithBasePath("/")}, nil, "bare '/' basePath (root is expressed by omitting)")
	})
	t.Run("def basePath with space rejected", func(t *testing.T) {
		wantErr(t, []api.ReusableUpstream{mkDefWithBasePath("/api v2")}, nil, "basePath with a space")
	})
	t.Run("def basePath with query rejected", func(t *testing.T) {
		wantErr(t, []api.ReusableUpstream{mkDefWithBasePath("/api?x=1")}, nil, "basePath with a query string")
	})
	t.Run("def valid basePath accepted", func(t *testing.T) {
		if err := s.validateUpstreamRefs(&[]api.ReusableUpstream{mkDefWithBasePath("/api/v2")}, validUp, nil); err != nil {
			t.Errorf("basePath /api/v2 should be valid, got %v", err)
		}
	})
	t.Run("def empty basePath treated as unset", func(t *testing.T) {
		// omitempty drops an empty basePath from the deployment YAML, so the gateway
		// never sees it; rejecting it here would be stricter than the deploy contract.
		if err := s.validateUpstreamRefs(&[]api.ReusableUpstream{mkDefWithBasePath("")}, validUp, nil); err != nil {
			t.Errorf("empty basePath should be treated as unset, got %v", err)
		}
	})
	t.Run("per-op empty ref rejected", func(t *testing.T) {
		defs := []api.ReusableUpstream{mkDef("svc", "http://b:8080")}
		ops := []api.Operation{{Request: api.OperationRequest{Method: api.OperationRequestMethodGET, Path: "/x",
			Upstream: &api.OperationUpstream{Main: newAPIOperationUpstreamTarget("")}}}}
		wantErr(t, defs, &ops, "empty per-op ref")
	})
	t.Run("per-op ref name contract enforced", func(t *testing.T) {
		defs := []api.ReusableUpstream{mkDef("svc", "http://b:8080")}
		ops := []api.Operation{{Request: api.OperationRequest{Method: api.OperationRequestMethodGET, Path: "/x",
			Upstream: &api.OperationUpstream{Main: newAPIOperationUpstreamTarget("bad ref!")}}}}
		err := s.validateUpstreamRefs(&defs, validUp, &ops)
		if err == nil || !strings.Contains(err.Error(), "operations[0].upstream.main.ref") {
			t.Fatalf("expected indexed per-op ref error, got %v", err)
		}
	})
	t.Run("per-op empty wrapper rejected", func(t *testing.T) {
		defs := []api.ReusableUpstream{mkDef("svc", "http://b:8080")}
		ops := []api.Operation{{Request: api.OperationRequest{Method: api.OperationRequestMethodGET, Path: "/x",
			Upstream: &api.OperationUpstream{}}}}
		wantErr(t, defs, &ops, "empty per-op upstream wrapper")
	})
	t.Run("invalid timeout.connect rejected", func(t *testing.T) {
		bad := "5x"
		d := ru("t")
		d.Timeout = &api.UpstreamTimeout{Connect: &bad}
		if err := s.validateUpstreamRefs(&[]api.ReusableUpstream{d}, validUp, nil); err == nil {
			t.Error("expected error for invalid timeout.connect")
		}
	})
	t.Run("non-positive timeout.connect rejected", func(t *testing.T) {
		for _, v := range []string{"0s", "-1s"} {
			d := ru("t")
			d.Timeout = &api.UpstreamTimeout{Connect: &v}
			if err := s.validateUpstreamRefs(&[]api.ReusableUpstream{d}, validUp, nil); err == nil {
				t.Errorf("expected error for non-positive timeout.connect %q", v)
			}
		}
	})
	t.Run("valid timeout.connect accepted", func(t *testing.T) {
		good := "5s"
		d := ru("t")
		d.Timeout = &api.UpstreamTimeout{Connect: &good}
		if err := s.validateUpstreamRefs(&[]api.ReusableUpstream{d}, validUp, nil); err != nil {
			t.Errorf("expected nil for valid timeout.connect, got %v", err)
		}
	})
	t.Run("blank timeout.connect accepted", func(t *testing.T) {
		// The gateway trims and treats a blank or whitespace-only connect as unset, so the
		// platform must accept it too rather than reject a config the gateway would deploy.
		for _, v := range []string{"", "   "} {
			d := ru("t")
			d.Timeout = &api.UpstreamTimeout{Connect: &v}
			if err := s.validateUpstreamRefs(&[]api.ReusableUpstream{d}, validUp, nil); err != nil {
				t.Errorf("expected nil for blank timeout.connect %q, got %v", v, err)
			}
		}
	})
	t.Run("non-canonical timeout.connect unit rejected", func(t *testing.T) {
		// time.ParseDuration accepts these, but the gateway only allows ms, s, m, h, so the
		// control plane must reject them too or the API saves and then fails to deploy.
		for _, v := range []string{"1h30m", "500ns", "500us"} {
			d := ru("t")
			d.Timeout = &api.UpstreamTimeout{Connect: &v}
			if err := s.validateUpstreamRefs(&[]api.ReusableUpstream{d}, validUp, nil); err == nil {
				t.Errorf("expected error for non-canonical timeout.connect unit %q", v)
			}
		}
	})
	t.Run("direct URL without definitions is valid", func(t *testing.T) {
		if err := s.validateUpstreamRefs(nil, validUp, nil); err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})
}

const upstreamITOrgUUID = "org-upstream-it"
const upstreamITProjectUUID = "proj-upstream-it"
const upstreamITProjectHandle = "upstream-it-proj"

// setupUpstreamITEnv creates a real SQLite-backed APIService (real service and
// repositories, no mocks; collaborators these tests never reach are nil) plus a
// seeded organization and project.
func setupUpstreamITEnv(t *testing.T) *APIService {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "upstream-it.db")
	sqlDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if _, err := sqlDB.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	db := &database.DB{DB: sqlDB}
	t.Cleanup(func() { sqlDB.Close() })

	schemaPath := filepath.Join("..", "database", "schema.sqlite.sql")
	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if _, err = db.Exec(string(schema)); err != nil {
		t.Fatalf("apply schema: %v", err)
	}

	if _, err = db.Exec(`INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid, created_at, updated_at)
		VALUES (?, 'upstream-it-org', 'Upstream IT Org', 'default', 'idp-ref', datetime('now'), datetime('now'))`, upstreamITOrgUUID); err != nil {
		t.Fatalf("insert org: %v", err)
	}
	if _, err = db.Exec(`INSERT INTO projects (uuid, handle, display_name, description, organization_uuid, created_at, updated_at)
		VALUES (?, ?, 'Upstream IT Project', '', ?, datetime('now'), datetime('now'))`, upstreamITProjectUUID, upstreamITProjectHandle, upstreamITOrgUUID); err != nil {
		t.Fatalf("insert project: %v", err)
	}

	identity := NewIdentityService(repository.NewUserIdentityMappingRepo(db))
	apiRepo := repository.NewAPIRepo(db)
	projectRepo := repository.NewProjectRepo(db)
	auditRepo := repository.NewAuditRepo(db)

	return NewAPIService(apiRepo, projectRepo, nil, nil, nil, nil, nil, nil, &utils.APIUtil{}, slog.Default(), auditRepo, identity)
}

// perOpReusableUpstream builds a pool entry with a single backend.
func perOpReusableUpstream(name, basePath, backendURL string) api.ReusableUpstream {
	r := api.ReusableUpstream{Name: name}
	if basePath != "" {
		r.BasePath = &basePath
	}
	r.Upstreams = append(r.Upstreams, struct {
		Url    string `json:"url" yaml:"url"`
		Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
	}{Url: backendURL})
	return r
}

// perOpCreateRequest builds a create request with a pool and a per-operation ref on /whoami.
func perOpCreateRequest() *api.CreateRESTAPIRequest {
	handle := "perop-svc-it-api"
	mainURL := "http://default-backend:8080"
	defs := []api.ReusableUpstream{perOpReusableUpstream("alt-backend", "/alternate", "http://alt-backend:9090")}
	ops := []api.Operation{
		{Request: api.OperationRequest{
			Method: api.OperationRequestMethodGET, Path: "/whoami",
			Upstream: &api.OperationUpstream{Main: newAPIOperationUpstreamTarget("alt-backend")},
		}},
		{Request: api.OperationRequest{Method: api.OperationRequestMethodGET, Path: "/ping"}},
	}
	return &api.CreateRESTAPIRequest{
		DisplayName:         "PerOp Svc IT API",
		Id:                  &handle,
		Context:             "/perop-svc-it",
		Version:             "1.0.0",
		ProjectId:           upstreamITProjectHandle,
		Upstream:            api.Upstream{Main: api.UpstreamDefinition{Url: &mainURL}},
		UpstreamDefinitions: &defs,
		Operations:          &ops,
	}
}

// assertPoolAndPerOp checks that a RESTAPI carries the reusable pool and that the per-op
// ref is attached to /whoami (and not inherited by /ping).
func assertPoolAndPerOp(t *testing.T, where string, a *api.RESTAPI) {
	t.Helper()
	if a.UpstreamDefinitions == nil || len(*a.UpstreamDefinitions) != 1 {
		t.Fatalf("%s: want 1 upstreamDefinition, got %+v", where, a.UpstreamDefinitions)
	}
	def := (*a.UpstreamDefinitions)[0]
	if def.Name != "alt-backend" {
		t.Errorf("%s: pool name mismatch: %+v", where, def)
	}
	if def.BasePath == nil || *def.BasePath != "/alternate" {
		t.Errorf("%s: pool basePath mismatch: %+v", where, def.BasePath)
	}
	if len(def.Upstreams) != 1 || def.Upstreams[0].Url != "http://alt-backend:9090" {
		t.Errorf("%s: pool backends mismatch: %+v", where, def.Upstreams)
	}
	if a.Operations == nil || len(*a.Operations) != 2 {
		t.Fatalf("%s: want 2 operations, got %+v", where, a.Operations)
	}
	var whoami, ping *api.Operation
	for i := range *a.Operations {
		op := &(*a.Operations)[i]
		switch op.Request.Path {
		case "/whoami":
			whoami = op
		case "/ping":
			ping = op
		}
	}
	if whoami == nil || whoami.Request.Upstream == nil || whoami.Request.Upstream.Main == nil ||
		whoami.Request.Upstream.Main.Ref != "alt-backend" {
		t.Errorf("%s: /whoami per-op ref missing or wrong: %+v", where, whoami)
	}
	if ping == nil || ping.Request.Upstream != nil {
		t.Errorf("%s: /ping must not inherit a per-op upstream: %+v", where, ping)
	}
}

// TestAPIService_CreatePersistAndReadPerOpUpstream proves the pool and per-operation ref
// survive create and read through the real service and repositories.
func TestAPIService_CreatePersistAndReadPerOpUpstream(t *testing.T) {
	svc := setupUpstreamITEnv(t)

	created, err := svc.CreateAPI(perOpCreateRequest(), upstreamITOrgUUID, "tester")
	if err != nil {
		t.Fatalf("CreateAPI: %v", err)
	}
	assertPoolAndPerOp(t, "create", created)

	read, err := svc.GetAPIByHandle("perop-svc-it-api", upstreamITOrgUUID)
	if err != nil {
		t.Fatalf("GetAPIByHandle: %v", err)
	}
	assertPoolAndPerOp(t, "read", read)
}

// TestAPIService_CreateRejectsUnresolvedPerOpRef ensures a per-operation ref that does
// not resolve to a declared upstreamDefinition is rejected as a validation failure,
// not silently persisted.
func TestAPIService_CreateRejectsUnresolvedPerOpRef(t *testing.T) {
	svc := setupUpstreamITEnv(t)

	req := perOpCreateRequest()
	badHandle := "bad-ref-api"
	req.Id = &badHandle
	req.Context = "/perop-svc-it-bad"
	req.UpstreamDefinitions = nil // drop the pool so the per-op ref no longer resolves

	if _, err := svc.CreateAPI(req, upstreamITOrgUUID, "tester"); err == nil {
		t.Fatal("expected error for unresolved per-op ref")
	} else if !apperror.ValidationFailed.Is(err) {
		t.Errorf("want ValidationFailed, got %v", err)
	}
}

// TestAPIService_UpdateRejectsPoolRemovalWithDanglingRef ensures an update that replaces the
// pool cannot orphan a stored per-operation ref: validation runs on the merged result, so
// removing the referenced definition is rejected instead of persisting a config the gateway
// would refuse to deploy.
func TestAPIService_UpdateRejectsPoolRemovalWithDanglingRef(t *testing.T) {
	svc := setupUpstreamITEnv(t)

	if _, err := svc.CreateAPI(perOpCreateRequest(), upstreamITOrgUUID, "tester"); err != nil {
		t.Fatalf("CreateAPI: %v", err)
	}

	// Replace the pool with one that no longer contains "alt-backend", which the stored
	// /whoami operation still references.
	newPool := []api.ReusableUpstream{perOpReusableUpstream("other-backend", "/other", "http://other:9090")}
	_, err := svc.UpdateAPIByHandle("perop-svc-it-api", &api.RESTAPI{UpstreamDefinitions: &newPool}, upstreamITOrgUUID, "tester")
	if err == nil {
		t.Fatal("expected error for pool removal with dangling per-op ref")
	} else if !apperror.ValidationFailed.Is(err) {
		t.Errorf("want ValidationFailed, got %v", err)
	}
}

// TestAPIService_UpdateReplacesPoolAndRefsTogether proves a coordinated update that swaps
// the pool and repoints the operation refs in the same request succeeds and persists.
func TestAPIService_UpdateReplacesPoolAndRefsTogether(t *testing.T) {
	svc := setupUpstreamITEnv(t)

	if _, err := svc.CreateAPI(perOpCreateRequest(), upstreamITOrgUUID, "tester"); err != nil {
		t.Fatalf("CreateAPI: %v", err)
	}

	newPool := []api.ReusableUpstream{perOpReusableUpstream("new-backend", "/renewed", "http://new-backend:9191")}
	newOps := []api.Operation{
		{Request: api.OperationRequest{
			Method: api.OperationRequestMethodGET, Path: "/whoami",
			Upstream: &api.OperationUpstream{Main: newAPIOperationUpstreamTarget("new-backend")},
		}},
		{Request: api.OperationRequest{Method: api.OperationRequestMethodGET, Path: "/ping"}},
	}
	updated, err := svc.UpdateAPIByHandle("perop-svc-it-api",
		&api.RESTAPI{UpstreamDefinitions: &newPool, Operations: &newOps}, upstreamITOrgUUID, "tester")
	if err != nil {
		t.Fatalf("UpdateAPIByHandle: %v", err)
	}

	assertNewPool := func(where string, a *api.RESTAPI) {
		t.Helper()
		if a.UpstreamDefinitions == nil || len(*a.UpstreamDefinitions) != 1 ||
			(*a.UpstreamDefinitions)[0].Name != "new-backend" {
			t.Fatalf("%s: want pool [new-backend], got %+v", where, a.UpstreamDefinitions)
		}
		def := (*a.UpstreamDefinitions)[0]
		if len(def.Upstreams) != 1 || def.Upstreams[0].Url != "http://new-backend:9191" {
			t.Errorf("%s: pool backends mismatch: %+v", where, def.Upstreams)
		}
		if a.Operations == nil || len(*a.Operations) != 2 {
			t.Fatalf("%s: want 2 operations, got %+v", where, a.Operations)
		}
		for i := range *a.Operations {
			op := (*a.Operations)[i]
			if op.Request.Path == "/whoami" {
				if op.Request.Upstream == nil || op.Request.Upstream.Main == nil ||
					op.Request.Upstream.Main.Ref != "new-backend" {
					t.Errorf("%s: /whoami must ref new-backend: %+v", where, op.Request.Upstream)
				}
			}
		}
	}
	assertNewPool("update response", updated)

	read, err := svc.GetAPIByHandle("perop-svc-it-api", upstreamITOrgUUID)
	if err != nil {
		t.Fatalf("GetAPIByHandle: %v", err)
	}
	assertNewPool("read after update", read)
}

// TestAPIService_FailedUpdateLeavesStoredStateUnchanged ensures a rejected update (a pool
// replacement that would dangle a stored per-operation ref) does not modify the stored
// configuration.
func TestAPIService_FailedUpdateLeavesStoredStateUnchanged(t *testing.T) {
	svc := setupUpstreamITEnv(t)

	if _, err := svc.CreateAPI(perOpCreateRequest(), upstreamITOrgUUID, "tester"); err != nil {
		t.Fatalf("CreateAPI: %v", err)
	}

	newPool := []api.ReusableUpstream{perOpReusableUpstream("other-backend", "/other", "http://other:9090")}
	if _, err := svc.UpdateAPIByHandle("perop-svc-it-api",
		&api.RESTAPI{UpstreamDefinitions: &newPool}, upstreamITOrgUUID, "tester"); err == nil {
		t.Fatal("expected error for pool removal with dangling per-op ref")
	}

	read, err := svc.GetAPIByHandle("perop-svc-it-api", upstreamITOrgUUID)
	if err != nil {
		t.Fatalf("GetAPIByHandle: %v", err)
	}
	assertPoolAndPerOp(t, "after failed update", read)
}
