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

package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/model"

	"github.com/google/uuid"
)

// PublicationRepo implements PublicationRepository against api_publications
// and its satellite tables (api_publication_contents,
// api_publication_doc_mappings, api_publication_plan_mappings).
type PublicationRepo struct {
	db *database.DB
}

// NewPublicationRepo creates a new API Publication repository.
func NewPublicationRepo(db *database.DB) PublicationRepository {
	return &PublicationRepo{db: db}
}

// sqlExecutor is satisfied by both *database.DB (via its embedded *sql.DB) and
// *sql.Tx, so the helpers below work identically whether called standalone or
// mid-transaction.
type sqlExecutor interface {
	QueryRow(query string, args ...any) *sql.Row
	Query(query string, args ...any) (*sql.Rows, error)
	Exec(query string, args ...any) (sql.Result, error)
}

const publicationDetailColumns = `
	uuid, status, display_name, version, description, tags, labels, agent_visibility,
	production_url, sandbox_url, business_owner, business_owner_email, technical_owner, technical_owner_email,
	created_by, created_at, updated_by, updated_at
`

// scanPublicationDetails scans a row selected via publicationDetailColumns into pub.
// OrganizationUUID/ArtifactUUID/APIPortalUUID/IsDraft are not selected by that column
// list (the caller already knows them — they're the lookup key) and must be set by
// the caller afterward.
func scanPublicationDetails(row rowScanner, pub *model.Publication) error {
	var status, description, prodURL, sandboxURL sql.NullString
	var businessOwner, businessOwnerEmail, technicalOwner, technicalOwnerEmail sql.NullString
	var createdBy, updatedBy sql.NullString
	var tagsBytes, labelsBytes []byte
	if err := row.Scan(
		&pub.UUID, &status, &pub.DisplayName, &pub.Version, &description, &tagsBytes, &labelsBytes, &pub.AgentVisibility,
		&prodURL, &sandboxURL, &businessOwner, &businessOwnerEmail, &technicalOwner, &technicalOwnerEmail,
		&createdBy, &pub.CreatedAt, &updatedBy, &pub.UpdatedAt,
	); err != nil {
		return err
	}
	pub.Status = status.String
	pub.Description = description.String
	pub.ProductionURL = prodURL.String
	pub.SandboxURL = sandboxURL.String
	pub.BusinessOwner = businessOwner.String
	pub.BusinessOwnerEmail = businessOwnerEmail.String
	pub.TechnicalOwner = technicalOwner.String
	pub.TechnicalOwnerEmail = technicalOwnerEmail.String
	pub.CreatedBy = createdBy.String
	pub.UpdatedBy = updatedBy.String
	if len(tagsBytes) > 0 {
		if err := json.Unmarshal(tagsBytes, &pub.Tags); err != nil {
			return fmt.Errorf("failed to unmarshal publication tags: %w", err)
		}
	}
	if len(labelsBytes) > 0 {
		if err := json.Unmarshal(labelsBytes, &pub.Labels); err != nil {
			return fmt.Errorf("failed to unmarshal publication labels: %w", err)
		}
	}
	return nil
}

// marshalStringSlice serializes a string slice for a nullable BYTEA/BLOB column,
// returning nil (NULL) for an empty slice rather than storing "[]" or "null".
func marshalStringSlice(s []string) ([]byte, error) {
	if len(s) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal string slice: %w", err)
	}
	return b, nil
}

// nullIfEmpty maps an empty string to a driver NULL rather than storing "".
func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// contentFlags reports whether a thumbnail (IMAGE) and/or landing page
// (MARKETING) content row exists for publicationUUID.
func (r *PublicationRepo) contentFlags(exec sqlExecutor, publicationUUID, orgUUID string) (hasThumbnail, hasLandingPage bool, err error) {
	query := `
		SELECT type FROM api_publication_contents WHERE publication_uuid = ? AND organization_uuid = ?
	`
	rows, err := exec.Query(r.db.Rebind(query), publicationUUID, orgUUID)
	if err != nil {
		return false, false, fmt.Errorf("failed to load publication content flags: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return false, false, fmt.Errorf("failed to scan publication content type: %w", err)
		}
		switch model.PublicationContentType(t) {
		case model.PublicationContentTypeImage:
			hasThumbnail = true
		case model.PublicationContentTypeMarketing:
			hasLandingPage = true
		}
	}
	return hasThumbnail, hasLandingPage, rows.Err()
}

