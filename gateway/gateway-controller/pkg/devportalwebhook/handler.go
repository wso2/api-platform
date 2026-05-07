/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package devportalwebhook

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wso2/api-platform/common/apikey"
	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/apikeyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

const maxBodyBytes = 4 << 20 // 4 MB

// HandlerConfig bundles the dependencies for the webhook handler.
type HandlerConfig struct {
	Secret      []byte
	PrivateKey  *rsa.PrivateKey
	GatewayType string // "" = accept all gateway types
	Cache       *IdempotencyCache
	APIKeyMgr   *apikeyxds.APIKeyStateManager
	DB          storage.Storage
	EventHub    eventhub.EventHub
	GatewayID   string
	Logger      *slog.Logger
}

// Handler is the devportal webhook HTTP handler.
type Handler struct {
	secret      []byte
	privateKey  *rsa.PrivateKey
	gatewayType string
	cache       *IdempotencyCache
	apiKeyMgr   *apikeyxds.APIKeyStateManager
	db          storage.Storage
	eventHub    eventhub.EventHub
	gatewayID   string
	logger      *slog.Logger
}

// NewHandler constructs a Handler from HandlerConfig.
func NewHandler(cfg HandlerConfig) *Handler {
	return &Handler{
		secret:      cfg.Secret,
		privateKey:  cfg.PrivateKey,
		gatewayType: cfg.GatewayType,
		cache:       cfg.Cache,
		apiKeyMgr:   cfg.APIKeyMgr,
		db:          cfg.DB,
		eventHub:    cfg.EventHub,
		gatewayID:   cfg.GatewayID,
		logger:      cfg.Logger,
	}
}

