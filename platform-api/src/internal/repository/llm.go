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

	"platform-api/src/internal/database"
	"platform-api/src/internal/model"

	"github.com/google/uuid"
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
	u, err := uuid.NewV7()
	if err != nil {
		return err
	}
	t.UUID = u.String()
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
	_, err = r.db.Exec(query,
		t.UUID, t.OrganizationUUID, t.ID, t.Name, t.Description, t.CreatedBy,
		string(configJSON),
		t.CreatedAt, t.UpdatedAt,
	)
	return err
}

func (r *LLMProviderTemplateRepo) GetByID(templateID, orgUUID string) (*model.LLMProviderTemplate, error) {
	row := r.db.QueryRow(`
		SELECT uuid, organization_uuid, handle, name, description, created_by, configuration, created_at, updated_at
		FROM llm_provider_templates
		WHERE handle = ? AND organization_uuid = ?
	`, templateID, orgUUID)

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
	rows, err := r.db.Query(`
		SELECT uuid, organization_uuid, handle, name, description, created_by, configuration, created_at, updated_at
		FROM llm_provider_templates
		WHERE organization_uuid = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, orgUUID, limit, offset)
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
	result, err := r.db.Exec(query,
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
	result, err := r.db.Exec(`DELETE FROM llm_provider_templates WHERE handle = ? AND organization_uuid = ?`, templateID, orgUUID)
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
	err := r.db.QueryRow(`SELECT COUNT(*) FROM llm_provider_templates WHERE handle = ? AND organization_uuid = ?`, templateID, orgUUID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *LLMProviderTemplateRepo) Count(orgUUID string) (int, error) {
	var count int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM llm_provider_templates WHERE organization_uuid = ?`, orgUUID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// ---- LLM Providers ----

type LLMProviderRepo struct {
	db *database.DB
}

func NewLLMProviderRepo(db *database.DB) LLMProviderRepository {
	return &LLMProviderRepo{db: db}
}

func (r *LLMProviderRepo) Create(p *model.LLMProvider) error {
	u, err := uuid.NewV7()
	if err != nil {
		return err
	}
	p.UUID = u.String()
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()

	policiesColumn, err := marshalPolicies(p.Policies)
	if err != nil {
		return err
	}
	securityJSON, err := json.Marshal(p.Security)
	if err != nil {
		return err
	}
	rateLimitingJSON, err := json.Marshal(p.RateLimiting)
	if err != nil {
		return err
	}
	accessControlJSON, err := json.Marshal(p.AccessControl)
	if err != nil {
		return err
	}
	modelProvidersJSON, err := json.Marshal(p.ModelProviders)
	if err != nil {
		return err
	}
	var upstreamAuthJSON []byte
	if p.UpstreamAuth != nil {
		upstreamAuthJSON, err = json.Marshal(p.UpstreamAuth)
		if err != nil {
			return err
		}
	}

	query := `
		INSERT INTO llm_providers (
			uuid, organization_uuid, handle, name, description, created_by, version, context, vhost, template,
			upstream_url, upstream_auth, openapi_spec, model_list, rate_limiting, access_control, policies, security, status, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(query,
		p.UUID, p.OrganizationUUID, p.ID, p.Name, p.Description, p.CreatedBy, p.Version, p.Context, p.VHost, p.Template,
		p.UpstreamURL, string(upstreamAuthJSON), p.OpenAPISpec, string(modelProvidersJSON), string(rateLimitingJSON),
		string(accessControlJSON), policiesColumn, string(securityJSON), p.Status,
		p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (r *LLMProviderRepo) GetByID(providerID, orgUUID string) (*model.LLMProvider, error) {
	row := r.db.QueryRow(`
		SELECT uuid, organization_uuid, handle, name, description, created_by, version, context, vhost, template,
			upstream_url, upstream_auth, openapi_spec, model_list, rate_limiting, access_control, policies, security, status, created_at, updated_at
		FROM llm_providers
		WHERE handle = ? AND organization_uuid = ?
	`, providerID, orgUUID)

	var p model.LLMProvider
	var upstreamURL, openAPISpec, modelProvidersRaw sql.NullString
	var upstreamAuthJSON, rateLimitingJSON, accessControlJSON, policiesJSON, securityJSON sql.NullString
	if err := row.Scan(
		&p.UUID, &p.OrganizationUUID, &p.ID, &p.Name, &p.Description, &p.CreatedBy, &p.Version, &p.Context, &p.VHost, &p.Template,
		&upstreamURL, &upstreamAuthJSON, &openAPISpec, &modelProvidersRaw, &rateLimitingJSON,
		&accessControlJSON, &policiesJSON, &securityJSON, &p.Status, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	p.UpstreamURL = upstreamURL.String
	if upstreamAuthJSON.Valid && upstreamAuthJSON.String != "" {
		if err := json.Unmarshal([]byte(upstreamAuthJSON.String), &p.UpstreamAuth); err != nil {
			return nil, fmt.Errorf("unmarshal upstreamAuth for provider %s: %w", p.ID, err)
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
	if rateLimitingJSON.Valid && rateLimitingJSON.String != "" {
		if err := json.Unmarshal([]byte(rateLimitingJSON.String), &p.RateLimiting); err != nil {
			return nil, fmt.Errorf("unmarshal rateLimiting for provider %s: %w", p.ID, err)
		}
	}
	policies, err := unmarshalPolicies(policiesJSON)
	if err != nil {
		return nil, fmt.Errorf("unmarshal policies for provider %s: %w", p.ID, err)
	}
	p.Policies = policies

	if accessControlJSON.Valid && accessControlJSON.String != "" {
		if err := json.Unmarshal([]byte(accessControlJSON.String), &p.AccessControl); err != nil {
			return nil, fmt.Errorf("unmarshal accessControl for provider %s: %w", p.ID, err)
		}
	}
	if securityJSON.Valid && securityJSON.String != "" {
		if err := json.Unmarshal([]byte(securityJSON.String), &p.Security); err != nil {
			return nil, fmt.Errorf("unmarshal security for provider %s: %w", p.ID, err)
		}
	}

	return &p, nil
}

func (r *LLMProviderRepo) List(orgUUID string, limit, offset int) ([]*model.LLMProvider, error) {
	rows, err := r.db.Query(`
		SELECT uuid, organization_uuid, handle, name, description, created_by, version, context, vhost, template,
			upstream_url, upstream_auth, openapi_spec, model_list, rate_limiting, access_control, policies, security, status, created_at, updated_at
		FROM llm_providers
		WHERE organization_uuid = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, orgUUID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*model.LLMProvider
	for rows.Next() {
		var p model.LLMProvider
		var upstreamURL, openAPISpec, modelProvidersRaw sql.NullString
		var upstreamAuthJSON, rateLimitingJSON, accessControlJSON, policiesJSON, securityJSON sql.NullString
		err := rows.Scan(
			&p.UUID, &p.OrganizationUUID, &p.ID, &p.Name, &p.Description, &p.CreatedBy, &p.Version, &p.Context, &p.VHost, &p.Template,
			&upstreamURL, &upstreamAuthJSON, &openAPISpec, &modelProvidersRaw, &rateLimitingJSON,
			&accessControlJSON, &policiesJSON, &securityJSON, &p.Status, &p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		p.UpstreamURL = upstreamURL.String
		if upstreamAuthJSON.Valid && upstreamAuthJSON.String != "" {
			_ = json.Unmarshal([]byte(upstreamAuthJSON.String), &p.UpstreamAuth)
		}
		if openAPISpec.Valid {
			p.OpenAPISpec = openAPISpec.String
		}
		if modelProvidersRaw.Valid && modelProvidersRaw.String != "" {
			_ = json.Unmarshal([]byte(modelProvidersRaw.String), &p.ModelProviders)
		}
		if rateLimitingJSON.Valid && rateLimitingJSON.String != "" {
			_ = json.Unmarshal([]byte(rateLimitingJSON.String), &p.RateLimiting)
		}
		policies, err := unmarshalPolicies(policiesJSON)
		if err != nil {
			return nil, fmt.Errorf("unmarshal policies for provider %s: %w", p.ID, err)
		}
		p.Policies = policies
		if accessControlJSON.Valid && accessControlJSON.String != "" {
			_ = json.Unmarshal([]byte(accessControlJSON.String), &p.AccessControl)
		}
		if securityJSON.Valid && securityJSON.String != "" {
			_ = json.Unmarshal([]byte(securityJSON.String), &p.Security)
		}
		res = append(res, &p)
	}
	return res, rows.Err()
}

func (r *LLMProviderRepo) Count(orgUUID string) (int, error) {
	var count int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM llm_providers WHERE organization_uuid = ?`, orgUUID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *LLMProviderRepo) Update(p *model.LLMProvider) error {
	p.UpdatedAt = time.Now()

	policiesColumn, err := marshalPolicies(p.Policies)
	if err != nil {
		return err
	}
	securityJSON, err := json.Marshal(p.Security)
	if err != nil {
		return err
	}
	rateLimitingJSON, err := json.Marshal(p.RateLimiting)
	if err != nil {
		return err
	}
	accessControlJSON, err := json.Marshal(p.AccessControl)
	if err != nil {
		return err
	}
	modelProvidersJSON, err := json.Marshal(p.ModelProviders)
	if err != nil {
		return err
	}
	var upstreamAuthJSON []byte
	if p.UpstreamAuth != nil {
		upstreamAuthJSON, err = json.Marshal(p.UpstreamAuth)
		if err != nil {
			return err
		}
	}

	query := `
		UPDATE llm_providers
		SET name = ?, description = ?, version = ?, context = ?, vhost = ?, template = ?,
			upstream_url = ?, upstream_auth = ?, openapi_spec = ?, model_list = ?, rate_limiting = ?, access_control = ?, policies = ?, security = ?, status = ?, updated_at = ?
		WHERE handle = ? AND organization_uuid = ?
	`
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.Exec(query,
		p.Name, p.Description, p.Version, p.Context, p.VHost, p.Template,
		p.UpstreamURL, string(upstreamAuthJSON), p.OpenAPISpec, string(modelProvidersJSON), string(rateLimitingJSON),
		string(accessControlJSON), policiesColumn, string(securityJSON), p.Status, p.UpdatedAt,
		p.ID, p.OrganizationUUID,
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

	result, err := tx.Exec(`DELETE FROM llm_providers WHERE handle = ? AND organization_uuid = ?`, providerID, orgUUID)
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

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (r *LLMProviderRepo) Exists(providerID, orgUUID string) (bool, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM llm_providers WHERE handle = ? AND organization_uuid = ?`, providerID, orgUUID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ---- LLM Proxies ----

type LLMProxyRepo struct {
	db *database.DB
}

func NewLLMProxyRepo(db *database.DB) LLMProxyRepository {
	return &LLMProxyRepo{db: db}
}

func (r *LLMProxyRepo) Create(p *model.LLMProxy) error {
	u, err := uuid.NewV7()
	if err != nil {
		return err
	}
	p.UUID = u.String()
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()

	policiesColumn, err := marshalPolicies(p.Policies)
	if err != nil {
		return err
	}
	securityJSON, err := json.Marshal(p.Security)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO llm_proxies (
			uuid, organization_uuid, project_uuid, handle, name, description, created_by, version, context, vhost, provider,
			openapi_spec, policies, security, status, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(query,
		p.UUID, p.OrganizationUUID, p.ProjectUUID, p.ID, p.Name, p.Description, p.CreatedBy, p.Version, p.Context, p.VHost, p.Provider,
		p.OpenAPISpec, policiesColumn, string(securityJSON), p.Status,
		p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (r *LLMProxyRepo) GetByID(proxyID, orgUUID string) (*model.LLMProxy, error) {
	row := r.db.QueryRow(`
		SELECT uuid, organization_uuid, project_uuid, handle, name, description, created_by, version, context, vhost, provider,
				openapi_spec, policies, security, status, created_at, updated_at
		FROM llm_proxies
		WHERE handle = ? AND organization_uuid = ?
	`, proxyID, orgUUID)

	var p model.LLMProxy
	var openAPISpec, policiesJSON, securityJSON sql.NullString
	if err := row.Scan(
		&p.UUID, &p.OrganizationUUID, &p.ProjectUUID, &p.ID, &p.Name, &p.Description, &p.CreatedBy, &p.Version, &p.Context, &p.VHost, &p.Provider,
		&openAPISpec, &policiesJSON, &securityJSON, &p.Status, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if openAPISpec.Valid {
		p.OpenAPISpec = openAPISpec.String
	}
	policies, err := unmarshalPolicies(policiesJSON)
	if err != nil {
		return nil, fmt.Errorf("unmarshal policies for proxy %s: %w", p.ID, err)
	}
	p.Policies = policies
	if securityJSON.Valid && securityJSON.String != "" {
		if err := json.Unmarshal([]byte(securityJSON.String), &p.Security); err != nil {
			return nil, fmt.Errorf("unmarshal security for proxy %s: %w", p.ID, err)
		}
	}

	return &p, nil
}

func (r *LLMProxyRepo) List(orgUUID string, limit, offset int) ([]*model.LLMProxy, error) {
	rows, err := r.db.Query(`
		SELECT uuid, organization_uuid, project_uuid, handle, name, description, created_by, version, context, vhost, provider,
				openapi_spec, policies, security, status, created_at, updated_at
		FROM llm_proxies
		WHERE organization_uuid = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, orgUUID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*model.LLMProxy
	for rows.Next() {
		var p model.LLMProxy
		var openAPISpec, policiesJSON, securityJSON sql.NullString
		err := rows.Scan(
			&p.UUID, &p.OrganizationUUID, &p.ProjectUUID, &p.ID, &p.Name, &p.Description, &p.CreatedBy, &p.Version, &p.Context, &p.VHost, &p.Provider,
			&openAPISpec, &policiesJSON, &securityJSON, &p.Status, &p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		if openAPISpec.Valid {
			p.OpenAPISpec = openAPISpec.String
		}
		policies, err := unmarshalPolicies(policiesJSON)
		if err != nil {
			return nil, fmt.Errorf("unmarshal policies for proxy %s: %w", p.ID, err)
		}
		p.Policies = policies
		if securityJSON.Valid && securityJSON.String != "" {
			if err := json.Unmarshal([]byte(securityJSON.String), &p.Security); err != nil {
				return nil, fmt.Errorf("unmarshal security for proxy %s: %w", p.ID, err)
			}
		}
		res = append(res, &p)
	}
	return res, rows.Err()
}

func (r *LLMProxyRepo) ListByProject(orgUUID, projectUUID string, limit, offset int) ([]*model.LLMProxy, error) {
	rows, err := r.db.Query(`
		SELECT uuid, organization_uuid, project_uuid, handle, name, description, created_by, version, context, vhost, provider,
				openapi_spec, policies, security, status, created_at, updated_at
		FROM llm_proxies
		WHERE organization_uuid = ? AND project_uuid = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, orgUUID, projectUUID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*model.LLMProxy
	for rows.Next() {
		var p model.LLMProxy
		var openAPISpec, policiesJSON, securityJSON sql.NullString
		err := rows.Scan(
			&p.UUID, &p.OrganizationUUID, &p.ProjectUUID, &p.ID, &p.Name, &p.Description, &p.CreatedBy, &p.Version, &p.Context, &p.VHost, &p.Provider,
			&openAPISpec, &policiesJSON, &securityJSON, &p.Status, &p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		if openAPISpec.Valid {
			p.OpenAPISpec = openAPISpec.String
		}
		policies, err := unmarshalPolicies(policiesJSON)
		if err != nil {
			return nil, fmt.Errorf("unmarshal policies for proxy %s: %w", p.ID, err)
		}
		p.Policies = policies
		if securityJSON.Valid && securityJSON.String != "" {
			if err := json.Unmarshal([]byte(securityJSON.String), &p.Security); err != nil {
				return nil, fmt.Errorf("unmarshal security for proxy %s (project %s): %w", p.ID, projectUUID, err)
			}
		}
		res = append(res, &p)
	}
	return res, rows.Err()
}

func (r *LLMProxyRepo) ListByProvider(orgUUID, providerID string, limit, offset int) ([]*model.LLMProxy, error) {
	rows, err := r.db.Query(`
		SELECT uuid, organization_uuid, project_uuid, handle, name, description, created_by, version, context, vhost, provider,
				openapi_spec, policies, security, status, created_at, updated_at
		FROM llm_proxies
		WHERE organization_uuid = ? AND provider = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, orgUUID, providerID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*model.LLMProxy
	for rows.Next() {
		var p model.LLMProxy
		var openAPISpec, policiesJSON, securityJSON sql.NullString
		err := rows.Scan(
			&p.UUID, &p.OrganizationUUID, &p.ProjectUUID, &p.ID, &p.Name, &p.Description, &p.CreatedBy, &p.Version, &p.Context, &p.VHost, &p.Provider,
			&openAPISpec, &policiesJSON, &securityJSON, &p.Status, &p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		if openAPISpec.Valid {
			p.OpenAPISpec = openAPISpec.String
		}
		policies, err := unmarshalPolicies(policiesJSON)
		if err != nil {
			return nil, fmt.Errorf("unmarshal policies for proxy %s: %w", p.ID, err)
		}
		p.Policies = policies
		if securityJSON.Valid && securityJSON.String != "" {
			if err := json.Unmarshal([]byte(securityJSON.String), &p.Security); err != nil {
				return nil, fmt.Errorf("unmarshal security for proxy %s (provider %s): %w", p.ID, providerID, err)
			}
		}
		res = append(res, &p)
	}
	return res, rows.Err()
}

func (r *LLMProxyRepo) Count(orgUUID string) (int, error) {
	var count int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM llm_proxies WHERE organization_uuid = ?`, orgUUID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *LLMProxyRepo) CountByProject(orgUUID, projectUUID string) (int, error) {
	var count int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM llm_proxies WHERE organization_uuid = ? AND project_uuid = ?`, orgUUID, projectUUID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *LLMProxyRepo) CountByProvider(orgUUID, providerID string) (int, error) {
	var count int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM llm_proxies WHERE organization_uuid = ? AND provider = ?`, orgUUID, providerID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *LLMProxyRepo) Update(p *model.LLMProxy) error {
	p.UpdatedAt = time.Now()

	policiesColumn, err := marshalPolicies(p.Policies)
	if err != nil {
		return err
	}
	securityJSON, err := json.Marshal(p.Security)
	if err != nil {
		return err
	}

	query := `
		UPDATE llm_proxies
		SET name = ?, description = ?, version = ?, context = ?, vhost = ?, provider = ?,
			openapi_spec = ?, policies = ?, security = ?, status = ?, updated_at = ?
		WHERE handle = ? AND organization_uuid = ?
	`
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.Exec(query,
		p.Name, p.Description, p.Version, p.Context, p.VHost, p.Provider,
		p.OpenAPISpec, policiesColumn, string(securityJSON), p.Status, p.UpdatedAt,
		p.ID, p.OrganizationUUID,
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

	result, err := tx.Exec(`DELETE FROM llm_proxies WHERE handle = ? AND organization_uuid = ?`, proxyID, orgUUID)
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

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (r *LLMProxyRepo) Exists(proxyID, orgUUID string) (bool, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM llm_proxies WHERE handle = ? AND organization_uuid = ?`, proxyID, orgUUID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
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
