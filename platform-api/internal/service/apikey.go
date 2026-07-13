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
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

const (
	apiKeyNameMinLength     = 3
	apiKeyNameMaxLength     = 63
	hashingAlgorithmSHA256  = "sha256"
	defaultHashingAlgorithm = hashingAlgorithmSHA256
)

var (
	// invalidAPIKeyNameCharsRegex removes any character that is not lowercase alphanumeric or hyphen
	invalidAPIKeyNameCharsRegex = regexp.MustCompile(`[^a-z0-9\-]`)
	// consecutiveHyphensRegex collapses runs of hyphens into a single hyphen
	consecutiveHyphensRegex = regexp.MustCompile(`-+`)
)

// APIKeyService handles API key management operations for external API key injection
type APIKeyService struct {
	apiRepo              repository.APIRepository
	artifactRepo         repository.ArtifactRepository
	apiKeyRepo           repository.APIKeyRepository
	gatewayEventsService *GatewayEventsService
	auditRepo            repository.AuditRepository
	hashingAlgorithms    []string
	slogger              *slog.Logger
}

// NewAPIKeyService creates a new API key service instance.
// hashingAlgorithms specifies the algorithms used to hash API keys before storage and broadcast.
// If empty, defaults to [sha256].
func NewAPIKeyService(apiRepo repository.APIRepository, artifactRepo repository.ArtifactRepository, apiKeyRepo repository.APIKeyRepository, gatewayEventsService *GatewayEventsService, auditRepo repository.AuditRepository, hashingAlgorithms []string, slogger *slog.Logger) *APIKeyService {
	if len(hashingAlgorithms) == 0 {
		hashingAlgorithms = []string{defaultHashingAlgorithm}
	}
	return &APIKeyService{
		apiRepo:              apiRepo,
		artifactRepo:         artifactRepo,
		apiKeyRepo:           apiKeyRepo,
		gatewayEventsService: gatewayEventsService,
		auditRepo:            auditRepo,
		hashingAlgorithms:    hashingAlgorithms,
		slogger:              slogger,
	}
}

// hashAPIKey hashes a plain API key using the given algorithm.
// Currently only "sha256" is supported. Returns a hex-encoded hash string.
func hashAPIKey(plainAPIKey, algorithm string) (string, error) {
	trimmed := strings.TrimSpace(plainAPIKey)
	if trimmed == "" {
		return "", fmt.Errorf("API key cannot be empty")
	}
	switch algorithm {
	case hashingAlgorithmSHA256:
		h := sha256.New()
		h.Write([]byte(trimmed))
		return hex.EncodeToString(h.Sum(nil)), nil
	default:
		return "", fmt.Errorf("unsupported hashing algorithm: %s", algorithm)
	}
}

// buildAPIKeyHashesJSON hashes the plain API key with each of the given algorithms and
// returns a JSON object containing all results.
// Format: {"sha256": "<hash>", "sha512": "<hash>", ...}
func buildAPIKeyHashesJSON(plainAPIKey string, algorithms []string) (string, error) {
	pairs := make([]string, 0, len(algorithms))
	for _, algorithm := range algorithms {
		hash, err := hashAPIKey(plainAPIKey, algorithm)
		if err != nil {
			return "", fmt.Errorf("hashing with %s failed: %w", algorithm, err)
		}
		pairs = append(pairs, fmt.Sprintf(`"%s": "%s"`, algorithm, hash))
	}
	return "{" + strings.Join(pairs, ", ") + "}", nil
}

// resolveExpiresAt returns the absolute expiration time to persist.
// If expiresAt is set it takes precedence; otherwise expiresIn is converted to an
// absolute timestamp using the same unit mapping as the gateway controller
// (months approximated as 30 days). Returns an error if the resulting time is in
// the past or the unit is unrecognised.
func resolveExpiresAt(expiresAt *time.Time, expiresIn *api.ExpirationDuration) (*time.Time, error) {
	if expiresAt != nil {
		return expiresAt, nil
	}
	if expiresIn == nil {
		return nil, nil
	}

	now := time.Now()
	d := time.Duration(expiresIn.Duration)
	switch expiresIn.Unit {
	case api.Seconds:
		d *= time.Second
	case api.Minutes:
		d *= time.Minute
	case api.Hours:
		d *= time.Hour
	case api.Days:
		d *= 24 * time.Hour
	case api.Weeks:
		d *= 7 * 24 * time.Hour
	case api.Months:
		d *= 30 * 24 * time.Hour // approximate month as 30 days
	default:
		return nil, fmt.Errorf("unsupported expiration unit: %s", expiresIn.Unit)
	}

	expiry := now.Add(d)
	if expiry.Before(now) {
		return nil, fmt.Errorf("API key expiration time must be in the future")
	}
	return &expiry, nil
}

