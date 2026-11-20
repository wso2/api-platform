/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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
 *
 */

package service

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

const storedCredential = "stored-upstream-credential"

// errStopAfterCapture ends Update once the model to persist has been captured.
var errStopAfterCapture = errors.New("stop after capture")

func storedUpstreamConfig() model.UpstreamConfig {
	return model.UpstreamConfig{
		Main: &model.UpstreamEndpoint{
			URL: "https://backend.example.com/events",
			Auth: &model.UpstreamAuth{
				Type:   "api-key",
				Header: "X-API-Key",
				Value:  storedCredential,
			},
		},
		Sandbox: &model.UpstreamEndpoint{
			URL: "https://sandbox.example.com/events",
			Auth: &model.UpstreamAuth{
				Type:   "bearer",
				Header: "Authorization",
				Value:  storedCredential + "-sandbox",
			},
		},
	}
}

func authTypePtr(t api.UpstreamAuthType) *api.UpstreamAuthType { return &t }

// mapUpstreamModelToAPI backs every WebSub response shape that carries
// upstream. Type and Header identify the scheme and must survive; the value is
// writeOnly and must never appear on the wire.
func TestMapUpstreamModelToAPI_DoesNotReturnTheAuthValue(t *testing.T) {
	stored := storedUpstreamConfig()

	got := mapUpstreamModelToAPI(&stored)

	if got.Main.Auth == nil {
		t.Fatal("main auth block dropped; type and header must survive the mapping")
	}
	if got.Main.Auth.Value != nil {
		t.Errorf("main auth value returned on read: got %q, want nil", *got.Main.Auth.Value)
	}
	if got.Main.Auth.Header == nil || *got.Main.Auth.Header != "X-API-Key" {
		t.Errorf("main auth header not preserved: got %v", got.Main.Auth.Header)
	}
	if got.Main.Auth.Type == nil || *got.Main.Auth.Type != api.ApiKey {
		t.Errorf("main auth type not preserved: got %v", got.Main.Auth.Type)
	}
	if got.Main.Url == nil || *got.Main.Url != "https://backend.example.com/events" {
		t.Errorf("main url not preserved: got %v", got.Main.Url)
	}

	if got.Sandbox == nil || got.Sandbox.Auth == nil {
		t.Fatal("sandbox auth block dropped")
	}
	if got.Sandbox.Auth.Value != nil {
		t.Errorf("sandbox auth value returned on read: got %q, want nil", *got.Sandbox.Auth.Value)
	}

	// The Go field being nil is not the contract; the contract is what goes on
	// the wire. Assert the serialized form so dropping `omitempty` from the
	// tag, or any other marshalling change, fails here too.
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("failed to marshal the upstream response: %v", err)
	}
	if strings.Contains(string(encoded), `"value"`) {
		t.Errorf("upstream response carries a value field: %s", encoded)
	}
	if strings.Contains(string(encoded), storedCredential) {
		t.Errorf("upstream response carries the stored credential: %s", encoded)
	}
}

// A client that round-trips a GET response sends back an auth block with no
// value. That means "unchanged", not "clear it".
func TestPreserveWebSubUpstreamAuth_EmptyValueKeepsStoredCredential(t *testing.T) {
	stored := storedUpstreamConfig()
	roundTripped := api.Upstream{
		Main: api.UpstreamDefinition{
			Url:  utils.StringPtrIfNotEmpty("https://backend.example.com/events"),
			Auth: &api.UpstreamAuth{Type: authTypePtr(api.ApiKey), Header: utils.StringPtrIfNotEmpty("X-API-Key")},
		},
		Sandbox: &api.UpstreamDefinition{
			Url:  utils.StringPtrIfNotEmpty("https://sandbox.example.com/events"),
			Auth: &api.UpstreamAuth{Type: authTypePtr(api.Bearer), Header: utils.StringPtrIfNotEmpty("Authorization")},
		},
	}

	got := preserveWebSubUpstreamAuth(stored, roundTripped)

	if got.Main.Auth.Value == nil || *got.Main.Auth.Value != storedCredential {
		t.Errorf("main credential not preserved: got %v, want %q", got.Main.Auth.Value, storedCredential)
	}
	if got.Sandbox.Auth.Value == nil || *got.Sandbox.Auth.Value != storedCredential+"-sandbox" {
		t.Errorf("sandbox credential not preserved: got %v", got.Sandbox.Auth.Value)
	}
}

func TestPreserveWebSubUpstreamAuth_ExplicitValueReplacesStoredCredential(t *testing.T) {
	stored := storedUpstreamConfig()
	updated := api.Upstream{
		Main: api.UpstreamDefinition{
			Auth: &api.UpstreamAuth{
				Type:   authTypePtr(api.ApiKey),
				Header: utils.StringPtrIfNotEmpty("X-API-Key"),
				Value:  utils.StringPtrIfNotEmpty("rotated-credential"),
			},
		},
	}

	got := preserveWebSubUpstreamAuth(stored, updated)

	if got.Main.Auth.Value == nil || *got.Main.Auth.Value != "rotated-credential" {
		t.Errorf("new credential not applied: got %v", got.Main.Auth.Value)
	}
}