// planUUIDsForPublication returns the subscription_plan_uuid values mapped to
// publicationUUID, ordered for deterministic responses.
func (r *PublicationRepo) planUUIDsForPublication(exec sqlExecutor, publicationUUID, orgUUID string) ([]string, error) {
	query := `
		SELECT subscription_plan_uuid FROM api_publication_plan_mappings
		WHERE publication_uuid = ? AND organization_uuid = ?
		ORDER BY subscription_plan_uuid
	`
	rows, err := exec.Query(r.db.Rebind(query), publicationUUID, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to load publication plan mappings: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan publication plan mapping: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// docUUIDsForPublication returns the doc_uuid values mapped to publicationUUID,
// ordered for deterministic responses.
func (r *PublicationRepo) docUUIDsForPublication(exec sqlExecutor, publicationUUID, orgUUID string) ([]string, error) {
	query := `
		SELECT doc_uuid FROM api_publication_doc_mappings
		WHERE publication_uuid = ? AND organization_uuid = ?
		ORDER BY doc_uuid
	`
	rows, err := exec.Query(r.db.Rebind(query), publicationUUID, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to load publication document mappings: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan publication document mapping: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetDraft returns the draft row for (artifactUUID, apiPortalUUID, orgUUID),
// plus its raw plan/document UUIDs. Returns (nil, nil, nil, nil) when no draft
// has been saved.
func (r *PublicationRepo) GetDraft(artifactUUID, apiPortalUUID, orgUUID string) (*model.Publication, []string, []string, error) {
	return r.getPublicationRow(artifactUUID, apiPortalUUID, orgUUID, true)
}

// GetPublication returns the live (is_draft = 0) row for (artifactUUID,
// apiPortalUUID, orgUUID), plus its raw plan/document UUIDs. Returns
// (nil, nil, nil, nil) when this API is not published to this portal.
func (r *PublicationRepo) GetPublication(artifactUUID, apiPortalUUID, orgUUID string) (*model.Publication, []string, []string, error) {
	return r.getPublicationRow(artifactUUID, apiPortalUUID, orgUUID, false)
}

// getPublicationRow loads either tier of api_publications — the draft
// (isDraft true) or the live listing (isDraft false), which differ only in
// that discriminator. Returns (nil, nil, nil, nil) when no such row exists.
func (r *PublicationRepo) getPublicationRow(artifactUUID, apiPortalUUID, orgUUID string, isDraft bool) (*model.Publication, []string, []string, error) {
	query := `SELECT ` + publicationDetailColumns + `
		FROM api_publications
		WHERE organization_uuid = ? AND artifact_uuid = ? AND api_portal_uuid = ? AND is_draft = ?`

	pub := &model.Publication{
		OrganizationUUID: orgUUID,
		ArtifactUUID:     artifactUUID,
		APIPortalUUID:    apiPortalUUID,
		IsDraft:          isDraft,
	}
	row := r.db.QueryRow(r.db.Rebind(query), orgUUID, artifactUUID, apiPortalUUID, boolToInt(isDraft))
	if err := scanPublicationDetails(row, pub); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, nil, nil
		}
		return nil, nil, nil, fmt.Errorf("failed to get publication row: %w", err)
	}

	planUUIDs, err := r.planUUIDsForPublication(r.db, pub.UUID, orgUUID)
	if err != nil {
		return nil, nil, nil, err
	}
	docUUIDs, err := r.docUUIDsForPublication(r.db, pub.UUID, orgUUID)
	if err != nil {
		return nil, nil, nil, err
	}
	hasThumbnail, hasLandingPage, err := r.contentFlags(r.db, pub.UUID, orgUUID)
	if err != nil {
		return nil, nil, nil, err
	}
	pub.HasThumbnail = hasThumbnail
	pub.HasLandingPage = hasLandingPage

	return pub, planUUIDs, docUUIDs, nil
}

// ListStatusByArtifact returns every api_publications row (draft and/or
// live) for artifactUUID, reduced to the portal UUID, tier, status and
// updated_at the GET /api-publications rollup needs.
func (r *PublicationRepo) ListStatusByArtifact(artifactUUID, orgUUID string) ([]*model.PublicationStatusRow, error) {
	query := `
		SELECT api_portal_uuid, is_draft, status, updated_at
		FROM api_publications
		WHERE organization_uuid = ? AND artifact_uuid = ?
	`
	rows, err := r.db.Query(r.db.Rebind(query), orgUUID, artifactUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to list publication status rows: %w", err)
	}
	defer rows.Close()

	var result []*model.PublicationStatusRow
	for rows.Next() {
		row := &model.PublicationStatusRow{}
		var isDraft int
		var status sql.NullString
		if err := rows.Scan(&row.APIPortalUUID, &isDraft, &status, &row.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan publication status row: %w", err)
		}
		row.IsDraft = isDraft != 0
		row.Status = status.String
		result = append(result, row)
	}
	return result, rows.Err()
}

// SaveDraftDetails creates the draft row on first save, or replaces an
// existing one in full, together with its plan/document mapping rows.
func (r *PublicationRepo) SaveDraftDetails(pub *model.Publication, planUUIDs []string, docUUIDs []string, actor string) (*model.Publication, error) {
	now := time.Now().UTC()

	tagsBytes, err := marshalStringSlice(pub.Tags)
	if err != nil {
		return nil, err
	}
	labelsBytes, err := marshalStringSlice(pub.Labels)
	if err != nil {
		return nil, err
	}

	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var existingUUID string
	var createdBy string
	var createdAt time.Time
	lookupQuery := `
		SELECT uuid, created_by, created_at FROM api_publications
		WHERE organization_uuid = ? AND artifact_uuid = ? AND api_portal_uuid = ? AND is_draft = 1
	`
	lookupErr := tx.QueryRow(r.db.Rebind(lookupQuery), pub.OrganizationUUID, pub.ArtifactUUID, pub.APIPortalUUID).Scan(&existingUUID, &createdBy, &createdAt)

	switch {
	case errors.Is(lookupErr, sql.ErrNoRows):
		pub.UUID = uuid.New().String()
		pub.CreatedBy = actor
		pub.CreatedAt = now
		pub.UpdatedBy = actor
		pub.UpdatedAt = now
		pub.DataVersion = "1.0"
		insertQuery := `
			INSERT INTO api_publications (
				uuid, organization_uuid, artifact_uuid, api_portal_uuid, is_draft,
				display_name, version, description, tags, labels, agent_visibility,
				production_url, sandbox_url, business_owner, business_owner_email,
				technical_owner, technical_owner_email, data_version,
				created_by, created_at, updated_by, updated_at
			) VALUES (?, ?, ?, ?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`
		_, err = tx.Exec(r.db.Rebind(insertQuery),
			pub.UUID, pub.OrganizationUUID, pub.ArtifactUUID, pub.APIPortalUUID,
			pub.DisplayName, pub.Version, nullIfEmpty(pub.Description), tagsBytes, labelsBytes, pub.AgentVisibility,
			nullIfEmpty(pub.ProductionURL), nullIfEmpty(pub.SandboxURL), nullIfEmpty(pub.BusinessOwner), nullIfEmpty(pub.BusinessOwnerEmail),
			nullIfEmpty(pub.TechnicalOwner), nullIfEmpty(pub.TechnicalOwnerEmail), pub.DataVersion,
			pub.CreatedBy, pub.CreatedAt, pub.UpdatedBy, pub.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to insert publication draft: %w", err)
		}
	case lookupErr != nil:
		return nil, fmt.Errorf("failed to look up existing publication draft: %w", lookupErr)
	default:
		pub.UUID = existingUUID
		pub.CreatedBy = createdBy
		pub.CreatedAt = createdAt
		pub.UpdatedBy = actor
		pub.UpdatedAt = now
		updateQuery := `
			UPDATE api_publications SET
				display_name = ?, version = ?, description = ?, tags = ?, labels = ?, agent_visibility = ?,
				production_url = ?, sandbox_url = ?, business_owner = ?, business_owner_email = ?,
				technical_owner = ?, technical_owner_email = ?, updated_by = ?, updated_at = ?
			WHERE uuid = ? AND organization_uuid = ?
		`
		_, err = tx.Exec(r.db.Rebind(updateQuery),
			pub.DisplayName, pub.Version, nullIfEmpty(pub.Description), tagsBytes, labelsBytes, pub.AgentVisibility,
			nullIfEmpty(pub.ProductionURL), nullIfEmpty(pub.SandboxURL), nullIfEmpty(pub.BusinessOwner), nullIfEmpty(pub.BusinessOwnerEmail),
			nullIfEmpty(pub.TechnicalOwner), nullIfEmpty(pub.TechnicalOwnerEmail), pub.UpdatedBy, pub.UpdatedAt,
			pub.UUID, pub.OrganizationUUID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to update publication draft: %w", err)
		}
	}

	deletePlanMappingsQuery := `
		DELETE FROM api_publication_plan_mappings WHERE organization_uuid = ? AND publication_uuid = ?
	`
	if _, err := tx.Exec(r.db.Rebind(deletePlanMappingsQuery), pub.OrganizationUUID, pub.UUID); err != nil {
		return nil, fmt.Errorf("failed to clear publication plan mappings: %w", err)
	}
	for _, planUUID := range planUUIDs {
		insertPlanMappingQuery := `
			INSERT INTO api_publication_plan_mappings (organization_uuid, publication_uuid, subscription_plan_uuid, created_by, created_at)
			VALUES (?, ?, ?, ?, ?)
		`
		if _, err := tx.Exec(r.db.Rebind(insertPlanMappingQuery), pub.OrganizationUUID, pub.UUID, planUUID, actor, now); err != nil {
			return nil, fmt.Errorf("failed to insert publication plan mapping: %w", err)
		}
	}

	deleteDocMappingsQuery := `
		DELETE FROM api_publication_doc_mappings WHERE organization_uuid = ? AND publication_uuid = ?
	`
	if _, err := tx.Exec(r.db.Rebind(deleteDocMappingsQuery), pub.OrganizationUUID, pub.UUID); err != nil {
		return nil, fmt.Errorf("failed to clear publication document mappings: %w", err)
	}
	for _, docUUID := range docUUIDs {
		insertDocMappingQuery := `
			INSERT INTO api_publication_doc_mappings (organization_uuid, publication_uuid, doc_uuid, created_by, created_at)
			VALUES (?, ?, ?, ?, ?)
		`
		if _, err := tx.Exec(r.db.Rebind(insertDocMappingQuery), pub.OrganizationUUID, pub.UUID, docUUID, actor, now); err != nil {
			return nil, fmt.Errorf("failed to insert publication document mapping: %w", err)
		}
	}

	hasThumbnail, hasLandingPage, err := r.contentFlags(tx, pub.UUID, pub.OrganizationUUID)
	if err != nil {
		return nil, err
	}
	pub.HasThumbnail = hasThumbnail
	pub.HasLandingPage = hasLandingPage

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit publication draft save: %w", err)
	}
	return pub, nil
}

// mergeDraftIntoAnchor merges draftUUID into anchorUUID so the anchor's own
// uuid — the durable identity for this (artifact, api_portal) pairing,
// pinned to whichever row was first published — survives the transition
// instead of the draft's. It copies the draft's business/content columns
// onto the anchor row (never its uuid/organization_uuid/artifact_uuid/
// api_portal_uuid/created_by/created_at/data_version — those stay the
// anchor's own) and sets is_draft/status/updated_by/updated_at on that same
// row; re-parents the draft's satellite rows (api_publication_contents/
// _doc_mappings/_plan_mappings) onto the anchor, clearing the anchor's own
// stale satellite rows first, since the anchor row itself survives and so
// nothing cascades them away; and deletes the now-empty draft row.
//
// Ordering matters here: the draft row must be deleted, and its satellites
// re-parented, BEFORE the anchor's own is_draft is updated — api_publications
// has UNIQUE(organization_uuid, artifact_uuid, api_portal_uuid, is_draft), so
// flipping the anchor to the draft's is_draft value first (e.g. Unpublish's
// merge branch, anchor 0 -> 1) would collide with the not-yet-deleted draft
// row that already holds that value. Re-parenting the satellites before
// deleting the draft row (rather than after) is equally load-bearing the
// other way: the FK from each satellite table to api_publications is ON
// DELETE CASCADE, so deleting the draft row first would cascade-delete its
// satellite rows before they could be moved onto the anchor.
//
// Callers (PromoteDraftToPublication, UnpublishPublication) run this inside
// their own transaction; anchorUUID and draftUUID must both already be known
// to exist for orgUUID.
func (r *PublicationRepo) mergeDraftIntoAnchor(tx *sql.Tx, anchorUUID, draftUUID, orgUUID string, isDraft int, status interface{}, actor string, now time.Time) error {
	var displayName, version, agentVisibility string
	var description, prodURL, sandboxURL sql.NullString
	var businessOwner, businessOwnerEmail, technicalOwner, technicalOwnerEmail sql.NullString
	var tagsBytes, labelsBytes []byte
	draftQuery := `
		SELECT display_name, version, description, tags, labels, agent_visibility,
			production_url, sandbox_url, business_owner, business_owner_email,
			technical_owner, technical_owner_email
		FROM api_publications WHERE uuid = ? AND organization_uuid = ?
	`
	row := tx.QueryRow(r.db.Rebind(draftQuery), draftUUID, orgUUID)
	if err := row.Scan(
		&displayName, &version, &description, &tagsBytes, &labelsBytes, &agentVisibility,
		&prodURL, &sandboxURL, &businessOwner, &businessOwnerEmail, &technicalOwner, &technicalOwnerEmail,
	); err != nil {
		return fmt.Errorf("failed to read draft publication to merge: %w", err)
	}

	deleteContentsQuery := `
		DELETE FROM api_publication_contents WHERE publication_uuid = ? AND organization_uuid = ?
	`
	if _, err := tx.Exec(r.db.Rebind(deleteContentsQuery), anchorUUID, orgUUID); err != nil {
		return fmt.Errorf("failed to clear anchor's content before merge: %w", err)
	}
	reparentContentsQuery := `
		UPDATE api_publication_contents SET publication_uuid = ? WHERE publication_uuid = ? AND organization_uuid = ?
	`
	if _, err := tx.Exec(r.db.Rebind(reparentContentsQuery), anchorUUID, draftUUID, orgUUID); err != nil {
		return fmt.Errorf("failed to reparent draft's content onto anchor: %w", err)
	}

	deleteDocMappingsQuery := `
		DELETE FROM api_publication_doc_mappings WHERE publication_uuid = ? AND organization_uuid = ?
	`
	if _, err := tx.Exec(r.db.Rebind(deleteDocMappingsQuery), anchorUUID, orgUUID); err != nil {
		return fmt.Errorf("failed to clear anchor's document mappings before merge: %w", err)
	}
	reparentDocMappingsQuery := `
		UPDATE api_publication_doc_mappings SET publication_uuid = ? WHERE publication_uuid = ? AND organization_uuid = ?
	`
	if _, err := tx.Exec(r.db.Rebind(reparentDocMappingsQuery), anchorUUID, draftUUID, orgUUID); err != nil {
		return fmt.Errorf("failed to reparent draft's document mappings onto anchor: %w", err)
	}

	deletePlanMappingsQuery := `
		DELETE FROM api_publication_plan_mappings WHERE publication_uuid = ? AND organization_uuid = ?
	`
	if _, err := tx.Exec(r.db.Rebind(deletePlanMappingsQuery), anchorUUID, orgUUID); err != nil {
		return fmt.Errorf("failed to clear anchor's plan mappings before merge: %w", err)
	}
	reparentPlanMappingsQuery := `
		UPDATE api_publication_plan_mappings SET publication_uuid = ? WHERE publication_uuid = ? AND organization_uuid = ?
	`
	if _, err := tx.Exec(r.db.Rebind(reparentPlanMappingsQuery), anchorUUID, draftUUID, orgUUID); err != nil {
		return fmt.Errorf("failed to reparent draft's plan mappings onto anchor: %w", err)
	}

	deleteDraftQuery := `
		DELETE FROM api_publications WHERE uuid = ? AND organization_uuid = ?
	`
	if _, err := tx.Exec(r.db.Rebind(deleteDraftQuery), draftUUID, orgUUID); err != nil {
		return fmt.Errorf("failed to delete merged draft publication: %w", err)
	}

	updateAnchorQuery := `
		UPDATE api_publications SET
			display_name = ?, version = ?, description = ?, tags = ?, labels = ?, agent_visibility = ?,
			production_url = ?, sandbox_url = ?, business_owner = ?, business_owner_email = ?,
			technical_owner = ?, technical_owner_email = ?, is_draft = ?, status = ?, updated_by = ?, updated_at = ?
		WHERE uuid = ? AND organization_uuid = ?
	`
	if _, err := tx.Exec(r.db.Rebind(updateAnchorQuery),
		displayName, version, description, tagsBytes, labelsBytes, agentVisibility,
		prodURL, sandboxURL, businessOwner, businessOwnerEmail, technicalOwner, technicalOwnerEmail,
		isDraft, status, actor, now,
		anchorUUID, orgUUID,
	); err != nil {
		return fmt.Errorf("failed to merge draft into anchor publication: %w", err)
	}
	return nil
}

// PromoteDraftToPublication makes the draft row for (artifactUUID,
// apiPortalUUID, orgUUID) live. The anchor for this pairing — the row whose
// uuid is the durable identity of "this API's publication on this portal" —
// is whichever row was first published:
//   - First publish: no anchor exists yet, so the draft row becomes it,
//     flipped in place (is_draft=0, status='PUBLISHED'), same uuid, no copy.
//   - Republish: an anchor already exists as the live row. The draft's
//     content is merged into that anchor row in place (mergeDraftIntoAnchor)
//     and the draft row is discarded — the anchor's uuid never changes
//     across a republish.
//
// One transaction either way. Returns (nil, false, nil) if no draft exists
// to promote — the caller (PublicationService.Publish) treats this as a
// defensive DRAFT_NOT_FOUND, not a normal outcome, since the client always
// saves the draft immediately before calling publish. replaced reports
// whether an existing live row was found (republish) versus this being the
// first publish (create) — the handler uses it to choose 200 vs 201.
func (r *PublicationRepo) PromoteDraftToPublication(artifactUUID, apiPortalUUID, orgUUID, actor string) (pub *model.Publication, replaced bool, err error) {
	now := time.Now().UTC()

	tx, err := r.db.Begin()
	if err != nil {
		return nil, false, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var draftUUID string
	draftLookupQuery := `
		SELECT uuid FROM api_publications
		WHERE organization_uuid = ? AND artifact_uuid = ? AND api_portal_uuid = ? AND is_draft = 1
	`
	draftLookupErr := tx.QueryRow(r.db.Rebind(draftLookupQuery), orgUUID, artifactUUID, apiPortalUUID).Scan(&draftUUID)
	if errors.Is(draftLookupErr, sql.ErrNoRows) {
		return nil, false, nil
	}
	if draftLookupErr != nil {
		return nil, false, fmt.Errorf("failed to look up publication draft to promote: %w", draftLookupErr)
	}

	var liveUUID string
	liveLookupQuery := `
		SELECT uuid FROM api_publications
		WHERE organization_uuid = ? AND artifact_uuid = ? AND api_portal_uuid = ? AND is_draft = 0
	`
	liveLookupErr := tx.QueryRow(r.db.Rebind(liveLookupQuery), orgUUID, artifactUUID, apiPortalUUID).Scan(&liveUUID)

	var anchorUUID string
	switch {
	case liveLookupErr == nil:
		if err := r.mergeDraftIntoAnchor(tx, liveUUID, draftUUID, orgUUID, 0, "PUBLISHED", actor, now); err != nil {
			return nil, false, err
		}
		anchorUUID = liveUUID
		replaced = true
	case errors.Is(liveLookupErr, sql.ErrNoRows):
		promoteQuery := `
			UPDATE api_publications
			SET is_draft = 0, status = 'PUBLISHED', updated_by = ?, updated_at = ?
			WHERE uuid = ? AND organization_uuid = ?
		`
		if _, err := tx.Exec(r.db.Rebind(promoteQuery), actor, now, draftUUID, orgUUID); err != nil {
			return nil, false, fmt.Errorf("failed to promote publication draft: %w", err)
		}
		anchorUUID = draftUUID
	default:
		return nil, false, fmt.Errorf("failed to look up existing live publication: %w", liveLookupErr)
	}

	promotedQuery := `SELECT ` + publicationDetailColumns + `
		FROM api_publications WHERE uuid = ? AND organization_uuid = ?`
	row := tx.QueryRow(r.db.Rebind(promotedQuery), anchorUUID, orgUUID)
	promoted := &model.Publication{
		OrganizationUUID: orgUUID,
		ArtifactUUID:     artifactUUID,
		APIPortalUUID:    apiPortalUUID,
		IsDraft:          false,
	}
	if err := scanPublicationDetails(row, promoted); err != nil {
		return nil, false, fmt.Errorf("failed to read promoted publication: %w", err)
	}

	hasThumbnail, hasLandingPage, err := r.contentFlags(tx, promoted.UUID, orgUUID)
	if err != nil {
		return nil, false, err
	}
	promoted.HasThumbnail = hasThumbnail
	promoted.HasLandingPage = hasLandingPage

	if err := tx.Commit(); err != nil {
		return nil, false, fmt.Errorf("failed to commit publication promotion: %w", err)
	}
	return promoted, replaced, nil
}

// UnpublishPublication is PromoteDraftToPublication's mirror, called only
// after PortalPublisher.Unpublish has already succeeded. One transaction:
//   - No draft exists: the live row (the anchor) is demoted into the draft
//     in place — is_draft=1, status cleared to NULL (schema.sqlite.sql:
//     "status ... set only when is_draft = 0"), same row, same uuid, no
//     content copy.
//   - A draft already exists as its own row: that draft is merged into the
//     anchor (mergeDraftIntoAnchor) instead of deleting the anchor and
//     leaving the draft's own row as the survivor — the anchor's uuid
//     survives the unpublish too, inverted-but-symmetric with Publish's
//     republish branch. The draft's content replaces the live content.
//
// Either way a draft survives the operation. Returns found=false if no live
// row exists to unpublish — the same defensive re-check
// PromoteDraftToPublication does for its own precondition, since the caller
// (PublicationService.Unpublish) already checked this before calling the
// portal.
func (r *PublicationRepo) UnpublishPublication(artifactUUID, apiPortalUUID, orgUUID, actor string) (found bool, err error) {
	now := time.Now().UTC()

	tx, err := r.db.Begin()
	if err != nil {
		return false, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var liveUUID string
	liveLookupQuery := `
		SELECT uuid FROM api_publications
		WHERE organization_uuid = ? AND artifact_uuid = ? AND api_portal_uuid = ? AND is_draft = 0
	`
	lookupErr := tx.QueryRow(r.db.Rebind(liveLookupQuery), orgUUID, artifactUUID, apiPortalUUID).Scan(&liveUUID)
	if errors.Is(lookupErr, sql.ErrNoRows) {
		return false, nil
	}
	if lookupErr != nil {
		return false, fmt.Errorf("failed to look up live publication to unpublish: %w", lookupErr)
	}

	var draftUUID string
	draftLookupQuery := `
		SELECT uuid FROM api_publications
		WHERE organization_uuid = ? AND artifact_uuid = ? AND api_portal_uuid = ? AND is_draft = 1
	`
	draftErr := tx.QueryRow(r.db.Rebind(draftLookupQuery), orgUUID, artifactUUID, apiPortalUUID).Scan(&draftUUID)

	switch {
	case draftErr == nil:
		if err := r.mergeDraftIntoAnchor(tx, liveUUID, draftUUID, orgUUID, 1, nil, actor, now); err != nil {
			return false, err
		}
	case errors.Is(draftErr, sql.ErrNoRows):
		demoteQuery := `
			UPDATE api_publications
			SET is_draft = 1, status = NULL, updated_by = ?, updated_at = ?
			WHERE uuid = ? AND organization_uuid = ?
		`
		if _, err := tx.Exec(r.db.Rebind(demoteQuery), actor, now, liveUUID, orgUUID); err != nil {
			return false, fmt.Errorf("failed to demote publication to draft: %w", err)
		}
	default:
		return false, fmt.Errorf("failed to look up publication draft: %w", draftErr)
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("failed to commit unpublish: %w", err)
	}
	return true, nil
}

// DeprecatePublication sets the live row's status to DEPRECATED if it is currently
// PUBLISHED, changing only status and the audit columns. found is false when no row
// matched.
func (r *PublicationRepo) DeprecatePublication(artifactUUID, apiPortalUUID, orgUUID, actor string) (found bool, err error) {
	query := `
		UPDATE api_publications
		SET status = ?, updated_by = ?, updated_at = ?
		WHERE organization_uuid = ? AND artifact_uuid = ? AND api_portal_uuid = ? AND is_draft = 0 AND status = ?
	`
	result, err := r.db.Exec(r.db.Rebind(query), model.PublicationStatusDeprecated, actor, time.Now().UTC(),
		orgUUID, artifactUUID, apiPortalUUID, model.PublicationStatusPublished)
	if err != nil {
		return false, fmt.Errorf("failed to deprecate publication: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("failed to read deprecate result: %w", err)
	}
	return rows > 0, nil
}

// GetContent returns one content row (definition/landing page/thumbnail) for
// a publication row, or nil if none is stored.
func (r *PublicationRepo) GetContent(publicationUUID string, contentType model.PublicationContentType, orgUUID string) (*model.PublicationContent, error) {
	query := `
		SELECT uuid, organization_uuid, publication_uuid, type, file_name, content_type, content,
			data_version, created_by, created_at, updated_by, updated_at
		FROM api_publication_contents
		WHERE publication_uuid = ? AND type = ? AND organization_uuid = ?
	`
	c := &model.PublicationContent{}
	var fileName, ct, createdBy, updatedBy sql.NullString
	err := r.db.QueryRow(r.db.Rebind(query), publicationUUID, string(contentType), orgUUID).Scan(
		&c.UUID, &c.OrganizationUUID, &c.PublicationUUID, &c.Type, &fileName, &ct, &c.Content,
		&c.DataVersion, &createdBy, &c.CreatedAt, &updatedBy, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get publication content: %w", err)
	}
	c.FileName = fileName.String
	c.ContentType = ct.String
	c.CreatedBy = createdBy.String
	c.UpdatedBy = updatedBy.String
	return c, nil
}

// SaveContent replaces the named content row for content.PublicationUUID
// (creating it on first save) and bumps the parent api_publications row's
// updated_at/updated_by in the same transaction — one timestamp covers all
// four pieces (details, definition, landing page, thumbnail).
func (r *PublicationRepo) SaveContent(content *model.PublicationContent, actor string) error {
	now := time.Now().UTC()

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var existingUUID string
	lookupQuery := `
		SELECT uuid FROM api_publication_contents WHERE organization_uuid = ? AND publication_uuid = ? AND type = ?
	`
	lookupErr := tx.QueryRow(r.db.Rebind(lookupQuery), content.OrganizationUUID, content.PublicationUUID, string(content.Type)).Scan(&existingUUID)

	switch {
	case errors.Is(lookupErr, sql.ErrNoRows):
		content.UUID = uuid.New().String()
		content.DataVersion = "1.0"
		content.CreatedBy = actor
		content.CreatedAt = now
		content.UpdatedBy = actor
		content.UpdatedAt = now
		insertQuery := `
			INSERT INTO api_publication_contents (
				uuid, organization_uuid, publication_uuid, type, file_name, content_type, content,
				data_version, created_by, created_at, updated_by, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`
		_, err = tx.Exec(r.db.Rebind(insertQuery),
			content.UUID, content.OrganizationUUID, content.PublicationUUID, string(content.Type),
			nullIfEmpty(content.FileName), nullIfEmpty(content.ContentType), content.Content,
			content.DataVersion, content.CreatedBy, content.CreatedAt, content.UpdatedBy, content.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to insert publication content: %w", err)
		}
	case lookupErr != nil:
		return fmt.Errorf("failed to look up existing publication content: %w", lookupErr)
	default:
		content.UUID = existingUUID
		content.UpdatedBy = actor
		content.UpdatedAt = now
		updateQuery := `
			UPDATE api_publication_contents
			SET file_name = ?, content_type = ?, content = ?, updated_by = ?, updated_at = ?
			WHERE uuid = ? AND organization_uuid = ?
		`
		_, err = tx.Exec(r.db.Rebind(updateQuery),
			nullIfEmpty(content.FileName), nullIfEmpty(content.ContentType), content.Content,
			content.UpdatedBy, content.UpdatedAt, content.UUID, content.OrganizationUUID,
		)
		if err != nil {
			return fmt.Errorf("failed to update publication content: %w", err)
		}
	}

	touchPublicationQuery := `
		UPDATE api_publications SET updated_by = ?, updated_at = ? WHERE uuid = ? AND organization_uuid = ?
	`
	if _, err := tx.Exec(r.db.Rebind(touchPublicationQuery), actor, now, content.PublicationUUID, content.OrganizationUUID); err != nil {
		return fmt.Errorf("failed to bump publication updated_at: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit publication content save: %w", err)
	}
	return nil
}
