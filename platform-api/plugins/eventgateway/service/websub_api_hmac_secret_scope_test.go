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
	"log/slog"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

const (
	owningOrg  = "org-owner"
	foreignOrg = "org-other"
	ownedAPI   = "art-websub-owned"
)

// fakeWebSubRepo resolves exactly one API, and only under owningOrg — the same
// shape as the real repo, whose every getter takes the org in its WHERE clause.
type fakeWebSubRepo struct {
	repository.WebSubAPIRepository
	gets int
}

func (f *fakeWebSubRepo) GetByUUID(uuid, orgUUID string) (*model.WebSubAPI, error) {
	f.gets++
	if uuid == ownedAPI && orgUUID == owningOrg {
		return &model.WebSubAPI{UUID: ownedAPI}, nil
	}
	return nil, nil
}

// fakeHmacRepo counts reads so a test can assert no row was touched.
type fakeHmacRepo struct {
	repository.WebSubAPIHmacSecretRepository
	lists int
}

func (f *fakeHmacRepo) ListByArtifact(artifactUUID string) ([]*model.WebSubAPIHmacSecret, error) {
	f.lists++
	return []*model.WebSubAPIHmacSecret{{ArtifactUUID: artifactUUID, Handle: "github-hook"}}, nil
}

func newScopeTestService(websub *fakeWebSubRepo, hmac *fakeHmacRepo) *WebSubAPIHmacSecretService {
	return &WebSubAPIHmacSecretService{
		repo:       hmac,
		websubRepo: websub,
		slogger:    slog.Default(),
	}
}

func TestListByArtifactUUID_ReturnsSecretsForOwningOrg(t *testing.T) {
	websub, hmac := &fakeWebSubRepo{}, &fakeHmacRepo{}
	svc := newScopeTestService(websub, hmac)

	secrets, err := svc.ListByArtifactUUID(ownedAPI, owningOrg)
	if err != nil {
		t.Fatalf("ListByArtifactUUID: %v", err)
	}
	if len(secrets) != 1 || secrets[0].Handle != "github-hook" {
		t.Fatalf("secrets = %+v, want the one stored secret", secrets)
	}
}

// The artifact lookup is what scopes the result: websub_api_hmac_secrets has no
// organization column, so an API that does not resolve under the caller's org
// must stop the call before any secret row is read.
func TestListByArtifactUUID_ForeignOrgReadsNoSecret(t *testing.T) {
	websub, hmac := &fakeWebSubRepo{}, &fakeHmacRepo{}
	svc := newScopeTestService(websub, hmac)

	secrets, err := svc.ListByArtifactUUID(ownedAPI, foreignOrg)
	if !apperror.WebSubAPINotFound.Is(err) {
		t.Fatalf("err = %v, want WebSubAPINotFound", err)
	}
	if secrets != nil {
		t.Errorf("secrets = %+v, want nil", secrets)
	}
	if hmac.lists != 0 {
		t.Errorf("secret repo read %d times, want 0", hmac.lists)
	}
}

func TestListByArtifactUUID_UnknownArtifactReadsNoSecret(t *testing.T) {
	websub, hmac := &fakeWebSubRepo{}, &fakeHmacRepo{}
	svc := newScopeTestService(websub, hmac)

	if _, err := svc.ListByArtifactUUID("art-does-not-exist", owningOrg); !apperror.WebSubAPINotFound.Is(err) {
		t.Fatalf("err = %v, want WebSubAPINotFound", err)
	}
	if hmac.lists != 0 {
		t.Errorf("secret repo read %d times, want 0", hmac.lists)
	}
}