// An omitted auth block leaves the endpoint as configured rather than
// silently dropping its credential.
func TestPreserveWebSubUpstreamAuth_OmittedAuthBlockKeepsStoredCredential(t *testing.T) {
	stored := storedUpstreamConfig()
	updated := api.Upstream{Main: api.UpstreamDefinition{Url: utils.StringPtrIfNotEmpty("https://backend.example.com/events")}}

	got := preserveWebSubUpstreamAuth(stored, updated)

	if got.Main.Auth == nil {
		t.Fatal("auth block dropped when the request omitted it")
	}
	if got.Main.Auth.Value == nil || *got.Main.Auth.Value != storedCredential {
		t.Errorf("credential not preserved: got %v", got.Main.Auth.Value)
	}
}

// Changing the auth scheme must not reattach the previous scheme's secret.
func TestPreserveWebSubUpstreamAuth_AuthTypeChangeDoesNotReuseStoredCredential(t *testing.T) {
	stored := storedUpstreamConfig()
	updated := api.Upstream{
		Main: api.UpstreamDefinition{
			Auth: &api.UpstreamAuth{Type: authTypePtr(api.Basic), Header: utils.StringPtrIfNotEmpty("Authorization")},
		},
	}

	got := preserveWebSubUpstreamAuth(stored, updated)

	if got.Main.Auth.Value != nil {
		t.Errorf("stored credential reattached across an auth type change: got %q", *got.Main.Auth.Value)
	}
}

// An API with no stored credential must not gain one.
func TestPreserveWebSubUpstreamAuth_NoStoredCredentialIsANoOp(t *testing.T) {
	stored := model.UpstreamConfig{Main: &model.UpstreamEndpoint{URL: "https://backend.example.com/events"}}
	updated := api.Upstream{Main: api.UpstreamDefinition{Auth: &api.UpstreamAuth{Type: authTypePtr(api.ApiKey)}}}

	got := preserveWebSubUpstreamAuth(stored, updated)

	if got.Main.Auth != nil && got.Main.Auth.Value != nil {
		t.Errorf("credential invented where none was stored: got %q", *got.Main.Auth.Value)
	}
}

// The round trip as a whole: read redacts, update preserves, nothing is lost.
func TestWebSubUpstreamAuth_ReadThenUpdateRoundTripKeepsCredential(t *testing.T) {
	stored := storedUpstreamConfig()

	readBack := mapUpstreamModelToAPI(&stored)
	if readBack.Main.Auth.Value != nil {
		t.Fatal("read returned the credential")
	}

	saved := mapUpstreamAPIToModel(preserveWebSubUpstreamAuth(stored, readBack))

	if saved.Main.Auth == nil || saved.Main.Auth.Value != storedCredential {
		t.Errorf("credential lost on a read-modify-write round trip: got %v", saved.Main.Auth)
	}
	if saved.Sandbox == nil || saved.Sandbox.Auth == nil || saved.Sandbox.Auth.Value != storedCredential+"-sandbox" {
		t.Errorf("sandbox credential lost on round trip: got %v", saved.Sandbox)
	}
}

// capturingWebSubRepo records the model handed to Update and then fails the
// write, so a test can assert on what would have been persisted without
// needing the rest of the service's dependencies.
type capturingWebSubRepo struct {
	repository.WebSubAPIRepository
	stored  *model.WebSubAPI
	saved   *model.WebSubAPI
	errStop error
}

func (r *capturingWebSubRepo) GetByHandle(handle, orgUUID string) (*model.WebSubAPI, error) {
	return r.stored, nil
}

func (r *capturingWebSubRepo) Update(a *model.WebSubAPI) error {
	r.saved = a
	return r.errStop
}

