/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
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

package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"platform-api/src/internal/constants"
	"platform-api/src/internal/database"
	"platform-api/src/internal/model"
	"platform-api/src/internal/utils"
)

// ---- LLM Provider Templates ----

type LLMProviderTemplateRepo struct {
	db *database.DB
}

type llmProviderTemplateConfig struct {
	Metadata         *model.LLMProviderTemplateMetadata `json:"metadata,omitempty"`
	PromptTokens     *model.ExtractionIdentifier        `json:"promptTokens,omitempty"`
	CompletionTokens *model.ExtractionIdentifier        `json:"completionTokens,omitempty"`
	TotalTokens      *model.ExtractionIdentifier        `json:"totalTokens,omitempty"`
	RemainingTokens  *model.ExtractionIdentifier        `json:"remainingTokens,omitempty"`
	RequestModel     *model.ExtractionIdentifier        `json:"requestModel,omitempty"`
	ResponseModel    *model.ExtractionIdentifier        `json:"responseModel,omitempty"`
	ResourceMappings *model.LLMProviderTemplateResourceMappings `json:"resourceMappings,omitempty"`
}

func NewLLMProviderTemplateRepo(db *database.DB) LLMProviderTemplateRepository {
	return &LLMProviderTemplateRepo{db: db}
}

