package service

import (
	"errors"
	"testing"

	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
)

type mockLLMTemplateSeederRepo struct {
	repository.LLMProviderTemplateRepository

	byOrgID       map[string]map[string]*model.LLMProviderTemplate
	createdCount  int
	updatedCount  int
	existsByOrgID map[string]map[string]bool
}

func newMockLLMTemplateSeederRepo() *mockLLMTemplateSeederRepo {
	return &mockLLMTemplateSeederRepo{
		byOrgID:       make(map[string]map[string]*model.LLMProviderTemplate),
		existsByOrgID: make(map[string]map[string]bool),
	}
}

func (m *mockLLMTemplateSeederRepo) Count(orgUUID string) (int, error) {
	return len(m.byOrgID[orgUUID]), nil
}

func (m *mockLLMTemplateSeederRepo) List(orgUUID string, limit, offset int) ([]*model.LLMProviderTemplate, error) {
	orgTemplates := m.byOrgID[orgUUID]
	res := make([]*model.LLMProviderTemplate, 0, len(orgTemplates))
	for _, tpl := range orgTemplates {
		res = append(res, cloneTemplate(tpl))
	}
	return res, nil
}

func (m *mockLLMTemplateSeederRepo) Create(tpl *model.LLMProviderTemplate) error {
	if tpl == nil {
		return errors.New("template is nil")
	}
	if m.byOrgID[tpl.OrganizationUUID] == nil {
		m.byOrgID[tpl.OrganizationUUID] = make(map[string]*model.LLMProviderTemplate)
	}
	if m.existsByOrgID[tpl.OrganizationUUID] == nil {
		m.existsByOrgID[tpl.OrganizationUUID] = make(map[string]bool)
	}
	m.byOrgID[tpl.OrganizationUUID][tpl.ID] = cloneTemplate(tpl)
	m.existsByOrgID[tpl.OrganizationUUID][tpl.ID] = true
	m.createdCount++
	return nil
}

func (m *mockLLMTemplateSeederRepo) Update(tpl *model.LLMProviderTemplate) error {
	if tpl == nil {
		return errors.New("template is nil")
	}
	if m.byOrgID[tpl.OrganizationUUID] == nil {
		return errors.New("organization not found")
	}
	m.byOrgID[tpl.OrganizationUUID][tpl.ID] = cloneTemplate(tpl)
	m.updatedCount++
	return nil
}

func (m *mockLLMTemplateSeederRepo) Exists(templateID, orgUUID string) (bool, error) {
	if m.byOrgID[orgUUID] == nil {
		return false, nil
	}
	if m.existsByOrgID[orgUUID] != nil {
		if forced, ok := m.existsByOrgID[orgUUID][templateID]; ok {
			return forced, nil
		}
	}
	_, ok := m.byOrgID[orgUUID][templateID]
	return ok, nil
}

func cloneTemplate(in *model.LLMProviderTemplate) *model.LLMProviderTemplate {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func TestSeedForOrg_UpdatesExistingTemplateWhenSeedDefinitionChanges(t *testing.T) {
	orgID := "org-1"
	repo := newMockLLMTemplateSeederRepo()
	repo.byOrgID[orgID] = map[string]*model.LLMProviderTemplate{
		"openai": {
			OrganizationUUID: orgID,
			ID:               "openai",
			Name:             "OpenAI Old",
			Description:      "user description",
			Metadata: &model.LLMProviderTemplateMetadata{
				OpenapiSpecURL: "https://old.example/openapi.yaml",
			},
			PromptTokens: &model.ExtractionIdentifier{Location: "body", Identifier: "usage.old_prompt"},
		},
	}

	seed := []*model.LLMProviderTemplate{
		{
			ID:   "openai",
			Name: "OpenAI",
			Metadata: &model.LLMProviderTemplateMetadata{
				OpenapiSpecURL: "https://new.example/openapi.yaml",
			},
			PromptTokens: &model.ExtractionIdentifier{Location: "body", Identifier: "usage.prompt_tokens"},
		},
	}

	seeder := NewLLMTemplateSeeder(repo, seed)
	if err := seeder.SeedForOrg(orgID); err != nil {
		t.Fatalf("SeedForOrg returned error: %v", err)
	}

	updated := repo.byOrgID[orgID]["openai"]
	if updated == nil {
		t.Fatalf("expected updated template to exist")
	}
	if updated.Name != "OpenAI" {
		t.Fatalf("expected name to be synced, got %q", updated.Name)
	}
	if updated.Metadata == nil || updated.Metadata.OpenapiSpecURL != "https://new.example/openapi.yaml" {
		t.Fatalf("expected metadata.openapiSpecUrl to be synced")
	}
	if updated.PromptTokens == nil || updated.PromptTokens.Identifier != "usage.prompt_tokens" {
		t.Fatalf("expected prompt token extraction identifier to be synced")
	}
	if updated.Description != "user description" {
		t.Fatalf("expected description to remain unchanged, got %q", updated.Description)
	}
	if repo.updatedCount != 1 {
		t.Fatalf("expected one update call, got %d", repo.updatedCount)
	}
}

func TestSeedForOrg_DoesNotUpdateWhenTemplateIsAlreadyInSync(t *testing.T) {
	orgID := "org-1"
	repo := newMockLLMTemplateSeederRepo()
	repo.byOrgID[orgID] = map[string]*model.LLMProviderTemplate{
		"openai": {
			OrganizationUUID: orgID,
			ID:               "openai",
			Name:             "OpenAI",
			Metadata: &model.LLMProviderTemplateMetadata{
				OpenapiSpecURL: "https://same.example/openapi.yaml",
			},
			PromptTokens: &model.ExtractionIdentifier{Location: "body", Identifier: "usage.prompt_tokens"},
		},
	}

	seed := []*model.LLMProviderTemplate{
		{
			ID:   "openai",
			Name: "OpenAI",
			Metadata: &model.LLMProviderTemplateMetadata{
				OpenapiSpecURL: "https://same.example/openapi.yaml",
			},
			PromptTokens: &model.ExtractionIdentifier{Location: "body", Identifier: "usage.prompt_tokens"},
		},
	}

	seeder := NewLLMTemplateSeeder(repo, seed)
	if err := seeder.SeedForOrg(orgID); err != nil {
		t.Fatalf("SeedForOrg returned error: %v", err)
	}

	if repo.updatedCount != 0 {
		t.Fatalf("expected no update call, got %d", repo.updatedCount)
	}
}

func TestSeedForOrg_DoesNotCreateTwiceForDuplicateSeedIDs(t *testing.T) {
	orgID := "org-duplicate"
	repo := newMockLLMTemplateSeederRepo()

	seed := []*model.LLMProviderTemplate{
		{
			ID:   "openai",
			Name: "OpenAI",
			Metadata: &model.LLMProviderTemplateMetadata{
				OpenapiSpecURL: "https://example/openapi.yaml",
			},
		},
		{
			ID:   "openai",
			Name: "OpenAI",
			Metadata: &model.LLMProviderTemplateMetadata{
				OpenapiSpecURL: "https://example/openapi.yaml",
			},
		},
	}

	seeder := NewLLMTemplateSeeder(repo, seed)
	if err := seeder.SeedForOrg(orgID); err != nil {
		t.Fatalf("SeedForOrg returned error: %v", err)
	}

	if repo.createdCount != 1 {
		t.Fatalf("expected one create call for duplicate seed IDs, got %d", repo.createdCount)
	}
	if repo.updatedCount != 0 {
		t.Fatalf("expected no update call, got %d", repo.updatedCount)
	}
}