// HandleWebhook is the Gin handler for POST /webhooks/devportal.
func (h *Handler) HandleWebhook(c *gin.Context) {
	log := middleware.GetLogger(c, h.logger)
	correlationID := middleware.GetCorrelationID(c)

	// 1. Read raw body (bounded to maxBodyBytes for safety).
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxBodyBytes))
	if err != nil {
		log.Error("Failed to read webhook body", slog.String("correlation_id", correlationID), slog.Any("error", err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	// 2. Verify HMAC-SHA256 signature before any processing.
	sigHeader := c.GetHeader("X-Devportal-Signature")
	if err := VerifySignature(h.secret, sigHeader, body, time.Now()); err != nil {
		log.Warn("Webhook signature verification failed",
			slog.String("correlation_id", correlationID),
			slog.Any("error", err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid signature"})
		return
	}

	// 3. Parse the event envelope.
	var env EventEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		log.Error("Failed to parse webhook body", slog.String("correlation_id", correlationID), slog.Any("error", err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "malformed JSON body"})
		return
	}

	log = log.With(
		slog.String("event_id", env.EventID),
		slog.String("event_type", env.EventType),
		slog.String("gateway_type", env.GatewayType),
	)

	// 4. Filter by gateway_type when a specific type is configured.
	if h.gatewayType != "" && env.GatewayType != h.gatewayType {
		log.Debug("Ignoring event for different gateway type",
			slog.String("expected", h.gatewayType),
			slog.String("got", env.GatewayType))
		c.JSON(http.StatusOK, gin.H{"status": "ignored"})
		return
	}

	// 5. Idempotency guard — deduplicate retries from the devportal.
	if h.cache.CheckAndSet(env.EventID) {
		log.Debug("Duplicate event ignored")
		c.JSON(http.StatusOK, gin.H{"status": "duplicate"})
		return
	}

	// 6. Dispatch by event type.
	var handlerErr error
	switch env.EventType {
	case "apikey.generated", "apikey.regenerated":
		handlerErr = h.handleAPIKeyGenerated(c.Request.Context(), &env, correlationID, log)
	case "apikey.revoked":
		handlerErr = h.handleAPIKeyRevoked(c.Request.Context(), &env, correlationID, log)
	case "subscription.created":
		handlerErr = h.handleSubscriptionCreated(c.Request.Context(), &env, correlationID, log)
	case "subscription.deleted":
		handlerErr = h.handleSubscriptionDeleted(c.Request.Context(), &env, correlationID, log)
	default:
		log.Debug("Unhandled devportal event type — ignoring")
		c.JSON(http.StatusOK, gin.H{"status": "unhandled"})
		return
	}

	// 7. Return 500 on handler errors so the devportal retries; 200 on success.
	if handlerErr != nil {
		log.Error("Failed to process devportal event",
			slog.String("correlation_id", correlationID),
			slog.Any("error", handlerErr))
		c.JSON(http.StatusInternalServerError, gin.H{"error": handlerErr.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// --- apikey.generated / apikey.regenerated ---

func (h *Handler) handleAPIKeyGenerated(ctx context.Context, env *EventEnvelope, correlationID string, log *slog.Logger) error {
	var data APIKeyEventData
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return fmt.Errorf("parsing apikey event data: %w", err)
	}

	// Decrypt the API key secret.
	plaintext, err := DecryptAPIKey(h.privateKey, &data.EncryptedKey)
	if err != nil {
		return fmt.Errorf("decrypting API key: %w", err)
	}

	// Hash using SHA-256 (matches hashAPIKeyWithSHA256 in APIKeyService).
	hashedKey := hashSHA256Hex(plaintext)

	// Mask for safe logging (show first 8 + "****" + last 4 chars).
	maskedKey := maskAPIKey(plaintext)

	// Resolve the gateway-internal API UUID.
	apiID, apiName, apiVersion, err := h.resolveAPI(ctx, data.API)
	if err != nil {
		return fmt.Errorf("resolving API for key %q: %w", data.Name, err)
	}

	action := "CREATE"
	if env.EventType == "apikey.regenerated" {
		action = "UPDATE"
	}

	now := time.Now()
	keyUUID := uuid.New().String()
	externalRefID := data.KeyID

	apiKeyRecord := &models.APIKey{
		UUID:          keyUUID,
		Name:          data.Name,
		APIKey:        hashedKey,
		MaskedAPIKey:  maskedKey,
		ArtifactUUID:  apiID,
		Status:        models.APIKeyStatusActive,
		Source:        "external",
		ExternalRefId: &externalRefID,
		ExpiresAt:     data.ExpiresAt,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// Persist to DB so the key survives restarts and other replicas can load it.
	if err := h.db.UpsertAPIKey(apiKeyRecord); err != nil {
		return fmt.Errorf("persisting API key to DB: %w", err)
	}

	// Update in-memory store and xDS policy engine snapshot on this replica.
	if err := h.apiKeyMgr.StoreAPIKey(apiID, apiName, apiVersion, apiKeyRecord, correlationID); err != nil {
		return fmt.Errorf("updating API key xDS state: %w", err)
	}

	// Publish to EventHub so other replicas pick up the change via EventListener.
	h.publishEvent(eventhub.EventTypeAPIKey, action, apikey.BuildAPIKeyEntityID(apiID, keyUUID))

	log.Info("Processed devportal apikey event",
		slog.String("event_type", env.EventType),
		slog.String("key_name", data.Name),
		slog.String("api_id", apiID),
		slog.String("masked_key", maskedKey))

	return nil
}

// --- apikey.revoked ---

func (h *Handler) handleAPIKeyRevoked(ctx context.Context, env *EventEnvelope, correlationID string, log *slog.Logger) error {
	var data APIKeyRevokedData
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return fmt.Errorf("parsing apikey.revoked data: %w", err)
	}

	apiID, apiName, apiVersion, err := h.resolveAPI(ctx, data.API)
	if err != nil {
		return fmt.Errorf("resolving API for key revocation %q: %w", data.Name, err)
	}

	// Update in-memory store and xDS policy engine snapshot on this replica.
	if err := h.apiKeyMgr.RevokeAPIKey(apiID, apiName, apiVersion, data.Name, correlationID); err != nil {
		return fmt.Errorf("revoking API key in xDS state: %w", err)
	}

	// Remove from DB (non-fatal if already gone).
	if err := h.db.RemoveAPIKeyAPIAndName(apiID, data.Name); err != nil && !storage.IsNotFoundError(err) {
		log.Warn("Failed to remove revoked API key from DB",
			slog.String("api_id", apiID),
			slog.String("key_name", data.Name),
			slog.Any("error", err))
	}

	// Publish to EventHub for multi-replica sync.
	// Use keyID as a stable entity ID suffix since we don't have the internal UUID here.
	h.publishEvent(eventhub.EventTypeAPIKey, "DELETE", apikey.BuildAPIKeyEntityID(apiID, data.KeyID))

	log.Info("Processed devportal apikey.revoked event",
		slog.String("key_name", data.Name),
		slog.String("api_id", apiID))

	return nil
}

// --- subscription.created ---

func (h *Handler) handleSubscriptionCreated(ctx context.Context, env *EventEnvelope, correlationID string, log *slog.Logger) error {
	var data SubscriptionCreatedData
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return fmt.Errorf("parsing subscription.created data: %w", err)
	}

	apiID, _, _, err := h.resolveAPI(ctx, data.API)
	if err != nil {
		return fmt.Errorf("resolving API for subscription %q: %w", data.Subscription.RefID, err)
	}

	// Resolve the subscription plan ID from plan name.
	planID, err := h.resolvePlanID(data.Subscription.PlanName)
	if err != nil {
		log.Warn("Failed to resolve subscription plan; proceeding without plan ID",
			slog.String("plan_name", data.Subscription.PlanName),
			slog.Any("error", err))
	}

	now := time.Now()
	sub := &models.Subscription{
		ID:                data.Subscription.RefID,
		APIID:             apiID,
		SubscriptionPlanID: planID,
		GatewayID:         h.gatewayID,
		Status:            models.SubscriptionStatusActive,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := h.db.SaveSubscription(sub); err != nil {
		if storage.IsConflictError(err) {
			// Idempotent upsert on devportal retry.
			sub.UpdatedAt = now
			if updateErr := h.db.UpdateSubscription(sub); updateErr != nil {
				return fmt.Errorf("updating existing subscription %q: %w", data.Subscription.RefID, updateErr)
			}
		} else {
			return fmt.Errorf("persisting subscription %q: %w", data.Subscription.RefID, err)
		}
	}

	// Publish to EventHub; EventListener calls UpdateSnapshot() on all replicas.
	h.publishEvent(eventhub.EventTypeSubscription, "CREATE", data.Subscription.RefID)

	log.Info("Processed devportal subscription.created event",
		slog.String("subscription_ref_id", data.Subscription.RefID),
		slog.String("api_id", apiID),
		slog.String("plan_name", data.Subscription.PlanName))

	return nil
}

// --- subscription.deleted ---

func (h *Handler) handleSubscriptionDeleted(ctx context.Context, env *EventEnvelope, correlationID string, log *slog.Logger) error {
	var data SubscriptionDeletedData
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return fmt.Errorf("parsing subscription.deleted data: %w", err)
	}

	if err := h.db.DeleteSubscription(data.Subscription.RefID, h.gatewayID); err != nil {
		if storage.IsNotFoundError(err) {
			// Idempotent: already deleted.
			log.Debug("Subscription already absent, treating delete as success",
				slog.String("subscription_ref_id", data.Subscription.RefID))
			return nil
		}
		return fmt.Errorf("deleting subscription %q: %w", data.Subscription.RefID, err)
	}

	// Publish to EventHub; EventListener calls UpdateSnapshot() on all replicas.
	h.publishEvent(eventhub.EventTypeSubscription, "DELETE", data.Subscription.RefID)

	log.Info("Processed devportal subscription.deleted event",
		slog.String("subscription_ref_id", data.Subscription.RefID))

	return nil
}

// --- helpers ---

// resolveAPI maps a devportal APIRef to the gateway-internal UUID + display name + version.
// Lookup order: CPArtifactID (ref_id) → name+version scan over all configs.
func (h *Handler) resolveAPI(ctx context.Context, ref APIRef) (apiID, apiName, apiVersion string, err error) {
	if ref.RefID != "" {
		cfg, lookupErr := h.db.GetConfigByCPArtifactID(ref.RefID)
		if lookupErr == nil {
			return cfg.UUID, cfg.DisplayName, cfg.Version, nil
		}
		if !storage.IsNotFoundError(lookupErr) {
			return "", "", "", fmt.Errorf("looking up API by ref_id %q: %w", ref.RefID, lookupErr)
		}
		// Not found by ref_id — fall through to name+version scan.
	}

	if ref.Name == "" {
		return "", "", "", fmt.Errorf("cannot resolve API: ref_id is empty and api.name is missing")
	}

	all, err := h.db.GetAllConfigs()
	if err != nil {
		return "", "", "", fmt.Errorf("loading configs for API name lookup: %w", err)
	}
	for _, cfg := range all {
		if strings.EqualFold(cfg.DisplayName, ref.Name) && strings.EqualFold(cfg.Version, ref.Version) {
			return cfg.UUID, cfg.DisplayName, cfg.Version, nil
		}
	}

	return "", "", "", fmt.Errorf("API %q version %q not found in gateway", ref.Name, ref.Version)
}

// resolvePlanID looks up the subscription plan by name (case-insensitive).
// Returns nil if not found; the subscription is still created without rate-limit enforcement.
func (h *Handler) resolvePlanID(planName string) (*string, error) {
	plans, err := h.db.ListSubscriptionPlans(h.gatewayID)
	if err != nil {
		return nil, err
	}
	for _, p := range plans {
		if p != nil && strings.EqualFold(p.PlanName, planName) {
			id := p.ID
			return &id, nil
		}
	}
	return nil, nil
}

// publishEvent fires an EventHub event for multi-replica sync.
// Errors are logged but not propagated — the event is already committed to DB
// and the failure only affects cross-replica propagation (replicas will resync on restart).
func (h *Handler) publishEvent(eventType eventhub.EventType, action, entityID string) {
	evt := eventhub.Event{
		EventType: eventType,
		Action:    action,
		EntityID:  entityID,
		EventID:   uuid.New().String(),
		EventData: eventhub.EmptyEventData,
	}
	if err := h.eventHub.PublishEvent(h.gatewayID, evt); err != nil {
		h.logger.Warn("Failed to publish EventHub event for devportal webhook",
			slog.String("event_type", string(eventType)),
			slog.String("action", action),
			slog.String("entity_id", entityID),
			slog.Any("error", err))
	}
}

// hashSHA256Hex hashes a plaintext API key using SHA-256 and returns a hex string.
// This matches the algorithm used by APIKeyService.hashAPIKeyWithSHA256.
func hashSHA256Hex(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// maskAPIKey masks an API key for safe logging, showing first 8 and last 4 characters.
func maskAPIKey(key string) string {
	if len(key) <= 12 {
		return "****"
	}
	return key[:8] + "****" + key[len(key)-4:]
}
