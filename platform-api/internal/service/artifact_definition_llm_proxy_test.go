/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com) All Rights Reserved.
 *
 *  WSO2 LLC. licenses this file to you under the Apache License,
 *  Version 2.0 (the "License"); you may not use this file except
 *  in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing,
 *  software distributed under the License is distributed on an
 *  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 *  KIND, either express or implied. See the License for the
 *  specific language governing permissions and limitations
 *  under the License.
 */

package service

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

const (
	proxyDefTestOrgID     = "org-proxy-def"
	proxyDefTestProjectID = "proj-proxy-def"
)

// proxyDefTestDeps holds the repositories a proxy artifact is rendered from.
type proxyDefTestDeps struct {
	definition   ArtifactDefinition
	providerRepo repository.LLMProviderRepository
	templateRepo repository.LLMProviderTemplateRepository
	proxyRepo    repository.LLMProxyRepository
}

func setupProxyDefinitionTest(t *testing.T) *proxyDefTestDeps {
	t.Helper()

	sqlDB, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if _, err := sqlDB.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable fks: %v", err)
	}
	db := &database.DB{DB: sqlDB}
	t.Cleanup(func() { db.Close() })

	schema, err := os.ReadFile(filepath.Join("..", "database", "schema.sqlite.sql"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if _, err := db.Exec(string(schema)); err != nil {
		t.Fatalf("apply schema: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'idp-ref', datetime('now'), datetime('now'))`,
		proxyDefTestOrgID, "h-"+proxyDefTestOrgID, "Proxy Def Org", "default"); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO projects (uuid, handle, display_name, organization_uuid, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		proxyDefTestProjectID, "default", "Default", proxyDefTestOrgID, ""); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	providerRepo := repository.NewLLMProviderRepo(db)
	templateRepo := repository.NewLLMProviderTemplateRepo(db)
	proxyRepo := repository.NewLLMProxyRepo(db)

	return &proxyDefTestDeps{
		definition:   NewLLMProxyDefinition(proxyRepo, providerRepo, templateRepo),
		providerRepo: providerRepo,
		templateRepo: templateRepo,
		proxyRepo:    proxyRepo,
	}
}

// seedProvider creates a template and a provider referencing it, exactly as the
// control-plane create path does: the template is recorded by uuid on the
// provider row, and never written into the provider's configuration blob.
func (d *proxyDefTestDeps) seedProvider(t *testing.T, handle, templateHandle string) {
	t.Helper()

	tmpl, err := d.templateRepo.GetByID(templateHandle, proxyDefTestOrgID)
	if err != nil {
		t.Fatalf("look up template %q: %v", templateHandle, err)
	}
	if tmpl == nil {
		tmpl = &model.LLMProviderTemplate{
			OrganizationUUID: proxyDefTestOrgID,
			ID:               templateHandle,
			Name:             templateHandle,
			Origin:           constants.OriginCP,
		}
		if err := d.templateRepo.Create(tmpl); err != nil {
			t.Fatalf("seed template %q: %v", templateHandle, err)
		}
	}

	if err := d.providerRepo.Create(&model.LLMProvider{
		OrganizationUUID: proxyDefTestOrgID,
		ID:               handle,
		Name:             handle,
		Version:          "v1.0",
		TemplateUUID:     tmpl.UUID,
		Origin:           constants.OriginCP,
	}); err != nil {
		t.Fatalf("seed provider %q: %v", handle, err)
	}
}

func (d *proxyDefTestDeps) seedProxy(t *testing.T, handle string, cfg model.LLMProxyConfig) {
	t.Helper()

	primary, err := d.providerRepo.GetByID(model.PrimaryLLMProxyProviderID(cfg), proxyDefTestOrgID)
	if err != nil || primary == nil {
		t.Fatalf("resolve primary provider for proxy %q: (%v, %v)", handle, primary, err)
	}
	if err := d.proxyRepo.Create(&model.LLMProxy{
		OrganizationUUID: proxyDefTestOrgID,
		ID:               handle,
		Name:             handle,
		ProjectUUID:      proxyDefTestProjectID,
		Version:          "v1.0",
		ProviderUUID:     primary.UUID,
		Configuration:    cfg,
		Origin:           constants.OriginCP,
	}); err != nil {
		t.Fatalf("seed proxy %q: %v", handle, err)
	}
}

func (d *proxyDefTestDeps) render(t *testing.T, handle string) dto.LLMProxyDeploymentSpec {
	t.Helper()

	snapshot, err := d.definition.Current(&model.Artifact{
		Handle:           handle,
		OrganizationUUID: proxyDefTestOrgID,
		Kind:             constants.LLMProxy,
	})
	if err != nil {
		t.Fatalf("render proxy %q: %v", handle, err)
	}
	definition, ok := snapshot.Definition.(*dto.LLMProxyDeploymentYAML)
	if !ok {
		t.Fatalf("definition type = %T, want *dto.LLMProxyDeploymentYAML", snapshot.Definition)
	}
	return definition.Spec
}

// A proxy whose inbound interface merely restates its primary provider's own
// format is expressible as the `provider` / `additionalProviders` pair, so it
// must render as that pair — a gateway released before the canonical list reads
// only that shape, and drops `providers` silently, leaving it with no provider
// at all. The template backing the comparison lives on the provider's
// templateUuid; reading the configuration's template field instead leaves it
// empty for every provider created through the API, which turns every such
// proxy into a canonical-only artifact.
func TestLLMProxyDefinition_RedundantInboundInterfaceRendersLegacyPair(t *testing.T) {
	d := setupProxyDefinitionTest(t)
	d.seedProvider(t, "p-openai", "openai")
	d.seedProvider(t, "p-anthropic", "anthropic")

	d.seedProxy(t, "redundant-interface", model.LLMProxyConfig{
		Providers: []model.LLMProxyAttachment{
			{ID: "p-openai", IsPrimary: true},
			{ID: "p-anthropic"},
		},
		InboundTemplate: "openai",
	})

	spec := d.render(t, "redundant-interface")

	if spec.Providers != nil {
		t.Errorf("spec.providers = %+v; a redundant inbound interface must not force the canonical list", spec.Providers)
	}
	if spec.Provider == nil {
		t.Fatal("spec.provider is nil; the legacy pair must carry the primary")
	}
	if spec.Provider.ID != "p-openai" {
		t.Errorf("spec.provider.id = %q, want %q", spec.Provider.ID, "p-openai")
	}
	if len(spec.AdditionalProviders) != 1 || spec.AdditionalProviders[0].ID != "p-anthropic" {
		t.Errorf("spec.additionalProviders = %+v, want the one non-primary attachment", spec.AdditionalProviders)
	}
	if spec.InboundTemplate != "" {
		t.Errorf("spec.inboundTemplate = %q; a redundant interface is omitted, not restated", spec.InboundTemplate)
	}
}

// An interface that differs from the primary's format is beyond the pair, which
// has no slot for one, so the artifact keeps the canonical list and fails loudly
// on a gateway too old to read it.
func TestLLMProxyDefinition_DifferingInboundInterfaceRendersCanonicalList(t *testing.T) {
	d := setupProxyDefinitionTest(t)
	d.seedProvider(t, "p-openai", "openai")
	d.seedProvider(t, "p-anthropic", "anthropic")

	d.seedProxy(t, "differing-interface", model.LLMProxyConfig{
		Providers: []model.LLMProxyAttachment{
			{ID: "p-openai", IsPrimary: true},
			{ID: "p-anthropic"},
		},
		InboundTemplate: "anthropic",
	})

	spec := d.render(t, "differing-interface")

	if spec.Provider != nil || spec.AdditionalProviders != nil {
		t.Errorf("legacy pair emitted (%+v / %+v); an interface the pair cannot hold must use the canonical list",
			spec.Provider, spec.AdditionalProviders)
	}
	if len(spec.Providers) != 2 {
		t.Fatalf("spec.providers = %+v, want both attachments", spec.Providers)
	}
	if spec.InboundTemplate != "anthropic" {
		t.Errorf("spec.inboundTemplate = %q, want %q", spec.InboundTemplate, "anthropic")
	}
}