// The helper being correct is not enough — Update has to actually call it.
// A WebSub API saved from a redacted read must keep its stored credential.
func TestWebSubAPIService_UpdateDoesNotClearTheStoredCredential(t *testing.T) {
	stored := storedUpstreamConfig()
	repo := &capturingWebSubRepo{
		stored: &model.WebSubAPI{
			Handle:           "my-websub-api",
			OrganizationUUID: "org-1",
			Name:             "My WebSub API",
			Version:          "v1.0",
			Origin:           constants.OriginCP,
			Configuration: model.WebSubAPIConfiguration{
				Name:     "My WebSub API",
				Version:  "v1.0",
				Upstream: stored,
			},
		},
		errStop: errStopAfterCapture,
	}
	svc := &WebSubAPIService{repo: repo}

	// A client echoing back a GET response: auth block present, value absent.
	req := &api.WebSubAPI{
		DisplayName: "My WebSub API",
		Version:     "v1.0",
		Upstream:    mapUpstreamModelToAPI(&stored),
	}

	_, err := svc.Update("org-1", "my-websub-api", "user-1", req)
	if err == nil {
		t.Fatal("expected the capturing repo to stop the update")
	}
	if repo.saved == nil {
		t.Fatal("Update never reached the repository")
	}

	savedAuth := repo.saved.Configuration.Upstream.Main.Auth
	if savedAuth == nil || savedAuth.Value != storedCredential {
		t.Errorf("update wiped the stored credential: got %v, want %q", savedAuth, storedCredential)
	}

	// Sandbox carries its own credential and goes through the same merge, so
	// assert it at the write boundary too rather than trusting main to cover it.
	savedSandbox := repo.saved.Configuration.Upstream.Sandbox
	if savedSandbox == nil || savedSandbox.Auth == nil {
		t.Fatal("sandbox auth block dropped on update")
	}
	if wantSandbox := storedCredential + "-sandbox"; savedSandbox.Auth.Value != wantSandbox {
		t.Errorf("update wiped the stored sandbox credential: got %q, want %q", savedSandbox.Auth.Value, wantSandbox)
	}
}

// nil and "" both mean "unchanged" — a client may round-trip either the
// omitted field or an explicit empty string, and neither drops the credential.
func TestPreserveWebSubUpstreamAuth_NilAndEmptyValueAreEquivalent(t *testing.T) {
	withNil := preserveWebSubUpstreamAuth(storedUpstreamConfig(), api.Upstream{
		Main: api.UpstreamDefinition{Auth: &api.UpstreamAuth{Type: authTypePtr(api.ApiKey)}},
	})
	// Not utils.StringPtrIfNotEmpty(""), which returns nil and would make this
	// the same input as withNil rather than the other of the two.
	emptyValue := ""
	withEmpty := preserveWebSubUpstreamAuth(storedUpstreamConfig(), api.Upstream{
		Main: api.UpstreamDefinition{Auth: &api.UpstreamAuth{
			Type:  authTypePtr(api.ApiKey),
			Value: &emptyValue,
		}},
	})

	for name, got := range map[string]api.Upstream{"nil value": withNil, "empty value": withEmpty} {
		if got.Main.Auth.Value == nil || *got.Main.Auth.Value != storedCredential {
			t.Errorf("%s did not preserve the stored credential: got %v", name, got.Main.Auth.Value)
		}
	}
}

// Clearing a credential is explicit: switch the auth type (the contract
// provides "none" for exactly this). Absence of the value field never clears.
func TestPreserveWebSubUpstreamAuth_TypeNoneClearsTheCredential(t *testing.T) {
	updated := preserveWebSubUpstreamAuth(storedUpstreamConfig(), api.Upstream{
		Main: api.UpstreamDefinition{Auth: &api.UpstreamAuth{Type: authTypePtr(api.None)}},
	})

	saved := mapUpstreamAPIToModel(updated)
	if saved.Main.Auth.Value != "" {
		t.Errorf("auth type none must drop the stored credential: got %q", saved.Main.Auth.Value)
	}
	if saved.Main.Auth.Type != string(api.None) {
		t.Errorf("auth type not applied: got %q", saved.Main.Auth.Type)
	}
}

// An omitted auth type is "leave the scheme as it is", the same way an omitted
// value is. Preserving the credential while blanking its type would store a
// secret that belongs to no scheme.
func TestPreserveWebSubUpstreamAuth_OmittedTypeKeepsStoredScheme(t *testing.T) {
	updated := preserveWebSubUpstreamAuth(storedUpstreamConfig(), api.Upstream{
		Main: api.UpstreamDefinition{Auth: &api.UpstreamAuth{
			Header: utils.StringPtrIfNotEmpty("X-API-Key"),
		}},
	})

	saved := mapUpstreamAPIToModel(updated)
	if saved.Main.Auth.Type != string(api.ApiKey) {
		t.Errorf("stored auth type not kept: got %q, want %q", saved.Main.Auth.Type, api.ApiKey)
	}
	if saved.Main.Auth.Value != storedCredential {
		t.Errorf("stored credential not kept: got %q", saved.Main.Auth.Value)
	}
}

