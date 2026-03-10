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

	configJSON, err := json.Marshal(&llmProviderTemplateConfig{
		Metadata:         t.Metadata,
		PromptTokens:     t.PromptTokens,
		CompletionTokens: t.CompletionTokens,
		TotalTokens:      t.TotalTokens,
		RemainingTokens:  t.RemainingTokens,
		RequestModel:     t.RequestModel,
		ResponseModel:    t.ResponseModel,
	})
	if err != nil {
		return err
	}

	query := `
		INSERT INTO llm_provider_templates (
			uuid, organization_uuid, handle, name, description, created_by,
			configuration, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err = r.db.Exec(r.db.Rebind(query),
		t.UUID, t.OrganizationUUID, t.ID, t.Name, t.Description, t.CreatedBy,
		string(configJSON),
		t.CreatedAt, t.UpdatedAt,
	)
	return err
}

func (r *LLMProviderTemplateRepo) GetByID(templateID, orgUUID string) (*model.LLMProviderTemplate, error) {
	row := r.db.QueryRow(r.db.Rebind(`
		SELECT uuid, organization_uuid, handle, name, description, created_by, configuration, created_at, updated_at
		FROM llm_provider_templates
		WHERE handle = ? AND organization_uuid = ?
	`), templateID, orgUUID)

	var t model.LLMProviderTemplate
	var configJSON sql.NullString
	if err := row.Scan(
		&t.UUID, &t.OrganizationUUID, &t.ID, &t.Name, &t.Description, &t.CreatedBy, &configJSON,
		&t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if configJSON.Valid && configJSON.String != "" {
		var cfg llmProviderTemplateConfig
		if err := json.Unmarshal([]byte(configJSON.String), &cfg); err != nil {
			return nil, err
		}
		t.Metadata = cfg.Metadata
		t.PromptTokens = cfg.PromptTokens
		t.CompletionTokens = cfg.CompletionTokens
		t.TotalTokens = cfg.TotalTokens
		t.RemainingTokens = cfg.RemainingTokens
		t.RequestModel = cfg.RequestModel
		t.ResponseModel = cfg.ResponseModel
	}

	return &t, nil
}

func (r *LLMProviderTemplateRepo) GetByUUID(uuid, orgUUID string) (*model.LLMProviderTemplate, error) {
	row := r.db.QueryRow(r.db.Rebind(`
		SELECT uuid, organization_uuid, handle, name, description, created_by, configuration, created_at, updated_at
		FROM llm_provider_templates
		WHERE uuid = ? AND organization_uuid = ?
	`), uuid, orgUUID)

	var t model.LLMProviderTemplate
	var configJSON sql.NullString
	if err := row.Scan(
		&t.UUID, &t.OrganizationUUID, &t.ID, &t.Name, &t.Description, &t.CreatedBy, &configJSON,
		&t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if configJSON.Valid && configJSON.String != "" {
		var cfg llmProviderTemplateConfig
		if err := json.Unmarshal([]byte(configJSON.String), &cfg); err != nil {
			return nil, err
		}
		t.Metadata = cfg.Metadata
		t.PromptTokens = cfg.PromptTokens
		t.CompletionTokens = cfg.CompletionTokens
		t.TotalTokens = cfg.TotalTokens
		t.RemainingTokens = cfg.RemainingTokens
		t.RequestModel = cfg.RequestModel
		t.ResponseModel = cfg.ResponseModel
	}

	return &t, nil
}

func (r *LLMProviderTemplateRepo) List(orgUUID string, limit, offset int) ([]*model.LLMProviderTemplate, error) {
	rows, err := r.db.Query(r.db.Rebind(`
		SELECT uuid, organization_uuid, handle, name, description, created_by, configuration, created_at, updated_at
		FROM llm_provider_templates
		WHERE organization_uuid = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`), orgUUID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*model.LLMProviderTemplate
	for rows.Next() {
		var t model.LLMProviderTemplate
		var configJSON sql.NullString
		err := rows.Scan(
			&t.UUID, &t.OrganizationUUID, &t.ID, &t.Name, &t.Description, &t.CreatedBy, &configJSON,
			&t.CreatedAt, &t.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		if configJSON.Valid && configJSON.String != "" {
			var cfg llmProviderTemplateConfig
			if err := json.Unmarshal([]byte(configJSON.String), &cfg); err != nil {
				return nil, err
			}
			t.Metadata = cfg.Metadata
			t.PromptTokens = cfg.PromptTokens
			t.CompletionTokens = cfg.CompletionTokens
			t.TotalTokens = cfg.TotalTokens
			t.RemainingTokens = cfg.RemainingTokens
			t.RequestModel = cfg.RequestModel
			t.ResponseModel = cfg.ResponseModel
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
	})
	if err != nil {
		return err
	}

	query := `
		UPDATE llm_provider_templates
		SET name = ?, description = ?, configuration = ?, updated_at = ?
		WHERE handle = ? AND organization_uuid = ?
	`
	result, err := r.db.Exec(r.db.Rebind(query),
		t.Name, t.Description,
		string(configJSON),
		t.UpdatedAt,
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
		Handle:           p.ID,
		Name:             p.Name,
		Version:          p.Version,
		Kind:             constants.LLMProvider,
		OrganizationUUID: p.OrganizationUUID,
	}); err != nil {
		return fmt.Errorf("failed to create artifact: %w", err)
	}

	configurationJSON, err := serializeLLMProviderConfiguration(p.Configuration)
	if err != nil {
		return fmt.Errorf("failed to serialize configuration: %w", err)
	}

	// Insert into llm_providers table
	query := `
		INSERT INTO llm_providers (
			uuid, description, created_by, template_uuid, openapi_spec, model_list, status, configuration
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = tx.Exec(r.db.Rebind(query),
		p.UUID, p.Description, p.CreatedBy, p.TemplateUUID,
		p.OpenAPISpec, string(modelProvidersJSON), p.Status, configurationJSON,
	)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (r *LLMProviderRepo) GetByID(providerID, orgUUID string) (*model.LLMProvider, error) {
	query := `
		SELECT
			a.uuid, a.handle, a.name, a.version, a.organization_uuid, a.created_at, a.updated_at,
			p.description, p.created_by, p.template_uuid, p.openapi_spec, p.model_list, p.status, p.configuration
		FROM artifacts a
		JOIN llm_providers p ON a.uuid = p.uuid
		WHERE a.handle = ? AND a.organization_uuid = ? AND a.kind = ?`
	row := r.db.QueryRow(r.db.Rebind(query), providerID, orgUUID, constants.LLMProvider)

	var p model.LLMProvider
	var openAPISpec, modelProvidersRaw sql.NullString
	var configurationJSON sql.NullString
	if err := row.Scan(
		&p.UUID, &p.ID, &p.Name, &p.Version, &p.OrganizationUUID, &p.CreatedAt, &p.UpdatedAt,
		&p.Description, &p.CreatedBy, &p.TemplateUUID, &openAPISpec, &modelProvidersRaw, &p.Status, &configurationJSON,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if configurationJSON.Valid && configurationJSON.String != "" {
		if config, err := deserializeLLMProviderConfiguration(configurationJSON); err != nil {
			return nil, fmt.Errorf("unmarshal configuration for provider %s: %w", p.ID, err)
		} else if config != nil {
			p.Configuration = *config
		}
	}

	if openAPISpec.Valid {
		p.OpenAPISpec = openAPISpec.String
	}
	if modelProvidersRaw.Valid && modelProvidersRaw.String != "" {
		if err := json.Unmarshal([]byte(modelProvidersRaw.String), &p.ModelProviders); err != nil {
			return nil, fmt.Errorf("unmarshal modelProviders for provider %s: %w", p.ID, err)
		}
	}

	return &p, nil
}

func (r *LLMProviderRepo) List(orgUUID string, limit, offset int) ([]*model.LLMProvider, error) {
	query := `
		SELECT
			a.uuid, a.handle, a.name, a.version, a.organization_uuid, a.created_at, a.updated_at,
			p.description, p.created_by, p.template_uuid, p.openapi_spec, p.model_list, p.status, p.configuration
		FROM artifacts a
		JOIN llm_providers p ON a.uuid = p.uuid
		WHERE a.organization_uuid = ? AND a.kind = ?
		ORDER BY a.created_at DESC
		LIMIT ? OFFSET ?`
	rows, err := r.db.Query(r.db.Rebind(query), orgUUID, constants.LLMProvider, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*model.LLMProvider
	for rows.Next() {
		var p model.LLMProvider
		var openAPISpec, modelProvidersRaw sql.NullString
		var configurationJSON sql.NullString
		err := rows.Scan(
			&p.UUID, &p.ID, &p.Name, &p.Version, &p.OrganizationUUID, &p.CreatedAt, &p.UpdatedAt,
			&p.Description, &p.CreatedBy, &p.TemplateUUID, &openAPISpec, &modelProvidersRaw, &p.Status, &configurationJSON,
		)
		if err != nil {
			return nil, err
		}
		if openAPISpec.Valid {
			p.OpenAPISpec = openAPISpec.String
		}
		if modelProvidersRaw.Valid && modelProvidersRaw.String != "" {
			if err := json.Unmarshal([]byte(modelProvidersRaw.String), &p.ModelProviders); err != nil {
				return nil, fmt.Errorf("unmarshal modelProviders for provider %s: %w", p.ID, err)
			}
		}
		if configurationJSON.Valid && configurationJSON.String != "" {
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
		SELECT uuid FROM artifacts
		WHERE handle = ? AND organization_uuid = ? AND kind = ?`
	err = tx.QueryRow(r.db.Rebind(query), p.ID, p.OrganizationUUID, constants.LLMProvider).Scan(&providerUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		return err
	}

	// Update artifacts table
	if err := r.artifactRepo.Update(tx, &model.Artifact{
		UUID:             providerUUID,
		Name:             p.Name,
		Version:          p.Version,
		OrganizationUUID: p.OrganizationUUID,
		UpdatedAt:        now,
	}); err != nil {
		return fmt.Errorf("failed to update artifact: %w", err)
	}

	// Update llm_providers table
	query = `
		UPDATE llm_providers
		SET description = ?, template_uuid = ?, openapi_spec = ?, model_list = ?, status = ?, configuration = ?
		WHERE uuid = ?`
	result, err := tx.Exec(r.db.Rebind(query),
		p.Description, p.TemplateUUID, p.OpenAPISpec, string(modelProvidersJSON), p.Status, configurationJSON,
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
		SELECT uuid FROM artifacts
		WHERE handle = ? AND organization_uuid = ? AND kind = ?`
	err = tx.QueryRow(r.db.Rebind(query), providerID, orgUUID, constants.LLMProvider).Scan(&providerUUID)
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
		Handle:           p.ID,
		Name:             p.Name,
		Version:          p.Version,
		Kind:             constants.LLMProxy,
		OrganizationUUID: p.OrganizationUUID,
	}); err != nil {
		return fmt.Errorf("failed to create artifact: %w", err)
	}

	// Insert into llm_proxies table
	query := `
		INSERT INTO llm_proxies (
			uuid, project_uuid, description, created_by, provider_uuid, openapi_spec, status, configuration
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = tx.Exec(r.db.Rebind(query),
		p.UUID, p.ProjectUUID, p.Description, p.CreatedBy, p.ProviderUUID,
		p.OpenAPISpec, p.Status, configurationJSON,
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
			a.uuid, a.handle, a.name, a.version, a.organization_uuid, a.created_at, a.updated_at,
			p.project_uuid, p.description, p.created_by, p.provider_uuid, p.openapi_spec, p.status, p.configuration
		FROM artifacts a
		JOIN llm_proxies p ON a.uuid = p.uuid
		WHERE a.handle = ? AND a.organization_uuid = ? AND a.kind = ?`
	row := r.db.QueryRow(r.db.Rebind(query), proxyID, orgUUID, constants.LLMProxy)

	var p model.LLMProxy
	var openAPISpec, configurationJSON sql.NullString
	if err := row.Scan(
		&p.UUID, &p.ID, &p.Name, &p.Version, &p.OrganizationUUID, &p.CreatedAt, &p.UpdatedAt,
		&p.ProjectUUID, &p.Description, &p.CreatedBy, &p.ProviderUUID,
		&openAPISpec, &p.Status, &configurationJSON,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if openAPISpec.Valid {
		p.OpenAPISpec = openAPISpec.String
	}
	if configurationJSON.Valid && configurationJSON.String != "" {
		if config, err := deserializeLLMProxyConfiguration(configurationJSON); err != nil {
			return nil, fmt.Errorf("unmarshal configuration for proxy %s: %w", p.ID, err)
		} else if config != nil {
			p.Configuration = *config
		}
	}

	return &p, nil
}

func (r *LLMProxyRepo) List(orgUUID string, limit, offset int) ([]*model.LLMProxy, error) {
	query := `
		SELECT
			a.uuid, a.handle, a.name, a.version, a.organization_uuid, a.created_at, a.updated_at,
			p.project_uuid, p.description, p.created_by, p.provider_uuid,
			p.openapi_spec, p.status, p.configuration
		FROM artifacts a
		JOIN llm_proxies p ON a.uuid = p.uuid
		WHERE a.organization_uuid = ? AND a.kind = ?
		ORDER BY a.created_at DESC
		LIMIT ? OFFSET ?`
	rows, err := r.db.Query(r.db.Rebind(query), orgUUID, constants.LLMProxy, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*model.LLMProxy
	for rows.Next() {
		var p model.LLMProxy
		var openAPISpec, configurationJSON sql.NullString
		err := rows.Scan(
			&p.UUID, &p.ID, &p.Name, &p.Version, &p.OrganizationUUID, &p.CreatedAt, &p.UpdatedAt,
			&p.ProjectUUID, &p.Description, &p.CreatedBy, &p.ProviderUUID,
			&openAPISpec, &p.Status, &configurationJSON,
		)
		if err != nil {
			return nil, err
		}
		if openAPISpec.Valid {
			p.OpenAPISpec = openAPISpec.String
		}
		if configurationJSON.Valid && configurationJSON.String != "" {
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
	query := `
		SELECT
			a.uuid, a.handle, a.name, a.version, a.organization_uuid, a.created_at, a.updated_at,
			p.project_uuid, p.description, p.created_by, p.provider_uuid,
			p.openapi_spec, p.status, p.configuration
		FROM artifacts a
		JOIN llm_proxies p ON a.uuid = p.uuid
		WHERE a.organization_uuid = ? AND p.project_uuid = ? AND a.kind = ?
		ORDER BY a.created_at DESC
		LIMIT ? OFFSET ?`
	rows, err := r.db.Query(r.db.Rebind(query), orgUUID, projectUUID, constants.LLMProxy, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*model.LLMProxy
	for rows.Next() {
		var p model.LLMProxy
		var openAPISpec, configurationJSON sql.NullString
		err := rows.Scan(
			&p.UUID, &p.ID, &p.Name, &p.Version, &p.OrganizationUUID, &p.CreatedAt, &p.UpdatedAt,
			&p.ProjectUUID, &p.Description, &p.CreatedBy, &p.ProviderUUID,
			&openAPISpec, &p.Status, &configurationJSON,
		)
		if err != nil {
			return nil, err
		}
		if openAPISpec.Valid {
			p.OpenAPISpec = openAPISpec.String
		}
		if configurationJSON.Valid && configurationJSON.String != "" {
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
	query := `
		SELECT
			a.uuid, a.handle, a.name, a.version, a.organization_uuid, a.created_at, a.updated_at,
			p.project_uuid, p.description, p.created_by, p.provider_uuid,
			p.openapi_spec, p.status, p.configuration
		FROM artifacts a
		JOIN llm_proxies p ON a.uuid = p.uuid
		WHERE a.organization_uuid = ? AND p.provider_uuid = ? AND a.kind = ?
		ORDER BY a.created_at DESC
		LIMIT ? OFFSET ?`
	rows, err := r.db.Query(r.db.Rebind(query), orgUUID, providerUUID, constants.LLMProxy, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*model.LLMProxy
	for rows.Next() {
		var p model.LLMProxy
		var openAPISpec, configurationJSON sql.NullString
		err := rows.Scan(
			&p.UUID, &p.ID, &p.Name, &p.Version, &p.OrganizationUUID, &p.CreatedAt, &p.UpdatedAt,
			&p.ProjectUUID, &p.Description, &p.CreatedBy, &p.ProviderUUID,
			&openAPISpec, &p.Status, &configurationJSON,
		)
		if err != nil {
			return nil, err
		}
		if openAPISpec.Valid {
			p.OpenAPISpec = openAPISpec.String
		}
		if configurationJSON.Valid && configurationJSON.String != "" {
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
	query := `
		SELECT COUNT(*) FROM artifacts a
		JOIN llm_proxies p ON a.uuid = p.uuid
		WHERE a.organization_uuid = ? AND a.kind = ?`
	if err := r.db.QueryRow(r.db.Rebind(query), orgUUID, constants.LLMProxy).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *LLMProxyRepo) CountByProject(orgUUID, projectUUID string) (int, error) {
	var count int
	query := `
		SELECT COUNT(*) FROM artifacts a
		JOIN llm_proxies p ON a.uuid = p.uuid
		WHERE a.organization_uuid = ? AND p.project_uuid = ? AND a.kind = ?`
	if err := r.db.QueryRow(r.db.Rebind(query), orgUUID, projectUUID, constants.LLMProxy).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *LLMProxyRepo) CountByProvider(orgUUID, providerUUID string) (int, error) {
	var count int
	query := `
		SELECT COUNT(*) FROM artifacts a
		JOIN llm_proxies p ON a.uuid = p.uuid
		WHERE a.organization_uuid = ? AND p.provider_uuid = ? AND a.kind = ?`
	if err := r.db.QueryRow(r.db.Rebind(query), orgUUID, providerUUID, constants.LLMProxy).Scan(&count); err != nil {
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
		SELECT uuid FROM artifacts
		WHERE handle = ? AND organization_uuid = ? AND kind = ?`
	reboundQuery := r.db.Rebind(query)
	err = tx.QueryRow(reboundQuery, p.ID, p.OrganizationUUID, constants.LLMProxy).Scan(&proxyUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		return err
	}

	// Update artifacts table
	if err := r.artifactRepo.Update(tx, &model.Artifact{
		UUID:             proxyUUID,
		Name:             p.Name,
		Version:          p.Version,
		OrganizationUUID: p.OrganizationUUID,
		UpdatedAt:        now,
	}); err != nil {
		return fmt.Errorf("failed to update artifact: %w", err)
	}

	// Update llm_proxies table
	query = `
		UPDATE llm_proxies
		SET description = ?, provider_uuid = ?,
			openapi_spec = ?, status = ?, configuration = ?
		WHERE uuid = ?`
	result, err := tx.Exec(r.db.Rebind(query),
		p.Description, p.ProviderUUID,
		p.OpenAPISpec, p.Status, configurationJSON,
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
		SELECT uuid FROM artifacts
		WHERE handle = ? AND organization_uuid = ? AND kind = ?`
	err = tx.QueryRow(r.db.Rebind(query), proxyID, orgUUID, constants.LLMProxy).Scan(&proxyUUID)
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

func serializeLLMProviderConfiguration(config model.LLMProviderConfig) (string, error) {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(configJSON), nil
}

func deserializeLLMProviderConfiguration(configJSON sql.NullString) (*model.LLMProviderConfig, error) {
	if !configJSON.Valid || configJSON.String == "" {
		return nil, fmt.Errorf("null configuration")
	}
	var config model.LLMProviderConfig
	if err := json.Unmarshal([]byte(configJSON.String), &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func serializeLLMProxyConfiguration(config model.LLMProxyConfig) (string, error) {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(configJSON), nil
}

func deserializeLLMProxyConfiguration(configJSON sql.NullString) (*model.LLMProxyConfig, error) {
	if !configJSON.Valid || configJSON.String == "" {
		return nil, fmt.Errorf("null configuration")
	}
	var config model.LLMProxyConfig
	if err := json.Unmarshal([]byte(configJSON.String), &config); err != nil {
		return nil, err
	}
	return &config, nil
}