// maskAPIKey returns an 8-character masked representation of the API key:
// "***" + last 5 characters. If the key is 5 characters or shorter, returns "********".
func maskAPIKey(apiKey string) string {
	if len(apiKey) <= 5 {
		return "********"
	}
	return "***" + apiKey[len(apiKey)-5:]
}

// filterGatewaysByAllowedTargets returns only the gateways whose Name appears in the
// comma-separated allowedTargets string. When allowedTargets is empty or equals
// constants.APIKeyAllowedTargetsAll ("ALL") every gateway is returned unchanged.
func filterGatewaysByAllowedTargets(gateways []*model.Gateway, allowedTargets string) []*model.Gateway {
	if allowedTargets == "" || allowedTargets == constants.APIKeyAllowedTargetsAll {
		return gateways
	}
	filtered := make([]*model.Gateway, 0, len(gateways))
	for _, gw := range gateways {
		if gatewayAllowedByTargets(gw.Name, allowedTargets) {
			filtered = append(filtered, gw)
		}
	}
	return filtered
}

// filterAPIGatewaysByAllowedTargets is filterGatewaysByAllowedTargets for the
// association-scoped []*model.APIGatewayWithDetails shape returned by
// GetAPIGatewaysWithDetails. Matching is by gateway Name, identical to the base helper.
func filterAPIGatewaysByAllowedTargets(gateways []*model.APIGatewayWithDetails, allowedTargets string) []*model.APIGatewayWithDetails {
	if allowedTargets == "" || allowedTargets == constants.APIKeyAllowedTargetsAll {
		return gateways
	}
	filtered := make([]*model.APIGatewayWithDetails, 0, len(gateways))
	for _, gw := range gateways {
		if gatewayAllowedByTargets(gw.Name, allowedTargets) {
			filtered = append(filtered, gw)
		}
	}
	return filtered
}

// gatewayAllowedByTargets reports whether a gateway (identified by name) is permitted by a
// key's allowedTargets. An empty value or constants.APIKeyAllowedTargetsAll ("ALL") permits
// every gateway; otherwise the gateway name must appear in the comma-separated list. This is
// the single source of truth shared by the filter helpers and the deploy-time backfill.
func gatewayAllowedByTargets(gatewayName, allowedTargets string) bool {
	if allowedTargets == "" || allowedTargets == constants.APIKeyAllowedTargetsAll {
		return true
	}
	for _, name := range strings.Split(allowedTargets, ",") {
		if strings.TrimSpace(name) == gatewayName {
			return true
		}
	}
	return false
}