func (r *LLMProviderTemplateRepo) Create(t *model.LLMProviderTemplate) error {
	uuidStr, err := utils.GenerateUUID()
	if err != nil {
		return fmt.Errorf("failed to generate LLM provider template ID: %w", err)
	}
	t.UUID = uuidStr
	t.CreatedAt = time.Now()
	t.UpdatedAt = time.Now()
	if t.GroupID == "" {
		t.GroupID = t.ID
	}

	configJSON, err := json.Marshal(&llmProviderTemplateConfig{
		Metadata:         t.Metadata,
		PromptTokens:     t.PromptTokens,
		CompletionTokens: t.CompletionTokens,
		TotalTokens:      t.TotalTokens,
		RemainingTokens:  t.RemainingTokens,
		RequestModel:     t.RequestModel,
		ResponseModel:    t.ResponseModel,
		ResourceMappings: t.ResourceMappings,
	})
	if err != nil {
		return err
	}

	query := `
		INSERT INTO llm_provider_templates (
			uuid, organization_uuid, handle, group_id, name, description, managed_by, created_by,
			configuration, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err = r.db.Exec(r.db.Rebind(query),
		t.UUID, t.OrganizationUUID, t.ID, t.GroupID, t.Name, t.Description, t.ManagedBy, t.CreatedBy,
		string(configJSON),
		t.CreatedAt, t.UpdatedAt,
	)
	return err
}

func (r *LLMProviderTemplateRepo) GetByID(templateID, orgUUID string) (*model.LLMProviderTemplate, error) {
	row := r.db.QueryRow(r.db.Rebind(`
		SELECT uuid, organization_uuid, handle, group_id, name, description, managed_by, created_by, configuration, created_at, updated_at
		FROM llm_provider_templates
		WHERE handle = ? AND organization_uuid = ?
	`), templateID, orgUUID)

	var t model.LLMProviderTemplate
	var createdBy sql.NullString
	var configJSON []byte
	if err := row.Scan(
		&t.UUID, &t.OrganizationUUID, &t.ID, &t.GroupID, &t.Name, &t.Description, &t.ManagedBy, &createdBy, &configJSON,
		&t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	t.CreatedBy = createdBy.String

	if len(configJSON) > 0 {
		var cfg llmProviderTemplateConfig
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return nil, err
		}
		t.Metadata = cfg.Metadata
		t.PromptTokens = cfg.PromptTokens
		t.CompletionTokens = cfg.CompletionTokens
		t.TotalTokens = cfg.TotalTokens
		t.RemainingTokens = cfg.RemainingTokens
		t.RequestModel = cfg.RequestModel
		t.ResponseModel = cfg.ResponseModel
		t.ResourceMappings = cfg.ResourceMappings
	}

	return &t, nil
}

func (r *LLMProviderTemplateRepo) GetByUUID(uuid, orgUUID string) (*model.LLMProviderTemplate, error) {
	row := r.db.QueryRow(r.db.Rebind(`
		SELECT uuid, organization_uuid, handle, group_id, name, description, managed_by, created_by, configuration, created_at, updated_at
		FROM llm_provider_templates
		WHERE uuid = ? AND organization_uuid = ?
	`), uuid, orgUUID)

	var t model.LLMProviderTemplate
	var createdBy sql.NullString
	var configJSON []byte
	if err := row.Scan(
		&t.UUID, &t.OrganizationUUID, &t.ID, &t.GroupID, &t.Name, &t.Description, &t.ManagedBy, &createdBy, &configJSON,
		&t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	t.CreatedBy = createdBy.String

	if len(configJSON) > 0 {
		var cfg llmProviderTemplateConfig
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return nil, err
		}
		t.Metadata = cfg.Metadata
		t.PromptTokens = cfg.PromptTokens
		t.CompletionTokens = cfg.CompletionTokens
		t.TotalTokens = cfg.TotalTokens
		t.RemainingTokens = cfg.RemainingTokens
		t.RequestModel = cfg.RequestModel
		t.ResponseModel = cfg.ResponseModel
		t.ResourceMappings = cfg.ResourceMappings
	}

	return &t, nil
}

func (r *LLMProviderTemplateRepo) List(orgUUID string, limit, offset int) ([]*model.LLMProviderTemplate, error) {
	pageClause, pageArgs := r.db.PaginationClause(limit, offset)
	query := `
		SELECT uuid, organization_uuid, handle, group_id, name, description, managed_by, created_by, configuration, created_at, updated_at
		FROM llm_provider_templates
		WHERE organization_uuid = ?
		ORDER BY created_at DESC
		` + pageClause
	rows, err := r.db.Query(r.db.Rebind(query), append([]any{orgUUID}, pageArgs...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*model.LLMProviderTemplate
	for rows.Next() {
		var t model.LLMProviderTemplate
		var createdBy sql.NullString
	var configJSON []byte
		err := rows.Scan(
			&t.UUID, &t.OrganizationUUID, &t.ID, &t.GroupID, &t.Name, &t.Description, &t.ManagedBy, &createdBy, &configJSON,
			&t.CreatedAt, &t.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		t.CreatedBy = createdBy.String
		if len(configJSON) > 0 {
			var cfg llmProviderTemplateConfig
			if err := json.Unmarshal(configJSON, &cfg); err != nil {
				return nil, err
			}
			t.Metadata = cfg.Metadata
			t.PromptTokens = cfg.PromptTokens
			t.CompletionTokens = cfg.CompletionTokens
			t.TotalTokens = cfg.TotalTokens
			t.RemainingTokens = cfg.RemainingTokens
			t.RequestModel = cfg.RequestModel
			t.ResponseModel = cfg.ResponseModel
			t.ResourceMappings = cfg.ResourceMappings
		}
		res = append(res, &t)
	}
	return res, rows.Err()
}

func (r *LLMProviderTemplateRepo) Update(t *model.LLMProviderTemplate) error {
	t.UpdatedAt = time.Now()

	configJSON, err := json.Marshal(&llmProviderTemplateConfig{
		Metadata:         t.Metadata,
		PromptTokens:     t.PromptTokens,
		CompletionTokens: t.CompletionTokens,
		TotalTokens:      t.TotalTokens,
		RemainingTokens:  t.RemainingTokens,
		RequestModel:     t.RequestModel,
		ResponseModel:    t.ResponseModel,
		ResourceMappings: t.ResourceMappings,
	})
	if err != nil {
		return err
	}

	query := `
		UPDATE llm_provider_templates
		SET name = ?, description = ?, configuration = ?, updated_by = ?, updated_at = ?
		WHERE handle = ? AND organization_uuid = ?
	`
	result, err := r.db.Exec(r.db.Rebind(query),
		t.Name, t.Description,
		string(configJSON),
		t.UpdatedBy, t.UpdatedAt,
		t.ID, t.OrganizationUUID,
	)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *LLMProviderTemplateRepo) Delete(templateID, orgUUID string) error {
	result, err := r.db.Exec(r.db.Rebind(`DELETE FROM llm_provider_templates WHERE handle = ? AND organization_uuid = ?`), templateID, orgUUID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *LLMProviderTemplateRepo) Exists(templateID, orgUUID string) (bool, error) {
	var count int
	err := r.db.QueryRow(r.db.Rebind(`SELECT COUNT(*) FROM llm_provider_templates WHERE handle = ? AND organization_uuid = ?`), templateID, orgUUID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *LLMProviderTemplateRepo) Count(orgUUID string) (int, error) {
	var count int
	if err := r.db.QueryRow(r.db.Rebind(`SELECT COUNT(*) FROM llm_provider_templates WHERE organization_uuid = ?`), orgUUID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// ---- LLM Providers ----

type LLMProviderRepo struct {
	db           *database.DB
	artifactRepo *ArtifactRepo
}

func NewLLMProviderRepo(db *database.DB) LLMProviderRepository {
	return &LLMProviderRepo{db: db, artifactRepo: NewArtifactRepo(db)}
}

func (r *LLMProviderRepo) Create(p *model.LLMProvider) error {
	uuidStr, err := utils.GenerateUUID()
	if err != nil {
		return fmt.Errorf("failed to generate LLM provider ID: %w", err)
	}
	p.UUID = uuidStr
	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now

	modelProvidersJSON, err := json.Marshal(p.ModelProviders)
	if err != nil {
		return err
	}

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert into artifacts table first using artifactRepo
	if err := r.artifactRepo.Create(tx, &model.Artifact{
		UUID:             p.UUID,
		Type:             constants.LLMProvider,
		OrganizationUUID: p.OrganizationUUID,
	}); err != nil {
		return fmt.Errorf("failed to create artifact: %w", err)
	}

	configurationJSON, err := serializeLLMProviderConfiguration(p.Configuration)
	if err != nil {
		return fmt.Errorf("failed to serialize configuration: %w", err)
	}

	// Insert into llm_providers table (handle/name/version/timestamps now live here)
	query := `
		INSERT INTO llm_providers (
			uuid, handle, name, version, description, created_by, template_uuid, openapi_spec, model_list, configuration, created_at, updated_at, organization_uuid
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = tx.Exec(r.db.Rebind(query),
		p.UUID, p.ID, p.Name, p.Version, p.Description, p.CreatedBy, p.TemplateUUID,
		[]byte(p.OpenAPISpec), modelProvidersJSON, configurationJSON, p.CreatedAt, p.UpdatedAt,
		p.OrganizationUUID,
	)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	// Persist gateway associations (if any) within the same transaction.
	assocQuery := `
		INSERT INTO gateway_association_mappings (
			artifact_uuid, organization_uuid, gateway_uuid, metadata, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?)`
	for _, assoc := range p.AssociatedGateways {
		var metadata sql.NullString
		if assoc.Metadata != "" {
			metadata = sql.NullString{String: assoc.Metadata, Valid: true}
		}
		if _, err := tx.Exec(r.db.Rebind(assocQuery),
			p.UUID, p.OrganizationUUID, assoc.GatewayUUID, metadata, now, now,
		); err != nil {
			return fmt.Errorf("failed to create gateway association: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (r *LLMProviderRepo) GetByID(providerID, orgUUID string) (*model.LLMProvider, error) {
	query := `
		SELECT
			uuid, handle, name, version, organization_uuid, created_at, updated_at,
			description, created_by, template_uuid, openapi_spec, model_list, configuration
		FROM llm_providers
		WHERE handle = ? AND organization_uuid = ?`
	row := r.db.QueryRow(r.db.Rebind(query), providerID, orgUUID)

	var p model.LLMProvider
	var createdBy sql.NullString
	var openAPISpec, modelProvidersRaw []byte
	var configurationJSON []byte
	if err := row.Scan(
		&p.UUID, &p.ID, &p.Name, &p.Version, &p.OrganizationUUID, &p.CreatedAt, &p.UpdatedAt,
		&p.Description, &createdBy, &p.TemplateUUID, &openAPISpec, &modelProvidersRaw, &configurationJSON,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	p.CreatedBy = createdBy.String

	if len(configurationJSON) > 0 {
		if config, err := deserializeLLMProviderConfiguration(configurationJSON); err != nil {
			return nil, fmt.Errorf("unmarshal configuration for provider %s: %w", p.ID, err)
		} else if config != nil {
			p.Configuration = *config
		}
	}

	if len(openAPISpec) > 0 {
		p.OpenAPISpec = string(openAPISpec)
	}
	if len(modelProvidersRaw) > 0 {
		if err := json.Unmarshal(modelProvidersRaw, &p.ModelProviders); err != nil {
			return nil, fmt.Errorf("unmarshal modelProviders for provider %s: %w", p.ID, err)
		}
	}

	associations, err := r.getAssociatedGateways(p.UUID, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to load gateway associations for provider %s: %w", p.ID, err)
	}
	p.AssociatedGateways = associations

	return &p, nil
}

// getAssociatedGateways returns the gateway associations for an artifact, joining
// the gateways table to resolve each gateway handle.
func (r *LLMProviderRepo) getAssociatedGateways(artifactUUID, orgUUID string) ([]model.AssociatedGatewayMapping, error) {
	query := `
		SELECT m.gateway_uuid, g.handle, m.metadata
		FROM gateway_association_mappings m
		JOIN gateways g ON g.uuid = m.gateway_uuid
		WHERE m.artifact_uuid = ? AND m.organization_uuid = ?
		ORDER BY m.created_at`
	rows, err := r.db.Query(r.db.Rebind(query), artifactUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var associations []model.AssociatedGatewayMapping
	for rows.Next() {
		var assoc model.AssociatedGatewayMapping
		var metadata sql.NullString
		if err := rows.Scan(&assoc.GatewayUUID, &assoc.GatewayHandle, &metadata); err != nil {
			return nil, err
		}
		if metadata.Valid {
			assoc.Metadata = metadata.String
		}
		associations = append(associations, assoc)
	}
	return associations, rows.Err()
}

func (r *LLMProviderRepo) List(orgUUID string, limit, offset int) ([]*model.LLMProvider, error) {
	pageClause, pageArgs := r.db.PaginationClause(limit, offset)
	args := append([]any{orgUUID}, pageArgs...)
	query := `
		SELECT
			uuid, handle, name, version, organization_uuid, created_at, updated_at,
			description, created_by, template_uuid, openapi_spec, model_list, configuration
		FROM llm_providers
		WHERE organization_uuid = ?
		ORDER BY created_at DESC
		` + pageClause
	rows, err := r.db.Query(r.db.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*model.LLMProvider
	for rows.Next() {
		var p model.LLMProvider
		var createdBy sql.NullString
		var openAPISpec, modelProvidersRaw []byte
		var configurationJSON []byte
		err := rows.Scan(
			&p.UUID, &p.ID, &p.Name, &p.Version, &p.OrganizationUUID, &p.CreatedAt, &p.UpdatedAt,
			&p.Description, &createdBy, &p.TemplateUUID, &openAPISpec, &modelProvidersRaw, &configurationJSON,
		)
		if err != nil {
			return nil, err
		}
		p.CreatedBy = createdBy.String
		if len(openAPISpec) > 0 {
			p.OpenAPISpec = string(openAPISpec)
		}
		if len(modelProvidersRaw) > 0 {
			if err := json.Unmarshal(modelProvidersRaw, &p.ModelProviders); err != nil {
				return nil, fmt.Errorf("unmarshal modelProviders for provider %s: %w", p.ID, err)
			}
		}
		if len(configurationJSON) > 0 {
			if config, err := deserializeLLMProviderConfiguration(configurationJSON); err != nil {
				return nil, fmt.Errorf("unmarshal configuration for provider %s: %w", p.ID, err)
			} else if config != nil {
				p.Configuration = *config
			}
		}
		res = append(res, &p)
	}
	return res, rows.Err()
}

func (r *LLMProviderRepo) Count(orgUUID string) (int, error) {
	return r.artifactRepo.CountByKindAndOrg(constants.LLMProvider, orgUUID)
}

func (r *LLMProviderRepo) Update(p *model.LLMProvider) error {
	now := time.Now()
	p.UpdatedAt = now

	modelProvidersJSON, err := json.Marshal(p.ModelProviders)
	if err != nil {
		return err
	}
	configurationJSON, err := serializeLLMProviderConfiguration(p.Configuration)
	if err != nil {
		return fmt.Errorf("failed to serialize configuration: %w", err)
	}

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get the provider UUID from handle
	var providerUUID string
	query := `
		SELECT uuid FROM llm_providers
		WHERE handle = ? AND organization_uuid = ?`
	err = tx.QueryRow(r.db.Rebind(query), p.ID, p.OrganizationUUID).Scan(&providerUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		return err
	}

	// Update llm_providers table (name/version/updated_at now live here)
	query = `
		UPDATE llm_providers
		SET name = ?, version = ?, description = ?, template_uuid = ?, openapi_spec = ?, model_list = ?, configuration = ?, updated_by = ?, updated_at = ?
		WHERE uuid = ?`
	result, err := tx.Exec(r.db.Rebind(query),
		p.Name, p.Version, p.Description, p.TemplateUUID, []byte(p.OpenAPISpec), modelProvidersJSON, configurationJSON, p.UpdatedBy, now,
		providerUUID,
	)
	if err != nil {
		return fmt.Errorf("failed to update provider: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (r *LLMProviderRepo) Delete(providerID, orgUUID string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get the provider UUID from handle
	var providerUUID string
	query := `
		SELECT uuid FROM llm_providers
		WHERE handle = ? AND organization_uuid = ?`
	err = tx.QueryRow(r.db.Rebind(query), providerID, orgUUID).Scan(&providerUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		return err
	}

	// Delete from llm_providers first, then artifacts
	_, err = tx.Exec(r.db.Rebind(`DELETE FROM llm_providers WHERE uuid = ?`), providerUUID)
	if err != nil {
		return err
	}

	if err := r.artifactRepo.Delete(tx, providerUUID); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (r *LLMProviderRepo) Exists(providerID, orgUUID string) (bool, error) {
	return r.artifactRepo.Exists(constants.LLMProvider, providerID, orgUUID)
}

// ---- LLM Proxies ----

type LLMProxyRepo struct {
	db           *database.DB
	artifactRepo *ArtifactRepo
}

func NewLLMProxyRepo(db *database.DB) LLMProxyRepository {
	return &LLMProxyRepo{db: db, artifactRepo: NewArtifactRepo(db)}
}

func (r *LLMProxyRepo) Create(p *model.LLMProxy) error {
	uuidStr, err := utils.GenerateUUID()
	if err != nil {
		return fmt.Errorf("failed to generate LLM proxy ID: %w", err)
	}
	p.UUID = uuidStr
	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now

	configurationJSON, err := serializeLLMProxyConfiguration(p.Configuration)
	if err != nil {
		return fmt.Errorf("failed to serialize configuration: %w", err)
	}

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert into artifacts table first using artifactRepo
	if err := r.artifactRepo.Create(tx, &model.Artifact{
		UUID:             p.UUID,
		Type:             constants.LLMProxy,
		OrganizationUUID: p.OrganizationUUID,
	}); err != nil {
		return fmt.Errorf("failed to create artifact: %w", err)
	}

	// Insert into llm_proxies table (handle/name/version/timestamps now live here)
	query := `
		INSERT INTO llm_proxies (
			uuid, handle, name, version, project_uuid, description, created_by, provider_uuid, openapi_spec, configuration, created_at, updated_at, organization_uuid
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = tx.Exec(r.db.Rebind(query),
		p.UUID, p.ID, p.Name, p.Version, p.ProjectUUID, p.Description, p.CreatedBy, p.ProviderUUID,
		[]byte(p.OpenAPISpec), configurationJSON, p.CreatedAt, p.UpdatedAt,
		p.OrganizationUUID,
	)
	if err != nil {
		return fmt.Errorf("failed to create proxy: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (r *LLMProxyRepo) GetByID(proxyID, orgUUID string) (*model.LLMProxy, error) {
	query := `
		SELECT
			uuid, handle, name, version, organization_uuid, created_at, updated_at,
			project_uuid, description, created_by, provider_uuid, openapi_spec, configuration
		FROM llm_proxies
		WHERE handle = ? AND organization_uuid = ?`
	row := r.db.QueryRow(r.db.Rebind(query), proxyID, orgUUID)

	var p model.LLMProxy
	var createdBy sql.NullString
	var openAPISpec, configurationJSON []byte
	if err := row.Scan(
		&p.UUID, &p.ID, &p.Name, &p.Version, &p.OrganizationUUID, &p.CreatedAt, &p.UpdatedAt,
		&p.ProjectUUID, &p.Description, &createdBy, &p.ProviderUUID,
		&openAPISpec, &configurationJSON,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	p.CreatedBy = createdBy.String

	if len(openAPISpec) > 0 {
		p.OpenAPISpec = string(openAPISpec)
	}
	if len(configurationJSON) > 0 {
		if config, err := deserializeLLMProxyConfiguration(configurationJSON); err != nil {
			return nil, fmt.Errorf("unmarshal configuration for proxy %s: %w", p.ID, err)
		} else if config != nil {
			p.Configuration = *config
		}
	}

	return &p, nil
}

func (r *LLMProxyRepo) List(orgUUID string, limit, offset int) ([]*model.LLMProxy, error) {
	pageClause, pageArgs := r.db.PaginationClause(limit, offset)
	args := append([]any{orgUUID}, pageArgs...)
	query := `
		SELECT
			uuid, handle, name, version, organization_uuid, created_at, updated_at,
			project_uuid, description, created_by, provider_uuid,
			openapi_spec, configuration
		FROM llm_proxies
		WHERE organization_uuid = ?
		ORDER BY created_at DESC
		` + pageClause
	rows, err := r.db.Query(r.db.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*model.LLMProxy
	for rows.Next() {
		var p model.LLMProxy
		var createdBy sql.NullString
		var openAPISpec, configurationJSON []byte
		err := rows.Scan(
			&p.UUID, &p.ID, &p.Name, &p.Version, &p.OrganizationUUID, &p.CreatedAt, &p.UpdatedAt,
			&p.ProjectUUID, &p.Description, &createdBy, &p.ProviderUUID,
			&openAPISpec, &configurationJSON,
		)
		if err != nil {
			return nil, err
		}
		p.CreatedBy = createdBy.String
		if len(openAPISpec) > 0 {
			p.OpenAPISpec = string(openAPISpec)
		}
		if len(configurationJSON) > 0 {
			if config, err := deserializeLLMProxyConfiguration(configurationJSON); err != nil {
				return nil, fmt.Errorf("unmarshal configuration for proxy %s: %w", p.ID, err)
			} else if config != nil {
				p.Configuration = *config
			}
		}
		res = append(res, &p)
	}
	return res, rows.Err()
}

func (r *LLMProxyRepo) ListByProject(orgUUID, projectUUID string, limit, offset int) ([]*model.LLMProxy, error) {
	pageClause, pageArgs := r.db.PaginationClause(limit, offset)
	args := append([]any{orgUUID, projectUUID}, pageArgs...)
	query := `
		SELECT
			uuid, handle, name, version, organization_uuid, created_at, updated_at,
			project_uuid, description, created_by, provider_uuid,
			openapi_spec, configuration
		FROM llm_proxies
		WHERE organization_uuid = ? AND project_uuid = ?
		ORDER BY created_at DESC
		` + pageClause
	rows, err := r.db.Query(r.db.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*model.LLMProxy
	for rows.Next() {
		var p model.LLMProxy
		var createdBy sql.NullString
		var openAPISpec, configurationJSON []byte
		err := rows.Scan(
			&p.UUID, &p.ID, &p.Name, &p.Version, &p.OrganizationUUID, &p.CreatedAt, &p.UpdatedAt,
			&p.ProjectUUID, &p.Description, &createdBy, &p.ProviderUUID,
			&openAPISpec, &configurationJSON,
		)
		if err != nil {
			return nil, err
		}
		p.CreatedBy = createdBy.String
		if len(openAPISpec) > 0 {
			p.OpenAPISpec = string(openAPISpec)
		}
		if len(configurationJSON) > 0 {
			if config, err := deserializeLLMProxyConfiguration(configurationJSON); err != nil {
				return nil, fmt.Errorf("unmarshal configuration for proxy %s: %w", p.ID, err)
			} else if config != nil {
				p.Configuration = *config
			}
		}
		res = append(res, &p)
	}
	return res, rows.Err()
}

func (r *LLMProxyRepo) ListByProvider(orgUUID, providerUUID string, limit, offset int) ([]*model.LLMProxy, error) {
	pageClause, pageArgs := r.db.PaginationClause(limit, offset)
	args := append([]any{orgUUID, providerUUID}, pageArgs...)
	query := `
		SELECT
			uuid, handle, name, version, organization_uuid, created_at, updated_at,
			project_uuid, description, created_by, provider_uuid,
			openapi_spec, configuration
		FROM llm_proxies
		WHERE organization_uuid = ? AND provider_uuid = ?
		ORDER BY created_at DESC
		` + pageClause
	rows, err := r.db.Query(r.db.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*model.LLMProxy
	for rows.Next() {
		var p model.LLMProxy
		var createdBy sql.NullString
		var openAPISpec, configurationJSON []byte
		err := rows.Scan(
			&p.UUID, &p.ID, &p.Name, &p.Version, &p.OrganizationUUID, &p.CreatedAt, &p.UpdatedAt,
			&p.ProjectUUID, &p.Description, &createdBy, &p.ProviderUUID,
			&openAPISpec, &configurationJSON,
		)
		if err != nil {
			return nil, err
		}
		p.CreatedBy = createdBy.String
		if len(openAPISpec) > 0 {
			p.OpenAPISpec = string(openAPISpec)
		}
		if len(configurationJSON) > 0 {
			if config, err := deserializeLLMProxyConfiguration(configurationJSON); err != nil {
				return nil, fmt.Errorf("unmarshal configuration for proxy %s: %w", p.ID, err)
			} else if config != nil {
				p.Configuration = *config
			}
		}
		res = append(res, &p)
	}
	return res, rows.Err()
}

func (r *LLMProxyRepo) Count(orgUUID string) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM llm_proxies WHERE organization_uuid = ?`
	if err := r.db.QueryRow(r.db.Rebind(query), orgUUID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *LLMProxyRepo) CountByProject(orgUUID, projectUUID string) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM llm_proxies WHERE organization_uuid = ? AND project_uuid = ?`
	if err := r.db.QueryRow(r.db.Rebind(query), orgUUID, projectUUID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *LLMProxyRepo) CountByProvider(orgUUID, providerUUID string) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM llm_proxies WHERE organization_uuid = ? AND provider_uuid = ?`
	if err := r.db.QueryRow(r.db.Rebind(query), orgUUID, providerUUID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *LLMProxyRepo) Update(p *model.LLMProxy) error {
	now := time.Now()
	p.UpdatedAt = now

	configurationJSON, err := serializeLLMProxyConfiguration(p.Configuration)
	if err != nil {
		return fmt.Errorf("failed to serialize configuration: %w", err)
	}

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get the proxy UUID from handle
	var proxyUUID string
	query := `
		SELECT uuid FROM llm_proxies
		WHERE handle = ? AND organization_uuid = ?`
	err = tx.QueryRow(r.db.Rebind(query), p.ID, p.OrganizationUUID).Scan(&proxyUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		return err
	}

	// Update llm_proxies table (name/version/updated_at now live here)
	query = `
		UPDATE llm_proxies
		SET name = ?, version = ?, description = ?, provider_uuid = ?,
			openapi_spec = ?, configuration = ?, updated_by = ?, updated_at = ?
		WHERE uuid = ?`
	result, err := tx.Exec(r.db.Rebind(query),
		p.Name, p.Version, p.Description, p.ProviderUUID,
		[]byte(p.OpenAPISpec), configurationJSON, p.UpdatedBy, now,
		proxyUUID,
	)
	if err != nil {
		return fmt.Errorf("failed to update proxy: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (r *LLMProxyRepo) Delete(proxyID, orgUUID string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get the proxy UUID from handle
	var proxyUUID string
	query := `
		SELECT uuid FROM llm_proxies
		WHERE handle = ? AND organization_uuid = ?`
	err = tx.QueryRow(r.db.Rebind(query), proxyID, orgUUID).Scan(&proxyUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		return err
	}

	// Delete from llm_proxies first, then artifacts using artifactRepo
	_, err = tx.Exec(r.db.Rebind(`DELETE FROM llm_proxies WHERE uuid = ?`), proxyUUID)
	if err != nil {
		return err
	}

	if err := r.artifactRepo.Delete(tx, proxyUUID); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (r *LLMProxyRepo) Exists(proxyID, orgUUID string) (bool, error) {
	return r.artifactRepo.Exists(constants.LLMProxy, proxyID, orgUUID)
}

func marshalPolicies(policies []model.LLMPolicy) (string, error) {
	if policies == nil {
		policies = []model.LLMPolicy{}
	}
	policiesJSON, err := json.Marshal(policies)
	if err != nil {
		return "", err
	}
	return string(policiesJSON), nil
}

func unmarshalPolicies(policiesJSON sql.NullString) ([]model.LLMPolicy, error) {
	if !policiesJSON.Valid || policiesJSON.String == "" {
		return []model.LLMPolicy{}, nil
	}
	var policies []model.LLMPolicy
	if err := json.Unmarshal([]byte(policiesJSON.String), &policies); err != nil {
		return nil, err
	}
	return policies, nil
}

func serializeLLMProviderConfiguration(config model.LLMProviderConfig) ([]byte, error) {
	return json.Marshal(config)
}

func deserializeLLMProviderConfiguration(configJSON []byte) (*model.LLMProviderConfig, error) {
	if len(configJSON) == 0 {
		return nil, fmt.Errorf("null configuration")
	}
	var config model.LLMProviderConfig
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func serializeLLMProxyConfiguration(config model.LLMProxyConfig) ([]byte, error) {
	return json.Marshal(config)
}

func deserializeLLMProxyConfiguration(configJSON []byte) (*model.LLMProxyConfig, error) {
	if len(configJSON) == 0 {
		return nil, fmt.Errorf("null configuration")
	}
	var config model.LLMProxyConfig
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return nil, err
	}
	return &config, nil
}