// The scheme is restored even when there is no stored credential to preserve:
// the type and the value are independent parts of the stored configuration.
func TestPreserveWebSubUpstreamAuth_OmittedTypeKeepsStoredSchemeWithoutCredential(t *testing.T) {
	stored := model.UpstreamConfig{
		Main: &model.UpstreamEndpoint{
			URL:  "https://backend.example.com/events",
			Auth: &model.UpstreamAuth{Type: "bearer", Header: "Authorization"},
		},
	}

	updated := preserveWebSubUpstreamAuth(stored, api.Upstream{
		Main: api.UpstreamDefinition{Auth: &api.UpstreamAuth{
			Header: utils.StringPtrIfNotEmpty("Authorization"),
		}},
	})

	saved := mapUpstreamAPIToModel(updated)
	if saved.Main.Auth.Type != string(api.Bearer) {
		t.Errorf("stored auth type not kept: got %q, want %q", saved.Main.Auth.Type, api.Bearer)
	}
}

// The request belongs to the caller. Restoring a credential must not write it
// back onto the request's own auth blocks, where a later read of req would see
// a secret the client never sent.
func TestPreserveWebSubUpstreamAuth_DoesNotMutateTheRequest(t *testing.T) {
	mainAuth := &api.UpstreamAuth{Type: authTypePtr(api.ApiKey)}
	sandboxAuth := &api.UpstreamAuth{Type: authTypePtr(api.Bearer)}
	req := api.Upstream{
		Main:    api.UpstreamDefinition{Auth: mainAuth},
		Sandbox: &api.UpstreamDefinition{Auth: sandboxAuth},
	}

	got := preserveWebSubUpstreamAuth(storedUpstreamConfig(), req)

	if got.Main.Auth.Value == nil || *got.Main.Auth.Value != storedCredential {
		t.Fatalf("the merge did not restore the credential: got %v", got.Main.Auth.Value)
	}
	if mainAuth.Value != nil {
		t.Errorf("request main auth was mutated: got %q", *mainAuth.Value)
	}
	if sandboxAuth.Value != nil {
		t.Errorf("request sandbox auth was mutated: got %q", *sandboxAuth.Value)
	}
	if req.Sandbox.Auth != sandboxAuth {
		t.Error("request sandbox definition was mutated")
	}
}

// A stored auth block with no credential is still configuration: the scheme
// and the header name. An update that omits the block keeps it rather than
// dropping it, so a deliberate "auth.type: none" survives the next save.
func TestPreserveWebSubUpstreamAuth_OmittedBlockKeepsSchemeWithoutCredential(t *testing.T) {
	stored := model.UpstreamConfig{
		Main: &model.UpstreamEndpoint{
			URL:  "https://backend.example.com/events",
			Auth: &model.UpstreamAuth{Type: string(api.None)},
		},
	}

	updated := preserveWebSubUpstreamAuth(stored, api.Upstream{
		Main: api.UpstreamDefinition{Url: utils.StringPtrIfNotEmpty("https://backend.example.com/events")},
	})

	saved := mapUpstreamAPIToModel(updated)
	if saved.Main.Auth == nil {
		t.Fatal("stored auth block dropped by an update that did not mention it")
	}
	if saved.Main.Auth.Type != string(api.None) {
		t.Errorf("stored auth type not kept: got %q, want %q", saved.Main.Auth.Type, api.None)
	}
	if saved.Main.Auth.Value != "" {
		t.Errorf("credential invented where none was stored: got %q", saved.Main.Auth.Value)
	}
}

// A credential-only update keeps the header of the unchanged scheme. Without
// this the header name is blanked and an api-key credential is sent nowhere.
func TestPreserveWebSubUpstreamAuth_OmittedHeaderKeepsStoredHeader(t *testing.T) {
	updated := preserveWebSubUpstreamAuth(storedUpstreamConfig(), api.Upstream{
		Main: api.UpstreamDefinition{Auth: &api.UpstreamAuth{
			Value: utils.StringPtrIfNotEmpty("rotated-key"),
		}},
	})

	saved := mapUpstreamAPIToModel(updated)
	if saved.Main.Auth.Header != "X-API-Key" {
		t.Errorf("stored auth header not kept: got %q, want %q", saved.Main.Auth.Header, "X-API-Key")
	}
	if saved.Main.Auth.Value != "rotated-key" {
		t.Errorf("new credential not applied: got %q", saved.Main.Auth.Value)
	}
}

// A header the request supplies wins over the stored one.
func TestPreserveWebSubUpstreamAuth_SuppliedHeaderWins(t *testing.T) {
	updated := preserveWebSubUpstreamAuth(storedUpstreamConfig(), api.Upstream{
		Main: api.UpstreamDefinition{Auth: &api.UpstreamAuth{
			Header: utils.StringPtrIfNotEmpty("X-Other-Key"),
		}},
	})

	saved := mapUpstreamAPIToModel(updated)
	if saved.Main.Auth.Header != "X-Other-Key" {
		t.Errorf("request header not applied: got %q", saved.Main.Auth.Header)
	}
	if saved.Main.Auth.Value != storedCredential {
		t.Errorf("credential not preserved alongside a header change: got %q", saved.Main.Auth.Value)
	}
}