// randomHexString generates a random lowercase hex string of the requested length.
func randomHexString(n int) (string, error) {
	bytes := make([]byte, (n+1)/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return hex.EncodeToString(bytes)[:n], nil
}

// generateAPIKeyName derives a URL-safe, slug-style name from a display name using the
// same algorithm as the gateway controller:
//   - Lowercase
//   - Spaces and underscores → hyphens
//   - Remove all non-[a-z0-9-] characters
//   - Collapse consecutive hyphens
//   - Trim leading/trailing hyphens
//   - Enforce length [3, 63]; pad with random hex if too short
func generateAPIKeyName(displayName string) (string, error) {
	name := strings.ToLower(strings.TrimSpace(displayName))
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")
	name = invalidAPIKeyNameCharsRegex.ReplaceAllString(name, "")
	name = consecutiveHyphensRegex.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")

	if len(name) > apiKeyNameMaxLength {
		name = strings.TrimRight(name[:apiKeyNameMaxLength], "-")
	}
	if len(name) < apiKeyNameMinLength {
		padding, err := randomHexString(apiKeyNameMinLength - len(name))
		if err != nil {
			return "", err
		}
		if name == "" {
			name = padding
		} else {
			name = name + "-" + padding
		}
		if len(name) > apiKeyNameMaxLength {
			name = strings.TrimRight(name[:apiKeyNameMaxLength], "-")
		}
	}
	return name, nil
}

// resolveUniqueKeyName uses the caller-supplied name if present, otherwise derives one
// from the display name (or the API handle as a fallback) using the same slug algorithm
// as the gateway controller. Either way, it retries with a short random suffix on collision.
func (s *APIKeyService) resolveUniqueKeyName(artifactUUID string, req *api.CreateAPIKeyRequest, apiHandle string) (string, error) {
	var baseName string
	if req.Id != nil && strings.TrimSpace(*req.Id) != "" {
		baseName = strings.TrimSpace(*req.Id)
	} else {
		// Determine display name to slug from
		var displayName string
		if strings.TrimSpace(req.DisplayName) != "" {
			displayName = strings.TrimSpace(req.DisplayName)
		} else {
			// Auto-generate: "<api-handle>-key-<short-id>"
			shortID, err := utils.GenerateUUID()
			if err != nil {
				return "", fmt.Errorf("failed to generate short ID: %w", err)
			}
			displayName = fmt.Sprintf("%s-key-%s", apiHandle, shortID[:8])
		}

		var err error
		baseName, err = generateAPIKeyName(displayName)
		if err != nil {
			return "", fmt.Errorf("failed to generate API key name: %w", err)
		}
	}

	// Check for collision and retry with a short suffix (up to 5 attempts)
	const maxRetries = 5
	name := baseName
	for i := 0; i < maxRetries; i++ {
		existing, err := s.apiKeyRepo.GetByArtifactAndName(artifactUUID, name)
		if err != nil {
			return "", fmt.Errorf("failed to check API key name existence: %w", err)
		}
		if existing == nil {
			return name, nil
		}
		suffix, err := randomHexString(4)
		if err != nil {
			return "", err
		}
		if len(baseName)+1+len(suffix) > apiKeyNameMaxLength {
			name = strings.TrimRight(baseName[:apiKeyNameMaxLength-1-len(suffix)], "-") + "-" + suffix
		} else {
			name = baseName + "-" + suffix
		}
	}
	return "", fmt.Errorf("failed to generate a unique API key name after %d retries", maxRetries)
}

// APIKeyCreatedEventFromModel builds an apikey.created broadcast payload from a
// persisted API key record. It sends the hash JSON and masked key, never the plain
// key. Used both at key-creation time and to (re)broadcast pre-existing keys to a
// gateway the API is newly associated with at deploy time.
func APIKeyCreatedEventFromModel(k *model.APIKey) *model.APIKeyCreatedEvent {
	event := &model.APIKeyCreatedEvent{
		UUID:         k.UUID,
		ApiId:        k.ArtifactUUID,
		Name:         k.Name,
		ApiKeyHashes: k.APIKeyHashes,
		MaskedApiKey: k.MaskedAPIKey,
		Issuer:       k.Issuer,
		CreatedAt:    k.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    k.UpdatedAt.Format(time.RFC3339),
	}
	if k.ExpiresAt != nil {
		expiresAtStr := k.ExpiresAt.Format(time.RFC3339)
		event.ExpiresAt = &expiresAtStr
	}
	return event
}

// BackfillAPIKeysToGateway (re)broadcasts an artifact's existing active API keys to a
// gateway it has just been deployed/associated to. Keys are broadcast to their associated
// gateways only once, at creation time (CreateAPIKey and the per-kind key services), so a
// key created before this association would otherwise reach the gateway only after the
// gateway-controller's next reconnect bulk sync — leaving a window where the artifact is
// live but the key is rejected. Pushing existing keys here closes that window.
//
// It is artifact-kind-agnostic: ListByArtifact and the apikey.created event/controller
// handler are the same for REST APIs, LLM providers/proxies, MCP proxies, and WebSub/
// WebBroker APIs, so every deploy path can share this one helper.
//
// Best-effort: the controller upserts apikey.created idempotently, so re-sending a key the
// gateway already holds is harmless, and any failure is logged without blocking the
// deployment (the reconnect bulk sync remains the safety net).
func BackfillAPIKeysToGateway(apiKeyRepo repository.APIKeyRepository, gatewayRepo repository.GatewayRepository, events *GatewayEventsService, slogger *slog.Logger, artifactUUID, gatewayID, actor string) {
	if events == nil || apiKeyRepo == nil {
		return
	}

	keys, err := apiKeyRepo.ListByArtifact(artifactUUID)
	if err != nil {
		if slogger != nil {
			slogger.Warn("Failed to load API keys for deploy-time backfill",
				"artifactId", artifactUUID, "gatewayId", gatewayID, "error", err)
		}
		return
	}

	// Resolve the target gateway's name once so we can honor each key's AllowedTargets,
	// which is matched by gateway name — consistent with the create/revoke broadcast paths
	// (filterGatewaysByAllowedTargets). If the name can't be resolved, keys restricted to
	// specific targets are conservatively skipped (only "ALL"/unrestricted keys go through).
	gatewayName := ""
	if gatewayRepo != nil {
		if gw, gwErr := gatewayRepo.GetByUUID(gatewayID); gwErr != nil {
			if slogger != nil {
				slogger.Warn("Failed to resolve gateway for API key backfill target filtering",
					"gatewayId", gatewayID, "error", gwErr)
			}
		} else if gw != nil {
			gatewayName = gw.Name
		}
	}

	now := time.Now()
	backfilled := 0
	for _, k := range keys {
		if k == nil || k.Status != constants.APIKeyStatusActive {
			continue
		}
		// Skip expired keys — never push a dead key to a gateway.
		if k.ExpiresAt != nil && !k.ExpiresAt.After(now) {
			continue
		}
		// Honor per-key AllowedTargets — skip keys not permitted for this gateway.
		if !gatewayAllowedByTargets(gatewayName, k.AllowedTargets) {
			continue
		}

		event := APIKeyCreatedEventFromModel(k)
		if err := events.BroadcastAPIKeyCreatedEvent(gatewayID, actor, event); err != nil {
			if slogger != nil {
				slogger.Warn("Failed to backfill API key to gateway",
					"artifactId", artifactUUID, "gatewayId", gatewayID, "keyName", k.Name, "error", err)
			}
			continue
		}
		backfilled++
	}

	if backfilled > 0 && slogger != nil {
		slogger.Info("Backfilled existing API keys to gateway at deploy time",
			"artifactId", artifactUUID, "gatewayId", gatewayID, "count", backfilled)
	}
}

// CreateAPIKey hashes an external API key and broadcasts it to gateways where the API is deployed.
// This method is used when external platforms inject API keys to hybrid gateways.
func (s *APIKeyService) CreateAPIKey(ctx context.Context, apiHandle, kind, orgId, userId string, req *api.CreateAPIKeyRequest) error {
	// Resolve API handle to UUID within the artifact table backing kind, so a handle shared across
	// kinds resolves to exactly one artifact.
	apiMetadata, err := s.artifactRepo.GetAPIMetadataByHandleAndKind(apiHandle, kind, orgId)
	if err != nil {
		s.slogger.Error("Failed to get API metadata for API key creation", "apiHandle", apiHandle, "kind", kind, "error", err)
		return fmt.Errorf("failed to get API by handle: %w", err)
	}
	if apiMetadata == nil {
		s.slogger.Warn("API not found by handle", "apiHandle", apiHandle, "orgId", orgId)
		return apperror.ArtifactNotFound.New()
	}
	apiId := apiMetadata.ID

	// Get all deployments for this API to find target gateways.
	// An empty list is valid: the key is still persisted centrally and any gateway
	// associated later picks it up via deployment-time sync.
	gateways, err := s.apiRepo.GetAPIGatewaysWithDetails(apiId, orgId)
	if err != nil {
		return fmt.Errorf("failed to get API deployments for API handle: %s: %w", apiHandle, err)
	}

	// Resolve key name (required for DB uniqueness; derive from request or generate)
	keyName, err := s.resolveUniqueKeyName(apiId, req, apiHandle)
	if err != nil {
		s.slogger.Error("Failed to resolve API key name", "apiHandle", apiHandle, "error", err)
		return fmt.Errorf("failed to resolve API key name: %w", err)
	}

	// Hash the API key with all configured algorithms before storage and broadcast
	apiKeyHashesJSON, err := buildAPIKeyHashesJSON(req.ApiKey, s.hashingAlgorithms)
	if err != nil {
		s.slogger.Error("Failed to hash API key", "apiHandle", apiHandle, "keyName", keyName, "error", err)
		return fmt.Errorf("failed to hash API key: %w", err)
	}

	// Persist the API key to the database before broadcasting
	maskedAPIKey := maskAPIKey(req.ApiKey)
	expiresAt, err := resolveExpiresAt(req.ExpiresAt, req.ExpiresIn)
	if err != nil {
		s.slogger.Error("Invalid expiration for API key creation", "apiHandle", apiHandle, "keyName", keyName, "error", err)
		return fmt.Errorf("invalid expiration: %w", err)
	}
	apiKeyUUID, err := utils.GenerateUUID()
	if err != nil {
		s.slogger.Error("Failed to generate UUID for API key", "apiHandle", apiHandle, "keyName", keyName, "error", err)
		return fmt.Errorf("failed to generate API key UUID: %w", err)
	}

	// Apply defaults for issuer and allowedTargets
	var issuer *string
	if req.Issuer != nil && strings.TrimSpace(*req.Issuer) != "" {
		v := strings.TrimSpace(*req.Issuer)
		issuer = &v
	}
	allowedTargets := constants.APIKeyAllowedTargetsAll

	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		displayName = keyName
	}

	dbKey := &model.APIKey{
		UUID:           apiKeyUUID,
		ArtifactUUID:   apiId,
		Name:           keyName,
		DisplayName:    displayName,
		MaskedAPIKey:   maskedAPIKey,
		APIKeyHashes:   apiKeyHashesJSON,
		Status:         constants.APIKeyStatusActive,
		CreatedBy:      userId,
		ExpiresAt:      expiresAt,
		Issuer:         issuer,
		AllowedTargets: allowedTargets,
	}
	if err := s.apiKeyRepo.Create(dbKey); err != nil {
		s.slogger.Error("Failed to persist API key to database", "apiHandle", apiHandle, "keyName", keyName, "error", err)
		return fmt.Errorf("failed to persist API key: %w", err)
	}
	_ = s.auditRepo.Record("CREATE", apiKeyUUID, "api_key", orgId, userId)

	// Build the API key created event — send the hash JSON and masked key, not the plain key
	event := APIKeyCreatedEventFromModel(dbKey)
	event.ExternalRefId = req.ExternalRefId

	successCount := 0
	failureCount := 0
	var lastError error
	for _, gateway := range gateways {
		s.slogger.Info("Broadcasting API key created event", "apiHandle", apiHandle, "gatewayId", gateway.ID, "keyName", keyName)
		err := s.gatewayEventsService.BroadcastAPIKeyCreatedEvent(gateway.ID, userId, event)
		if err != nil {
			failureCount++
			lastError = err
			s.slogger.Error("Failed to broadcast API key created event", "apiHandle", apiHandle, "gatewayId", gateway.ID, "keyName", keyName, "error", err)
		} else {
			successCount++
			s.slogger.Info("Successfully broadcast API key created event", "apiHandle", apiHandle, "gatewayId", gateway.ID, "keyName", keyName)
		}
	}
	s.slogger.Info("API key creation broadcast summary", "total", len(gateways), "success", successCount, "failed", failureCount)
	if successCount == 0 {
		s.slogger.Error("API key created event was not broadcast to any gateway", "apiHandle", apiHandle, "keyName", keyName, "lastError", lastError)
	} else if failureCount > 0 {
		s.slogger.Warn("Failed to broadcast API key created event to some gateways", "apiHandle", apiHandle, "keyName", keyName, "failed", failureCount, "lastError", lastError)
	}

	return nil
}

