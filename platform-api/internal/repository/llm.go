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
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// ---- LLM Provider Templates ----

type LLMProviderTemplateRepo struct {
	db *database.DB
}

type llmProviderTemplateConfig struct {
	ManagedBy        string                                     `json:"managedBy,omitempty"`
	Metadata         *model.LLMProviderTemplateMetadata         `json:"metadata,omitempty"`
	PromptTokens     *model.ExtractionIdentifier                `json:"promptTokens,omitempty"`
	CompletionTokens *model.ExtractionIdentifier                `json:"completionTokens,omitempty"`
	TotalTokens      *model.ExtractionIdentifier                `json:"totalTokens,omitempty"`
	RemainingTokens  *model.ExtractionIdentifier                `json:"remainingTokens,omitempty"`
	RequestModel     *model.ExtractionIdentifier                `json:"requestModel,omitempty"`
	ResponseModel    *model.ExtractionIdentifier                `json:"responseModel,omitempty"`
	ResourceMappings *model.LLMProviderTemplateResourceMappings `json:"resourceMappings,omitempty"`
}

func NewLLMProviderTemplateRepo(db *database.DB) LLMProviderTemplateRepository {
	return &LLMProviderTemplateRepo{db: db}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (r *LLMProviderTemplateRepo) Create(t *model.LLMProviderTemplate) error {
	uuidStr, err := utils.GenerateUUID()
	if err != nil {
		return fmt.Errorf("failed to generate LLM provider template ID: %w", err)
	}
	t.UUID = uuidStr
	// Preserve caller-provided timestamps: the DP->CP import sets created_at/updated_at to the
	// gateway deployment time (UTC) so they act as the last-in-wins watermark. Default to now
	// for control-plane-native creates that leave them unset.
	now := time.Now()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	if t.UpdatedAt.IsZero() {
		t.UpdatedAt = now
	}
	if t.Version == "" {
		t.Version = "v1.0"
	}
	if t.ManagedBy == "" {
		t.ManagedBy = "customer"
	}
	if t.GroupID == "" {
		t.GroupID = t.ID
	}
	origin := t.Origin
	if origin == "" {
		origin = constants.OriginCP
	}

	configJSON, err := json.Marshal(&llmProviderTemplateConfig{
		ManagedBy:        t.ManagedBy,
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
	t.IsLatest = true
	t.Enabled = true
	t.UpdatedBy = t.CreatedBy
	query := `
		INSERT INTO llm_provider_templates (
			uuid, organization_uuid, handle, group_id, display_name, managed_by, description, created_by, updated_by,
			origin, configuration, openapi_spec, version, is_latest, enabled, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err = r.db.Exec(r.db.Rebind(query),
		t.UUID, t.OrganizationUUID, t.ID, t.GroupID, t.Name, t.ManagedBy, t.Description, t.CreatedBy, t.UpdatedBy,
		origin, configJSON, []byte(t.OpenAPISpec), t.Version, boolToInt(t.IsLatest), boolToInt(t.Enabled),
		t.CreatedAt, t.UpdatedAt,
	)
	return err
}

const maxCreateNewVersionRetries = 3

func (r *LLMProviderTemplateRepo) CreateNewVersion(t *model.LLMProviderTemplate) error {
	configJSON, err := json.Marshal(&llmProviderTemplateConfig{
		ManagedBy:        t.ManagedBy,
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

	var lastErr error
	for attempt := 0; attempt < maxCreateNewVersionRetries; attempt++ {
		lastErr = r.createNewVersionOnce(t, configJSON)
		if lastErr == nil {
			return nil
		}
		if !r.db.IsDuplicateKeyError(lastErr) {
			return lastErr
		}
	}
	return lastErr
}

func (r *LLMProviderTemplateRepo) createNewVersionOnce(t *model.LLMProviderTemplate, configJSON []byte) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var latestUUID string
	err = tx.QueryRow(r.db.Rebind(`
		SELECT uuid FROM llm_provider_templates WHERE group_id = ? AND organization_uuid = ? AND is_latest = ?
	`), t.GroupID, t.OrganizationUUID, 1).Scan(&latestUUID)
	if errors.Is(err, sql.ErrNoRows) {
		return sql.ErrNoRows
	}
	if err != nil {
		return err
	}

	var sameVersion int
	if err = tx.QueryRow(r.db.Rebind(`
		SELECT COUNT(*) FROM llm_provider_templates WHERE group_id = ? AND organization_uuid = ? AND version = ?
	`), t.GroupID, t.OrganizationUUID, t.Version).Scan(&sameVersion); err != nil {
		return err
	}
	if sameVersion > 0 {
		return constants.ErrLLMProviderTemplateVersionExists
	}

	// Demote the current latest within this family (same group_id).
	if _, err = tx.Exec(r.db.Rebind(`
		UPDATE llm_provider_templates SET is_latest = ? WHERE group_id = ? AND organization_uuid = ? AND is_latest = ?
	`), 0, t.GroupID, t.OrganizationUUID, 1); err != nil {
		return err
	}

	uuidStr, err := utils.GenerateUUID()
	if err != nil {
		return fmt.Errorf("failed to generate LLM provider template ID: %w", err)
	}
	t.UUID = uuidStr
	t.IsLatest = true
	t.Enabled = true
	t.UpdatedBy = t.CreatedBy
	now := time.Now()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	if t.UpdatedAt.IsZero() {
		t.UpdatedAt = now
	}
	origin := t.Origin
	if origin == "" {
		origin = constants.OriginCP
	}

	if _, err = tx.Exec(r.db.Rebind(`
		INSERT INTO llm_provider_templates (
			uuid, organization_uuid, handle, group_id, display_name, managed_by, description, created_by, updated_by,
			origin, configuration, openapi_spec, version, is_latest, enabled, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`),
		t.UUID, t.OrganizationUUID, t.ID, t.GroupID, t.Name, t.ManagedBy, t.Description, t.CreatedBy, t.UpdatedBy,
		origin, configJSON, []byte(t.OpenAPISpec), t.Version, boolToInt(t.IsLatest), boolToInt(t.Enabled),
		t.CreatedAt, t.UpdatedAt,
	); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *LLMProviderTemplateRepo) familyGroupID(handle, orgUUID string) (string, error) {
	var base string
	err := r.db.QueryRow(r.db.Rebind(`
		SELECT group_id FROM llm_provider_templates WHERE handle = ? AND organization_uuid = ?
		ORDER BY (SELECT NULL) `+r.db.FetchFirstClause(1)), handle, orgUUID).Scan(&base)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return base, nil
}

func (r *LLMProviderTemplateRepo) GetGroupID(handle, orgUUID string) (string, error) {
	return r.familyGroupID(handle, orgUUID)
}

func (r *LLMProviderTemplateRepo) ManagedByForHandle(handle, orgUUID string) (string, error) {
	var managedBy string
	err := r.db.QueryRow(r.db.Rebind(`
		SELECT managed_by FROM llm_provider_templates WHERE handle = ? AND organization_uuid = ?
		ORDER BY (SELECT NULL) `+r.db.FetchFirstClause(1)), handle, orgUUID).Scan(&managedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return managedBy, nil
}

func (r *LLMProviderTemplateRepo) GetByID(templateID, orgUUID string) (*model.LLMProviderTemplate, error) {
	row := r.db.QueryRow(r.db.Rebind(`
		SELECT uuid, organization_uuid, handle, group_id, display_name, managed_by, description, created_by, updated_by,
		       origin, configuration, openapi_spec, version, is_latest, enabled, created_at, updated_at
		FROM llm_provider_templates
		WHERE handle = ? AND organization_uuid = ?
	`), templateID, orgUUID)
	return scanTemplateRow(row)
}

func (r *LLMProviderTemplateRepo) GetByUUID(uuid, orgUUID string) (*model.LLMProviderTemplate, error) {
	row := r.db.QueryRow(r.db.Rebind(`
		SELECT uuid, organization_uuid, handle, group_id, display_name, managed_by, description, created_by, updated_by,
		       origin, configuration, openapi_spec, version, is_latest, enabled, created_at, updated_at
		FROM llm_provider_templates
		WHERE uuid = ? AND organization_uuid = ?
	`), uuid, orgUUID)
	return scanTemplateRow(row)
}

func (r *LLMProviderTemplateRepo) GetByVersion(groupID, orgUUID, version string) (*model.LLMProviderTemplate, error) {
	row := r.db.QueryRow(r.db.Rebind(`
		SELECT uuid, organization_uuid, handle, group_id, display_name, managed_by, description, created_by, updated_by,
		       origin, configuration, openapi_spec, version, is_latest, enabled, created_at, updated_at
		FROM llm_provider_templates
		WHERE group_id = ? AND organization_uuid = ? AND version = ?
	`), groupID, orgUUID, version)
	return scanTemplateRow(row)
}

func (r *LLMProviderTemplateRepo) ListVersions(groupID, orgUUID string, limit, offset int) ([]*model.LLMProviderTemplate, error) {
	pageClause, pageArgs := r.db.PaginationClause(limit, offset)
	query := `
		SELECT uuid, organization_uuid, handle, group_id, display_name, managed_by, description, created_by, updated_by,
		       origin, configuration, openapi_spec, version, is_latest, enabled, created_at, updated_at
		FROM llm_provider_templates
		WHERE group_id = ? AND organization_uuid = ?
		ORDER BY created_at DESC
		` + pageClause
	rows, err := r.db.Query(r.db.Rebind(query), append([]any{groupID, orgUUID}, pageArgs...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*model.LLMProviderTemplate
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, t)
	}
	return res, rows.Err()
}

func (r *LLMProviderTemplateRepo) CountVersions(groupID, orgUUID string) (int, error) {
	var count int
	if err := r.db.QueryRow(r.db.Rebind(`SELECT COUNT(*) FROM llm_provider_templates WHERE group_id = ? AND organization_uuid = ?`), groupID, orgUUID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func scanTemplate(s rowScanner) (*model.LLMProviderTemplate, error) {
	var t model.LLMProviderTemplate
	var configJSON []byte
	var openapiSpec []byte
	var isLatest, enabled int
	if err := s.Scan(
		&t.UUID, &t.OrganizationUUID, &t.ID, &t.GroupID, &t.Name, &t.ManagedBy, &t.Description, &t.CreatedBy, &t.UpdatedBy,
		&t.Origin, &configJSON, &openapiSpec, &t.Version, &isLatest, &enabled,
		&t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		return nil, err
	}
	t.IsLatest = isLatest != 0
	t.Enabled = enabled != 0
	t.OpenAPISpec = string(openapiSpec)
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

func scanTemplateRow(row *sql.Row) (*model.LLMProviderTemplate, error) {
	t, err := scanTemplate(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return t, nil
}

func (r *LLMProviderTemplateRepo) List(orgUUID string, limit, offset int) ([]*model.LLMProviderTemplate, error) {
	pageClause, pageArgs := r.db.PaginationClause(limit, offset)
	query := `
		SELECT uuid, organization_uuid, handle, group_id, display_name, managed_by, description, created_by, updated_by,
		       origin, configuration, openapi_spec, version, is_latest, enabled, created_at, updated_at
		FROM llm_provider_templates t
		WHERE organization_uuid = ?
		  AND NOT EXISTS (
		    SELECT 1 FROM llm_provider_templates t2
		    WHERE t2.organization_uuid = t.organization_uuid
		      AND t2.group_id = t.group_id
		      AND (CASE WHEN t2.managed_by = 'wso2' THEN 1 ELSE 0 END) = (CASE WHEN t.managed_by = 'wso2' THEN 1 ELSE 0 END)
		      AND (t2.created_at > t.created_at OR (t2.created_at = t.created_at AND t2.uuid > t.uuid))
		  )
		ORDER BY created_at DESC
		` + pageClause
	rows, err := r.db.Query(r.db.Rebind(query), append([]any{orgUUID}, pageArgs...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*model.LLMProviderTemplate
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, t)
	}
	return res, rows.Err()
}

func (r *LLMProviderTemplateRepo) ListAllVersions(orgUUID string, limit, offset int) ([]*model.LLMProviderTemplate, error) {
	pageClause, pageArgs := r.db.PaginationClause(limit, offset)
	query := `
		SELECT uuid, organization_uuid, handle, group_id, display_name, managed_by, description, created_by, updated_by,
		       origin, configuration, openapi_spec, version, is_latest, enabled, created_at, updated_at
		FROM llm_provider_templates
		WHERE organization_uuid = ?
		ORDER BY display_name ASC, created_at DESC
		` + pageClause
	rows, err := r.db.Query(r.db.Rebind(query), append([]any{orgUUID}, pageArgs...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*model.LLMProviderTemplate
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, t)
	}
	return res, rows.Err()
}

func (r *LLMProviderTemplateRepo) CountAllVersions(orgUUID string) (int, error) {
	var count int
	if err := r.db.QueryRow(r.db.Rebind(`SELECT COUNT(*) FROM llm_provider_templates WHERE organization_uuid = ?`), orgUUID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *LLMProviderTemplateRepo) Update(t *model.LLMProviderTemplate) error {
	// Preserve a caller-provided updated_at: the DP->CP import sets it to the gateway deployment
	// time (UTC) so it acts as the last-in-wins watermark. Default to now for control-plane-native
	// updates that leave it unset.
	if t.UpdatedAt.IsZero() {
		t.UpdatedAt = time.Now()
	}

	configJSON, err := json.Marshal(&llmProviderTemplateConfig{
		ManagedBy:        t.ManagedBy,
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

	result, err := r.db.Exec(r.db.Rebind(`
		UPDATE llm_provider_templates
		SET display_name = ?, managed_by = ?, description = ?, configuration = ?, openapi_spec = ?, updated_by = ?, updated_at = ?
		WHERE handle = ? AND organization_uuid = ?
	`),
		t.Name, t.ManagedBy, t.Description, configJSON, []byte(t.OpenAPISpec), t.UpdatedBy, t.UpdatedAt,
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

func (r *LLMProviderTemplateRepo) RenameFamily(baseHandle, orgUUID, name string) error {
	_, err := r.db.Exec(r.db.Rebind(`
		UPDATE llm_provider_templates SET display_name = ?, updated_at = ?
		WHERE group_id = ? AND organization_uuid = ? AND managed_by != ?
	`), name, time.Now(), baseHandle, orgUUID, "wso2")
	return err
}

func (r *LLMProviderTemplateRepo) SetEnabled(groupID, orgUUID, version string, enabled bool) error {
	result, err := r.db.Exec(r.db.Rebind(`
		UPDATE llm_provider_templates SET enabled = ?, updated_at = ?
		WHERE group_id = ? AND organization_uuid = ? AND version = ?
	`), boolToInt(enabled), time.Now(), groupID, orgUUID, version)
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

func (r *LLMProviderTemplateRepo) DeleteVersion(groupID, orgUUID, version string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.Exec(r.db.Rebind(`
		DELETE FROM llm_provider_templates
		WHERE group_id = ? AND organization_uuid = ? AND version = ?
	`), groupID, orgUUID, version)
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

	var remaining, latestCount int
	if err := tx.QueryRow(r.db.Rebind(`
		SELECT COUNT(*), COALESCE(SUM(CASE WHEN is_latest = 1 THEN 1 ELSE 0 END), 0)
		FROM llm_provider_templates WHERE group_id = ? AND organization_uuid = ?
	`), groupID, orgUUID).Scan(&remaining, &latestCount); err != nil {
		return err
	}
	if remaining > 0 && latestCount == 0 {
		if _, err := tx.Exec(r.db.Rebind(`
			UPDATE llm_provider_templates SET is_latest = ?
			WHERE uuid = (
				SELECT uuid FROM llm_provider_templates
				WHERE group_id = ? AND organization_uuid = ?
				ORDER BY created_at DESC `+r.db.FetchFirstClause(1)+`
			)
		`), 1, groupID, orgUUID); err != nil {
			return err
		}
	}
	return tx.Commit()
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
	if err := r.db.QueryRow(r.db.Rebind(`
		SELECT COUNT(*) FROM (
		    SELECT group_id
		    FROM llm_provider_templates
		    WHERE organization_uuid = ?
		    GROUP BY group_id, CASE WHEN managed_by = 'wso2' THEN 1 ELSE 0 END
		) AS sub
	`), orgUUID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}
func (r *LLMProviderTemplateRepo) CountProvidersUsingTemplate(groupID, orgUUID, version string) (int, error) {
	if groupID == "" {
		return 0, nil
	}
	query := `
		SELECT COUNT(*)
		FROM llm_providers p
		JOIN llm_provider_templates t ON p.template_uuid = t.uuid
		WHERE t.group_id = ? AND t.organization_uuid = ?`
	args := []interface{}{groupID, orgUUID}
	if strings.TrimSpace(version) != "" {
		query += ` AND t.version = ?`
		args = append(args, version)
	}
	var count int
	if err := r.db.QueryRow(r.db.Rebind(query), args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// insertArtifactGatewayAssociations writes the given gateway associations for an artifact
// within the supplied transaction. metadata is a BYTEA column; a nil slice is stored as NULL.
func insertArtifactGatewayAssociations(tx *sql.Tx, db *database.DB, artifactUUID, orgUUID string, assocs []model.AssociatedGatewayMapping, now time.Time) error {
	if len(assocs) == 0 {
		return nil
	}
	query := `
		INSERT INTO artifact_gateway_mappings (
			artifact_uuid, organization_uuid, gateway_uuid, metadata, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?)`
	for _, assoc := range assocs {
		var metadata []byte
		if assoc.Metadata != "" {
			metadata = []byte(assoc.Metadata)
		}
		if _, err := tx.Exec(db.Rebind(query), artifactUUID, orgUUID, assoc.GatewayUUID, metadata, now, now); err != nil {
			return fmt.Errorf("failed to create gateway association: %w", err)
		}
	}
	return nil
}

// replaceArtifactGatewayAssociations replaces the full set of gateway associations for an
// artifact within the supplied transaction (delete-all then insert). Deployments are not touched.
func replaceArtifactGatewayAssociations(tx *sql.Tx, db *database.DB, artifactUUID, orgUUID string, assocs []model.AssociatedGatewayMapping, now time.Time) error {
	if _, err := tx.Exec(db.Rebind(
		`DELETE FROM artifact_gateway_mappings WHERE artifact_uuid = ? AND organization_uuid = ?`),
		artifactUUID, orgUUID); err != nil {
		return fmt.Errorf("failed to clear gateway associations: %w", err)
	}
	return insertArtifactGatewayAssociations(tx, db, artifactUUID, orgUUID, assocs, now)
}

// loadArtifactGatewayAssociations returns the gateway associations for an artifact, joining
// the gateways table to resolve each gateway handle.
func loadArtifactGatewayAssociations(db *database.DB, artifactUUID, orgUUID string) ([]model.AssociatedGatewayMapping, error) {
	query := `
		SELECT m.gateway_uuid, g.handle, m.metadata
		FROM artifact_gateway_mappings m
		JOIN gateways g ON g.uuid = m.gateway_uuid
		WHERE m.artifact_uuid = ? AND m.organization_uuid = ?
		ORDER BY m.created_at, m.gateway_uuid`
	rows, err := db.Query(db.Rebind(query), artifactUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var associations []model.AssociatedGatewayMapping
	for rows.Next() {
		var assoc model.AssociatedGatewayMapping
		var metadata []byte
		if err := rows.Scan(&assoc.GatewayUUID, &assoc.GatewayHandle, &metadata); err != nil {
			return nil, err
		}
		if len(metadata) > 0 {
			assoc.Metadata = string(metadata)
		}
		associations = append(associations, assoc)
	}
	return associations, rows.Err()
}

// ensureArtifactGatewayAssociation creates a gateway association for an artifact if one does
// not already exist and resolves the metadata to use for the deployment.
//
// metadataProvided distinguishes an omitted deploy metadata field from one explicitly set to
// empty, so a caller can deploy with empty metadata even when the association already carries
// a value. An association that already exists is never modified at deploy time:
//   - No association yet → create it, seeding its metadata from deployMetadata.
//   - Association exists, metadataProvided → use deployMetadata for the deployment (including
//     an explicit empty value); the association is left untouched.
//   - Association exists, metadata omitted → fall back to the association's stored metadata.
//
// It returns the metadata to persist on the deployment record. An empty string means "no metadata".
func ensureArtifactGatewayAssociation(db *database.DB, artifactUUID, gatewayUUID, orgUUID, deployMetadata string, metadataProvided bool) (string, error) {
	effectiveMetadata := func(existing []byte) string {
		if metadataProvided {
			return deployMetadata
		}
		if len(existing) > 0 {
			return string(existing)
		}
		return ""
	}

	existing, found, err := readArtifactGatewayAssociation(db, artifactUUID, gatewayUUID, orgUUID)
	if err != nil {
		return "", err
	}
	if found {
		// Existing association is never modified at deploy time.
		return effectiveMetadata(existing), nil
	}

	// No association yet → insert one, seeding its metadata from the deploy request.
	now := time.Now()
	var metaArg []byte
	if strings.TrimSpace(deployMetadata) != "" {
		metaArg = []byte(deployMetadata)
	}
	insertQuery := `
		INSERT INTO artifact_gateway_mappings (
			artifact_uuid, organization_uuid, gateway_uuid, metadata, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?)`
	if _, err := db.Exec(db.Rebind(insertQuery), artifactUUID, orgUUID, gatewayUUID, metaArg, now, now); err != nil {
		// A concurrent deploy for the same artifact/gateway may have inserted the row
		// between our read and this insert, tripping the primary key. Re-read: if the row
		// now exists the ensure has effectively succeeded (idempotent); otherwise the
		// failure is genuine and we surface the original error.
		existing, found, rereadErr := readArtifactGatewayAssociation(db, artifactUUID, gatewayUUID, orgUUID)
		if rereadErr == nil && found {
			return effectiveMetadata(existing), nil
		}
		return "", fmt.Errorf("failed to create gateway association: %w", err)
	}
	return deployMetadata, nil
}

// readArtifactGatewayAssociation returns the stored metadata for an artifact/gateway
// association and whether the association exists.
func readArtifactGatewayAssociation(db *database.DB, artifactUUID, gatewayUUID, orgUUID string) (metadata []byte, found bool, err error) {
	query := `
		SELECT metadata
		FROM artifact_gateway_mappings
		WHERE artifact_uuid = ? AND gateway_uuid = ? AND organization_uuid = ?`
	err = db.QueryRow(db.Rebind(query), artifactUUID, gatewayUUID, orgUUID).Scan(&metadata)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("failed to check gateway association: %w", err)
	}
	return metadata, true, nil
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

	origin := p.Origin
	if origin == "" {
		origin = constants.OriginCP
	}

	// Insert into llm_providers table (handle/name/version/timestamps now live here)
	query := `
		INSERT INTO llm_providers (
			uuid, handle, display_name, version, description, created_by, template_uuid, openapi_spec, model_list,
		                           configuration, origin, created_at, updated_at, organization_uuid
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = tx.Exec(r.db.Rebind(query),
		p.UUID, p.ID, p.Name, p.Version, p.Description, p.CreatedBy, p.TemplateUUID,
		[]byte(p.OpenAPISpec), modelProvidersJSON, configurationJSON, origin, p.CreatedAt, p.UpdatedAt,
		p.OrganizationUUID,
	)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	if err := upsertArtifactSecretRefs(tx, r.db, p.OrganizationUUID, p.UUID, []byte(configurationJSON)); err != nil {
		return fmt.Errorf("failed to upsert artifact secret refs: %w", err)
	}

	// Persist gateway associations (if any) within the same transaction.
	if err := insertArtifactGatewayAssociations(tx, r.db, p.UUID, p.OrganizationUUID, p.AssociatedGateways, now); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (r *LLMProviderRepo) GetByID(providerID, orgUUID string) (*model.LLMProvider, error) {
	query := `
		SELECT
			uuid, handle, display_name, version, organization_uuid, origin, created_at, updated_at,
			description, created_by, template_uuid, openapi_spec, model_list, configuration
		FROM llm_providers
		WHERE handle = ? AND organization_uuid = ?`
	row := r.db.QueryRow(r.db.Rebind(query), providerID, orgUUID)

	var p model.LLMProvider
	var createdBy sql.NullString
	var openAPISpec, modelProvidersRaw []byte
	var configurationJSON []byte
	if err := row.Scan(
		&p.UUID, &p.ID, &p.Name, &p.Version, &p.OrganizationUUID, &p.Origin, &p.CreatedAt, &p.UpdatedAt,
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

	associations, err := loadArtifactGatewayAssociations(r.db, p.UUID, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to load gateway associations for provider %s: %w", p.ID, err)
	}
	p.AssociatedGateways = associations

	return &p, nil
}

// EnsureGatewayAssociation creates a gateway association for the provider if one does not
// already exist and resolves the metadata to use for the deployment. See
// ensureArtifactGatewayAssociation for the full semantics.
func (r *LLMProviderRepo) EnsureGatewayAssociation(providerUUID, gatewayUUID, orgUUID, deployMetadata string, metadataProvided bool) (string, error) {
	return ensureArtifactGatewayAssociation(r.db, providerUUID, gatewayUUID, orgUUID, deployMetadata, metadataProvided)
}

func (r *LLMProviderRepo) List(orgUUID string, limit, offset int) ([]*model.LLMProvider, error) {
	pageClause, pageArgs := r.db.PaginationClause(limit, offset)
	args := append([]any{orgUUID}, pageArgs...)
	query := `
		SELECT
			uuid, handle, display_name, version, organization_uuid, origin, created_at, updated_at,
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
			&p.UUID, &p.ID, &p.Name, &p.Version, &p.OrganizationUUID, &p.Origin, &p.CreatedAt, &p.UpdatedAt,
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
		SET display_name = ?, version = ?, description = ?, template_uuid = ?, openapi_spec = ?, model_list = ?, configuration = ?, updated_by = ?, updated_at = ?
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

	if err := upsertArtifactSecretRefs(tx, r.db, p.OrganizationUUID, providerUUID, []byte(configurationJSON)); err != nil {
		return fmt.Errorf("failed to upsert artifact secret refs: %w", err)
	}

	// Replace the full set of gateway associations within the same transaction when the
	// caller manages associations. Deployments are intentionally left untouched.
	if p.ReplaceAssociatedGateways {
		if err := replaceArtifactGatewayAssociations(tx, r.db, providerUUID, p.OrganizationUUID, p.AssociatedGateways, now); err != nil {
			return err
		}
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

	origin := p.Origin
	if origin == "" {
		origin = constants.OriginCP
	}

	// Insert into llm_proxies table (handle/name/version/timestamps now live here)
	query := `
		INSERT INTO llm_proxies (
			uuid, handle, display_name, version, project_uuid, description, created_by, provider_uuid, openapi_spec,
		                         configuration, origin, created_at, updated_at, organization_uuid
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = tx.Exec(r.db.Rebind(query),
		p.UUID, p.ID, p.Name, p.Version, p.ProjectUUID, p.Description, p.CreatedBy, p.ProviderUUID,
		[]byte(p.OpenAPISpec), configurationJSON, origin, p.CreatedAt, p.UpdatedAt,
		p.OrganizationUUID,
	)
	if err != nil {
		return fmt.Errorf("failed to create proxy: %w", err)
	}

	if err := upsertArtifactSecretRefs(tx, r.db, p.OrganizationUUID, p.UUID, []byte(configurationJSON)); err != nil {
		return fmt.Errorf("failed to upsert artifact secret refs: %w", err)
	}

	// Persist gateway associations (if any) within the same transaction.
	if err := insertArtifactGatewayAssociations(tx, r.db, p.UUID, p.OrganizationUUID, p.AssociatedGateways, now); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (r *LLMProxyRepo) GetByID(proxyID, orgUUID string) (*model.LLMProxy, error) {
	query := `
		SELECT
			uuid, handle, display_name, version, organization_uuid, origin, created_at, updated_at,
			project_uuid, description, created_by, provider_uuid, openapi_spec, configuration
		FROM llm_proxies
		WHERE handle = ? AND organization_uuid = ?`
	row := r.db.QueryRow(r.db.Rebind(query), proxyID, orgUUID)

	var p model.LLMProxy
	var createdBy sql.NullString
	var openAPISpec, configurationJSON []byte
	if err := row.Scan(
		&p.UUID, &p.ID, &p.Name, &p.Version, &p.OrganizationUUID, &p.Origin, &p.CreatedAt, &p.UpdatedAt,
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

	associations, err := loadArtifactGatewayAssociations(r.db, p.UUID, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to load gateway associations for proxy %s: %w", p.ID, err)
	}
	p.AssociatedGateways = associations

	return &p, nil
}

func (r *LLMProxyRepo) List(orgUUID string, limit, offset int) ([]*model.LLMProxy, error) {
	pageClause, pageArgs := r.db.PaginationClause(limit, offset)
	args := append([]any{orgUUID}, pageArgs...)
	query := `
		SELECT
			uuid, handle, display_name, version, organization_uuid, origin, created_at, updated_at,
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
			&p.UUID, &p.ID, &p.Name, &p.Version, &p.OrganizationUUID, &p.Origin, &p.CreatedAt, &p.UpdatedAt,
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
			uuid, handle, display_name, version, organization_uuid, origin, created_at, updated_at,
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
			&p.UUID, &p.ID, &p.Name, &p.Version, &p.OrganizationUUID, &p.Origin, &p.CreatedAt, &p.UpdatedAt,
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
			uuid, handle, display_name, version, organization_uuid, origin, created_at, updated_at,
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
			&p.UUID, &p.ID, &p.Name, &p.Version, &p.OrganizationUUID, &p.Origin, &p.CreatedAt, &p.UpdatedAt,
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
		SET display_name = ?, version = ?, description = ?, provider_uuid = ?,
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

	if err := upsertArtifactSecretRefs(tx, r.db, p.OrganizationUUID, proxyUUID, []byte(configurationJSON)); err != nil {
		return fmt.Errorf("failed to upsert artifact secret refs: %w", err)
	}

	// Replace the full set of gateway associations within the same transaction when the
	// caller manages associations. Deployments are intentionally left untouched.
	if p.ReplaceAssociatedGateways {
		if err := replaceArtifactGatewayAssociations(tx, r.db, proxyUUID, p.OrganizationUUID, p.AssociatedGateways, now); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

// EnsureGatewayAssociation creates a gateway association for the proxy if one does not
// already exist and resolves the metadata to use for the deployment. See
// ensureArtifactGatewayAssociation for the full semantics.
func (r *LLMProxyRepo) EnsureGatewayAssociation(proxyUUID, gatewayUUID, orgUUID, deployMetadata string, metadataProvided bool) (string, error) {
	return ensureArtifactGatewayAssociation(r.db, proxyUUID, gatewayUUID, orgUUID, deployMetadata, metadataProvided)
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
