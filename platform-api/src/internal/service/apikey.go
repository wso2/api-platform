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

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"
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
	apiKeyRepo           repository.APIKeyRepository
	gatewayEventsService *GatewayEventsService
	hashingAlgorithms    []string
	slogger              *slog.Logger
}

// NewAPIKeyService creates a new API key service instance.
// hashingAlgorithms specifies the algorithms used to hash API keys before storage and broadcast.
// If empty, defaults to [sha256].
func NewAPIKeyService(apiRepo repository.APIRepository, apiKeyRepo repository.APIKeyRepository, gatewayEventsService *GatewayEventsService, hashingAlgorithms []string, slogger *slog.Logger) *APIKeyService {
	if len(hashingAlgorithms) == 0 {
		hashingAlgorithms = []string{defaultHashingAlgorithm}
	}
	return &APIKeyService{
		apiRepo:              apiRepo,
		apiKeyRepo:           apiKeyRepo,
		gatewayEventsService: gatewayEventsService,
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
	allowed := make(map[string]struct{})
	for _, name := range strings.Split(allowedTargets, ",") {
		allowed[strings.TrimSpace(name)] = struct{}{}
	}
	filtered := make([]*model.Gateway, 0, len(gateways))
	for _, gw := range gateways {
		if _, ok := allowed[gw.Name]; ok {
			filtered = append(filtered, gw)
		}
	}
	return filtered
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

// resolveUniqueKeyName returns the caller-supplied name if present, otherwise derives one
// from the display name (or the API handle as a fallback) using the same slug algorithm
// as the gateway controller. It retries with a short random suffix on collision.
func (s *APIKeyService) resolveUniqueKeyName(artifactUUID string, req *api.CreateAPIKeyRequest, apiHandle string) (string, error) {
	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		return strings.TrimSpace(*req.Name), nil
	}

	// Determine display name to slug from
	var displayName string
	if req.DisplayName != nil && strings.TrimSpace(*req.DisplayName) != "" {
		displayName = strings.TrimSpace(*req.DisplayName)
	} else {
		// Auto-generate: "<api-handle>-key-<short-id>"
		shortID, err := utils.GenerateUUID()
		if err != nil {
			return "", fmt.Errorf("failed to generate short ID: %w", err)
		}
		displayName = fmt.Sprintf("%s-key-%s", apiHandle, shortID[:8])
	}

	baseName, err := generateAPIKeyName(displayName)
	if err != nil {
		return "", fmt.Errorf("failed to generate API key name: %w", err)
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

// CreateAPIKey hashes an external API key and broadcasts it to gateways where the API is deployed.
// This method is used when external platforms inject API keys to hybrid gateways.
func (s *APIKeyService) CreateAPIKey(ctx context.Context, apiHandle, orgId, userId string, req *api.CreateAPIKeyRequest) error {
	// Resolve API handle to UUID
	apiMetadata, err := s.apiRepo.GetAPIMetadataByHandle(apiHandle, orgId)
	if err != nil {
		s.slogger.Error("Failed to get API metadata for API key creation", "apiHandle", apiHandle, "error", err)
		return fmt.Errorf("failed to get API by handle: %w", err)
	}
	if apiMetadata == nil {
		s.slogger.Warn("API not found by handle", "apiHandle", apiHandle, "orgId", orgId)
		return constants.ErrAPINotFound
	}
	apiId := apiMetadata.ID

	// Get all deployments for this API to find target gateways
	gateways, err := s.apiRepo.GetAPIGatewaysWithDetails(apiId, orgId)
	if err != nil {
		return fmt.Errorf("failed to get API deployments for API handle: %s: %w", apiHandle, err)
	}

	if len(gateways) == 0 {
		return constants.ErrGatewayUnavailable
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

	dbKey := &model.APIKey{
		UUID:           apiKeyUUID,
		ArtifactUUID:   apiId,
		Name:           keyName,
		MaskedAPIKey:   maskedAPIKey,
		APIKeyHashes:   apiKeyHashesJSON,
		Status:         "active",
		CreatedBy:      userId,
		ExpiresAt:      expiresAt,
		Issuer:         issuer,
		AllowedTargets: allowedTargets,
	}
	if err := s.apiKeyRepo.Create(dbKey); err != nil {
		s.slogger.Error("Failed to persist API key to database", "apiHandle", apiHandle, "keyName", keyName, "error", err)
		return fmt.Errorf("failed to persist API key: %w", err)
	}

	// Build the API key created event — send the hash JSON and masked key, not the plain key
	event := &model.APIKeyCreatedEvent{
		UUID:          apiKeyUUID,
		ApiId:         apiId,
		Name:          keyName,
		ApiKeyHashes:  apiKeyHashesJSON,
		MaskedApiKey:  maskedAPIKey,
		ExternalRefId: req.ExternalRefId,
		Issuer:        issuer,
		CreatedAt:     dbKey.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     dbKey.UpdatedAt.Format(time.RFC3339),
	}
	if expiresAt != nil {
		expiresAtStr := expiresAt.Format(time.RFC3339)
		event.ExpiresAt = &expiresAtStr
	}

	// Track delivery statistics
	successCount := 0
	failureCount := 0
	var lastError error

	// Broadcast event to all gateways where API is deployed
	for _, gateway := range gateways {
		gatewayID := gateway.ID

		s.slogger.Info("Broadcasting API key created event", "apiHandle", apiHandle, "gatewayId", gatewayID, "keyName", keyName)

		// Broadcast with retries
		err := s.gatewayEventsService.BroadcastAPIKeyCreatedEvent(gatewayID, userId, event)
		if err != nil {
			failureCount++
			lastError = err
			s.slogger.Error("Failed to broadcast API key created event", "apiHandle", apiHandle, "gatewayId", gatewayID, "keyName", keyName, "error", err)
		} else {
			successCount++
			s.slogger.Info("Successfully broadcast API key created event", "apiHandle", apiHandle, "gatewayId", gatewayID, "keyName", keyName)
		}
	}

	// Log summary — API key is persisted to DB regardless of broadcast outcome
	s.slogger.Info("API key creation broadcast summary", "apiHandle", apiHandle, "keyName", keyName, "total", len(gateways), "success", successCount, "failed", failureCount)
	if successCount == 0 {
		s.slogger.Error("API key created event was not broadcast to any gateway", "apiHandle", apiHandle, "keyName", keyName, "lastError", lastError)
	} else if failureCount > 0 {
		s.slogger.Warn("Failed to broadcast API key created event to some gateways", "apiHandle", apiHandle, "keyName", keyName, "failed", failureCount, "lastError", lastError)
	}

	return nil
}

// UpdateAPIKey updates/regenerates an API key and broadcasts it to all gateways where the API is deployed.
// This method is used when external platforms rotates/regenerates API keys on hybrid gateways.
func (s *APIKeyService) UpdateAPIKey(ctx context.Context, apiHandle, orgId, keyName, userId string, req *api.UpdateAPIKeyRequest) error {
	// Resolve API handle to UUID
	apiMetadata, err := s.apiRepo.GetAPIMetadataByHandle(apiHandle, orgId)
	if err != nil {
		s.slogger.Error("Failed to get API metadata for API key update", "apiHandle", apiHandle, "error", err)
		return fmt.Errorf("failed to get API by handle: %w", err)
	}
	if apiMetadata == nil {
		s.slogger.Warn("API not found by handle for API key update", "apiHandle", apiHandle)
		return constants.ErrAPINotFound
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
		return constants.ErrGatewayUnavailable
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
	dbKey := &model.APIKey{
		ArtifactUUID: apiId,
		Name:         keyName,
		MaskedAPIKey: maskedAPIKey,
		APIKeyHashes: apiKeyHashesJSON,
		Status:       "active",
		ExpiresAt:    expiresAt,
		Issuer:       req.Issuer,
	}
	if err := s.apiKeyRepo.Update(dbKey); err != nil {
		s.slogger.Error("Failed to update API key in database", "apiHandle", apiHandle, "keyName", keyName, "error", err)
		return fmt.Errorf("failed to update API key in database: %w", err)
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

	// Track delivery statistics
	successCount := 0
	failureCount := 0
	var lastError error

	// Broadcast event to all gateways where API is deployed
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
func (s *APIKeyService) RevokeAPIKey(ctx context.Context, apiHandle, orgId, keyName, userId string) error {
	// Resolve API handle to UUID
	apiMetadata, err := s.apiRepo.GetAPIMetadataByHandle(apiHandle, orgId)
	if err != nil {
		s.slogger.Error("Failed to get API metadata for API key revocation", "apiHandle", apiHandle, "error", err)
		return fmt.Errorf("failed to get API by handle: %w", err)
	}
	if apiMetadata == nil {
		s.slogger.Warn("API not found by handle for API key revocation", "apiHandle", apiHandle)
		return constants.ErrAPINotFound
	}
	apiId := apiMetadata.ID

	// Get all deployments for this API to find target gateways
	gateways, err := s.apiRepo.GetAPIGatewaysWithDetails(apiId, orgId)
	if err != nil {
		return fmt.Errorf("failed to get API deployments: %w", err)
	}

	if len(gateways) == 0 {
		return constants.ErrGatewayUnavailable
	}

	// Revoke the API key in the database before broadcasting
	if err := s.apiKeyRepo.Revoke(apiId, keyName); err != nil {
		s.slogger.Error("Failed to revoke API key in database", "apiHandle", apiHandle, "keyName", keyName, "error", err)
		return fmt.Errorf("failed to revoke API key in database: %w", err)
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

	// Broadcast event to all gateways where API is deployed
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