// UpdateAPIKey updates/regenerates an API key and broadcasts it to all gateways where the API is deployed.
// This method is used when external platforms rotates/regenerates API keys on hybrid gateways.
func (s *APIKeyService) UpdateAPIKey(ctx context.Context, apiHandle, kind, orgId, keyName, userId string, req *api.UpdateAPIKeyRequest) error {
	// Resolve API handle to UUID within the artifact table backing kind, so a handle shared across
	// kinds resolves to exactly one artifact.
	apiMetadata, err := s.artifactRepo.GetAPIMetadataByHandleAndKind(apiHandle, kind, orgId)
	if err != nil {
		s.slogger.Error("Failed to get API metadata for API key update", "apiHandle", apiHandle, "kind", kind, "error", err)
		return fmt.Errorf("failed to get API by handle: %w", err)
	}
	if apiMetadata == nil {
		s.slogger.Warn("API not found by handle for API key update", "apiHandle", apiHandle)
		return apperror.ArtifactNotFound.New()
	}
	apiId := apiMetadata.ID

	// Get all deployments for this API to find target gateways
	gateways, err := s.apiRepo.GetAPIGatewaysWithDetails(apiId, orgId)
	if err != nil {
		s.slogger.Error("Failed to get deployments for API key update", "apiHandle", apiHandle, "error", err)
		return fmt.Errorf("failed to get API deployments: %w", err)
	}
	if len(gateways) == 0 {
		s.slogger.Warn("No gateway deployments found for API", "apiHandle", apiHandle)
		return apperror.GatewayConnectionUnavailable.New()
	}

	// Hash the API key with all configured algorithms before storage and broadcast
	apiKeyHashesJSON, err := buildAPIKeyHashesJSON(req.ApiKey, s.hashingAlgorithms)
	if err != nil {
		s.slogger.Error("Failed to hash API key", "apiHandle", apiHandle, "keyName", keyName, "error", err)
		return fmt.Errorf("failed to hash API key: %w", err)
	}

	// Persist the updated API key to the database before broadcasting
	maskedAPIKey := maskAPIKey(req.ApiKey)
	expiresAt, err := resolveExpiresAt(req.ExpiresAt, req.ExpiresIn)
	if err != nil {
		s.slogger.Error("Invalid expiration for API key update", "apiHandle", apiHandle, "keyName", keyName, "error", err)
		return fmt.Errorf("invalid expiration: %w", err)
	}
	// Fetch UUID before update for consistent audit record (CREATE uses UUID, not name)
	existingKey, existingKeyErr := s.apiKeyRepo.GetByArtifactAndName(apiId, keyName)

	dbKey := &model.APIKey{
		ArtifactUUID: apiId,
		Name:         keyName,
		MaskedAPIKey: maskedAPIKey,
		APIKeyHashes: apiKeyHashesJSON,
		Status:       constants.APIKeyStatusActive,
		ExpiresAt:    expiresAt,
		Issuer:       req.Issuer,
	}
	if err := s.apiKeyRepo.Update(dbKey); err != nil {
		s.slogger.Error("Failed to update API key in database", "apiHandle", apiHandle, "keyName", keyName, "error", err)
		return fmt.Errorf("failed to update API key in database: %w", err)
	}
	if s.auditRepo != nil {
		auditUUID := keyName
		if existingKeyErr == nil && existingKey != nil {
			auditUUID = existingKey.UUID
		}
		_ = s.auditRepo.Record("UPDATE", auditUUID, "api_key", orgId, userId)
	}

	// Build the API key updated event — send the hash JSON and masked key, not the plain key
	event := &model.APIKeyUpdatedEvent{
		ApiId:        apiId,
		KeyName:      keyName,
		ApiKeyHashes: apiKeyHashesJSON,
		MaskedApiKey: maskedAPIKey,
		Issuer:       req.Issuer,
		UpdatedAt:    dbKey.UpdatedAt.Format(time.RFC3339),
	}
	if req.ExternalRefId != nil {
		event.ExternalRefId = req.ExternalRefId
	}
	if expiresAt != nil {
		expiresAtStr := expiresAt.Format(time.RFC3339)
		event.ExpiresAt = &expiresAtStr
	}

	successCount := 0
	failureCount := 0
	var lastError error
	for _, gateway := range gateways {
		gatewayID := gateway.ID

		s.slogger.Info("Broadcasting API key updated event", "apiHandle", apiHandle, "gatewayId", gatewayID, "keyName", keyName)

		// Broadcast with retries
		err := s.gatewayEventsService.BroadcastAPIKeyUpdatedEvent(gatewayID, userId, event)
		if err != nil {
			failureCount++
			lastError = err
			s.slogger.Error("Failed to broadcast API key updated event", "apiHandle", apiHandle, "gatewayId", gatewayID, "keyName", keyName, "error", err)
		} else {
			successCount++
			s.slogger.Info("Successfully broadcast API key updated event", "apiHandle", apiHandle, "gatewayId", gatewayID, "keyName", keyName)
		}
	}

	// Log summary
	s.slogger.Info("API key update broadcast summary", "apiHandle", apiHandle, "keyName", keyName, "total", len(gateways), "success", successCount, "failed", failureCount)

	if successCount == 0 {
		s.slogger.Error("Failed to deliver API key update to any gateway", "apiHandle", apiHandle, "keyName", keyName, "lastError", lastError)
	}

	return nil
}

// RevokeAPIKey broadcasts API key revocation to all gateways where the API is deployed
func (s *APIKeyService) RevokeAPIKey(ctx context.Context, apiHandle, kind, orgId, keyName, userId string) error {
	// Resolve API handle to UUID within the artifact table backing kind, so a handle shared across
	// kinds resolves to exactly one artifact.
	apiMetadata, err := s.artifactRepo.GetAPIMetadataByHandleAndKind(apiHandle, kind, orgId)
	if err != nil {
		s.slogger.Error("Failed to get API metadata for API key revocation", "apiHandle", apiHandle, "kind", kind, "error", err)
		return fmt.Errorf("failed to get API by handle: %w", err)
	}
	if apiMetadata == nil {
		s.slogger.Warn("API not found by handle for API key revocation", "apiHandle", apiHandle)
		return apperror.ArtifactNotFound.New()
	}
	apiId := apiMetadata.ID

	// Get all deployments for this API to find target gateways
	gateways, err := s.apiRepo.GetAPIGatewaysWithDetails(apiId, orgId)
	if err != nil {
		return fmt.Errorf("failed to get API deployments: %w", err)
	}
	if len(gateways) == 0 {
		return apperror.GatewayConnectionUnavailable.New()
	}

	// Fetch UUID before revoke for consistent audit record (CREATE uses UUID, not name)
	revokeKey, revokeKeyErr := s.apiKeyRepo.GetByArtifactAndName(apiId, keyName)

	// Revoke the API key in the database before broadcasting
	if err := s.apiKeyRepo.Revoke(apiId, keyName); err != nil {
		s.slogger.Error("Failed to revoke API key in database", "apiHandle", apiHandle, "keyName", keyName, "error", err)
		return fmt.Errorf("failed to revoke API key in database: %w", err)
	}
	if s.auditRepo != nil {
		auditUUID := keyName
		if revokeKeyErr == nil && revokeKey != nil {
			auditUUID = revokeKey.UUID
		}
		_ = s.auditRepo.Record("REVOKE", auditUUID, "api_key", orgId, userId)
	}

	// Build the API key revoked event
	event := &model.APIKeyRevokedEvent{
		ApiId:   apiId,
		KeyName: keyName,
	}

	// Track delivery statistics
	successCount := 0
	failureCount := 0
	var lastError error
	for _, gateway := range gateways {
		gatewayID := gateway.ID

		s.slogger.Info("Broadcasting API key revoked event", "apiHandle", apiId, "gatewayId", gatewayID, "keyName", keyName)

		// Broadcast with retries
		err := s.gatewayEventsService.BroadcastAPIKeyRevokedEvent(gatewayID, userId, event)
		if err != nil {
			failureCount++
			lastError = err
			s.slogger.Error("Failed to broadcast API key revoked event", "apiHandle", apiId, "gatewayId", gatewayID, "keyName", keyName, "error", err)
		} else {
			successCount++
			s.slogger.Info("Successfully broadcast API key revoked event", "apiHandle", apiId, "gatewayId", gatewayID, "keyName", keyName)
		}
	}

	// Log summary
	s.slogger.Info("API key revocation broadcast summary", "apiHandle", apiId, "keyName", keyName, "total", len(gateways), "success", successCount, "failed", failureCount)

	if failureCount == len(gateways) {
		s.slogger.Error("Failed to deliver API key revocation to any gateway", "apiHandle", apiId, "keyName", keyName, "lastError", lastError)
	}
	if failureCount > 0 {
		s.slogger.Warn("Partial delivery of API key revocation", "apiHandle", apiId, "keyName", keyName, "failureCount", failureCount, "total", len(gateways))
	}

	return nil
}
