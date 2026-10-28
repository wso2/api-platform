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

package webhook

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/wso2/api-platform/httpkit/httputil"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// RoutePath is the legacy webhook endpoint. Mounted only when
// config.Webhook.LegacySharedSecret is true; verified with the global
// webhook.secret. Replaced per-portal by PerPortalRoutePath; kept for the
// migration window and removed once every portal subscriber row points at the
// per-portal URL.
const RoutePath = "/api/internal/" + constants.APIVersion + "/webhook/events"

// PerPortalRoutePath carries the portal uuid in the path. The id is not a
// credential — the HMAC is still the authentication; the path selects which
// per-portal secret to verify with. After HMAC passes, the receiver rejects the
// delivery when payload.org.ref_id does not resolve to the portal's own org.
const PerPortalRoutePath = "/api/internal/" + constants.APIVersion + "/webhook/portals/{portalUUID}/events"

// apiKeyService is the subset of *service.APIKeyService the receiver depends on. The existing
// CreateAPIKey/UpdateAPIKey/RevokeAPIKey already hash an externally provisioned key, persist it,
// and broadcast to the gateways where the API is deployed — exactly what the webhook needs.
type apiKeyService interface {
	CreateAPIKey(ctx context.Context, apiHandle, kind, orgID, userID string, req *api.CreateAPIKeyRequest) (*api.CreateAPIKeyResponse, error)
	UpdateAPIKey(ctx context.Context, apiHandle, kind, orgID, keyName, userID string, keyAdmin, trustedOrigin bool, req *api.UpdateAPIKeyRequest) error
	RevokeAPIKey(ctx context.Context, apiHandle, kind, orgID, keyName, userID string, keyAdmin, trustedOrigin bool) error
}

// subscriptionService is the subset of *service.SubscriptionService the receiver depends on.
type subscriptionService interface {
	CreateSubscription(apiID, kind, orgUUID, subscriberID string, applicationID, subscriptionPlanID *string, subscriptionToken, status, actor string) (*model.Subscription, error)
	UpdateSubscription(subscriptionID, orgUUID, subscriberID, status, actor string) (*model.Subscription, error)
	ChangePlan(subscriptionID, orgUUID, subscriberID, planUUID string) (*model.Subscription, error)
	RegenerateToken(subscriptionID, orgUUID, subscriberID, newToken string) (*model.Subscription, error)
	DeleteSubscription(subscriptionID, orgUUID, subscriberID, actor string) error
	FindByArtifactKindAndSubscriber(orgUUID, apiHandle, kind, subscriberID string) (*model.Subscription, error)
}

// organizationResolver resolves the API Portal organization handle (delivered as org.ref_id)
// to the Platform API organization UUID that every downstream service and table is keyed by.
type organizationResolver interface {
	GetOrganizationByHandle(handle string) (*model.Organization, error)
}

// applicationService is the subset of *service.ApplicationService the receiver depends on to
// reconcile API Portal application events. The webhook-specific create/reconcile logic (default
// project, "genai" type, DP-id-as-handle, single-app key mapping) lives in the service.
type applicationService interface {
	CreateApplicationFromWebhook(handle, name, description, appType, orgID string) (*api.Application, error)
	UpdateApplication(appIDOrHandle string, req *api.Application, orgID, userID string) (*api.Application, error)
	DeleteApplication(appIDOrHandle, orgID, actor string) error
	SetAPIKeyApplication(keyName, artifactRef, kind, appIDOrHandle, orgID, userID string) error
}

// Receiver is the gin handler implementing the webhook processing flow.
// verifier + decryptor serve the legacy /webhook/events route and are nil when
// LegacySharedSecret is off or Secret is empty. keyRegistry serves the
// per-portal route (/webhook/portals/{portalUUID}/events) and is nil only in
// test setups that do not exercise that path.
type Receiver struct {
	cfg         config.Webhook
	verifier    *Verifier
	decryptor   *Decryptor
	keyRegistry PortalKeyProvider
	apiKeys     apiKeyService
	subs        subscriptionService
	apps        applicationService
	orgs        organizationResolver
	readOnly    readOnlyMode // TEMP-READ-ONLY-MODE: remove with receiver_readonly.go
	slogger     *slog.Logger

	handlers map[string]func(ctx context.Context, env *Envelope) error
}

// NewReceiver wires the receiver and its event-type dispatch table.
//
// Each per-portal signing secret is bound to the portal uuid; the verifier +
// decryptor pair for a request are resolved from keyRegistry at request time,
// keyed on the uuid in PerPortalRoutePath. The legacy cfg.Secret is used only
// by the shared-secret /webhook/events route and is built here only when
// cfg.LegacySharedSecret is true and cfg.Secret is non-empty — the two must
// stay in lockstep since the field-encryption key is derived from the signing
// secret.
func NewReceiver(
	cfg config.Webhook,
	keyRegistry PortalKeyProvider,
	apiKeys apiKeyService,
	subs subscriptionService,
	apps applicationService,
	orgs organizationResolver,
	slogger *slog.Logger,
) (*Receiver, error) {
	var (
		legacyVerifier  *Verifier
		legacyDecryptor *Decryptor
	)
	if cfg.LegacySharedSecret && cfg.Secret != "" {
		dec, err := NewDecryptor(cfg.Secret)
		if err != nil {
			return nil, err
		}
		legacyDecryptor = dec
		legacyVerifier = NewVerifier(cfg.Secret, cfg.SignatureTolerance)
	}
	r := &Receiver{
		cfg:         cfg,
		verifier:    legacyVerifier,
		decryptor:   legacyDecryptor,
		keyRegistry: keyRegistry,
		apiKeys:     apiKeys,
		subs:        subs,
		apps:        apps,
		orgs:        orgs,
		slogger:     slogger,
	}
	r.handlers = map[string]func(ctx context.Context, env *Envelope) error{
		EventAPIKeyGenerated:              r.handleAPIKeyGenerated,
		EventAPIKeyRegenerated:            r.handleAPIKeyRegenerated,
		EventAPIKeyRevoked:                r.handleAPIKeyRevoked,
		EventSubscriptionCreated:          r.handleSubscriptionCreated,
		EventSubscriptionUpdated:          r.handleSubscriptionUpdated,
		EventSubscriptionPlanChanged:      r.handleSubscriptionPlanChanged,
		EventSubscriptionTokenRegenerated: r.handleSubscriptionTokenRegenerated,
		EventSubscriptionDeleted:          r.handleSubscriptionDeleted,
		EventApplicationCreated:           r.handleApplicationCreated,
		EventApplicationUpdated:           r.handleApplicationUpdated,
		EventApplicationDeleted:           r.handleApplicationDeleted,
		EventAPIKeyApplicationUpdated:     r.handleAPIKeyApplicationUpdated,
	}
	return r, nil
}

// RegisterRoutes registers the webhook endpoints on the mux. The per-portal
// route is mounted whenever keyRegistry is non-nil; the legacy route is mounted
// only while LegacySharedSecret is true and Secret is set. Called only when the
// receiver itself is enabled.
func (r *Receiver) RegisterRoutes(mux *http.ServeMux) {
	if r.keyRegistry != nil {
		mux.HandleFunc("POST "+PerPortalRoutePath, middleware.MapErrors(r.slogger, r.ReceivePerPortalEvent))
	}
	if r.verifier != nil {
		mux.HandleFunc("POST "+RoutePath, middleware.MapErrors(r.slogger, r.ReceiveEvent))
	}
}

// ReceiveEvent runs the full webhook flow: size-limited read -> signature verify -> envelope
// decode/validate -> org resolve -> dispatch. Events are not persisted; duplicate deliveries are
// absorbed by each handler being idempotent by domain identity.
func (r *Receiver) ReceiveEvent(w http.ResponseWriter, req *http.Request) error {
	if !r.cfg.Enabled {
		return apperror.NotFound.New().WithLogMessage("webhook endpoint is disabled")
	}

	// Read the raw body with a size cap. The raw bytes are needed verbatim for HMAC verification.
	limited := http.MaxBytesReader(w, req.Body, r.cfg.MaxBodySize)
	body, err := io.ReadAll(limited)
	if err != nil {
		return apperror.ValidationFailed.Wrap(err, "Request body too large or unreadable").
			WithLogMessage(fmt.Sprintf("webhook body read failed (possibly too large), limit=%d", r.cfg.MaxBodySize))
	}

	// 1. Verify HMAC signature over the raw body. Per the unified auth-failure rule, this returns the
	// same generic response as any other auth failure; the specific reason is internal-only.
	if err := r.verifier.Verify(req.Header.Get(r.cfg.SignatureHeader), body, time.Now()); err != nil {
		return apperror.Unauthorized.Wrap(err).
			WithLogMessage(fmt.Sprintf("webhook signature verification failed, clientIP=%s", req.RemoteAddr))
	}

	// 2. Decode + validate the envelope.
	env, err := DecodeEnvelope(body)
	if err == nil {
		err = env.Validate()
	}
	if err != nil {
		return apperror.ValidationFailed.Wrap(err, "Invalid event envelope")
	}

	log := r.slogger.With("eventId", env.EventID, "eventType", env.EventType, "orgHandle", env.OrgID)

	// 3. Resolve the organization handle (org.ref_id) to the Platform API organization UUID that all
	//    downstream persistence is keyed by. The API Portal sends the handle; the services expect
	//    the UUID. An unknown handle is a terminal 404 (not retryable) since the event references an
	//    organization that does not exist in the control plane.
	if err := r.resolveOrgUUID(env); err != nil {
		log.Warn("Webhook organization resolution failed", "error", err)
		return mapWebhookError(err)
	}
	log = log.With("orgId", env.OrgID)
	if r.rejectIfReadOnly(w, env, log) { // TEMP-READ-ONLY-MODE: remove with receiver_readonly.go
		return nil
	}

	// 4. Dispatch to the matching handler. Duplicate (at-least-once) deliveries are made safe by
	//    each handler being idempotent by domain identity, so no envelope-level dedup is needed.
	handle, ok := r.handlers[env.EventType]
	if !ok {
		log.Warn("Unsupported webhook event type")
		return apperror.ValidationFailed.New("Unsupported event type")
	}

	if err := handle(req.Context(), env); err != nil {
		log.Error("Webhook event handling failed", "error", err)
		return mapWebhookError(err)
	}

	log.Info("Webhook event processed")
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "processed"})
	return nil
}

// ReceivePerPortalEvent runs the per-portal webhook flow: resolve portal uuid
// from the URL path, pick that portal's current (and in-grace previous) secret,
// verify the HMAC, decode and validate the envelope, enforce the org binding,
// and dispatch to the shared handler table with the per-portal field-decryption
// key threaded through the request context.
//
// An unknown portal uuid, a secret-row-missing portal, and a bad signature all
// surface as the same generic 401 — the response must not reveal whether the
// uuid identifies a real portal. A valid signature whose payload org.ref_id
// resolves to a different org than this portal owns is a 403 (cross-org
// forgery attempt) with the portal id and both org ids in the log only.
func (r *Receiver) ReceivePerPortalEvent(w http.ResponseWriter, req *http.Request) error {
	if !r.cfg.Enabled {
		return apperror.NotFound.New().WithLogMessage("webhook endpoint is disabled")
	}
	if r.keyRegistry == nil {
		return apperror.NotFound.New().WithLogMessage("per-portal webhook endpoint is disabled")
	}

	portalUUID := req.PathValue("portalUUID")
	if portalUUID == "" {
		return apperror.Unauthorized.New().
			WithLogMessage("per-portal webhook: missing portalUUID path value")
	}

	limited := http.MaxBytesReader(w, req.Body, r.cfg.MaxBodySize)
	body, err := io.ReadAll(limited)
	if err != nil {
		return apperror.ValidationFailed.Wrap(err, "Request body too large or unreadable").
			WithLogMessage(fmt.Sprintf("webhook body read failed (possibly too large), limit=%d", r.cfg.MaxBodySize))
	}

	keys, err := r.keyRegistry.Get(req.Context(), portalUUID)
	if err != nil {
		// Keep the response generic so an attacker cannot enumerate portal uuids
		// — both "unknown portal" and "bad signature" look identical on the wire.
		return apperror.Unauthorized.Wrap(err).
			WithLogMessage(fmt.Sprintf("per-portal webhook: key load failed, portal=%s clientIP=%s", portalUUID, req.RemoteAddr))
	}

	sigHeader := req.Header.Get(r.cfg.SignatureHeader)
	now := time.Now()
	var requestDecryptor *Decryptor
	if verr := keys.Current.Verify(sigHeader, body, now); verr == nil {
		requestDecryptor = keys.CurrentDecryptor
	} else if keys.HasPreviousInGrace(now) {
		if perr := keys.Previous.Verify(sigHeader, body, now); perr != nil {
			return apperror.Unauthorized.Wrap(perr).
				WithLogMessage(fmt.Sprintf("per-portal webhook signature verification failed under both current and previous (grace) secrets, portal=%s clientIP=%s", portalUUID, req.RemoteAddr))
		}
		requestDecryptor = keys.PreviousDecryptor
	} else {
		return apperror.Unauthorized.Wrap(verr).
			WithLogMessage(fmt.Sprintf("per-portal webhook signature verification failed, portal=%s clientIP=%s", portalUUID, req.RemoteAddr))
	}

	env, err := DecodeEnvelope(body)
	if err == nil {
		err = env.Validate()
	}
	if err != nil {
		return apperror.ValidationFailed.Wrap(err, "Invalid event envelope")
	}

	log := r.slogger.With("eventId", env.EventID, "eventType", env.EventType, "orgHandle", env.OrgID, "portalId", portalUUID)

	if err := r.resolveOrgUUID(env); err != nil {
		log.Warn("Webhook organization resolution failed", "error", err)
		// On the per-portal route, an unresolved payload org handle is treated as a
		// cross-org mismatch (FR-004): it collapses into the same 403 the explicit
		// org-binding check below returns. Surfacing it as 404 would be an existence
		// oracle for organisation handles — a caller probing the receiver could
		// enumerate which handles exist by watching 404 vs 403, defeating the
		// indistinguishability promise in FR-017 / SC-007. The legacy route in
		// ReceiveEvent keeps the 404 passthrough; only the per-portal path collapses
		// here.
		if apperror.OrganizationNotFound.Is(err) {
			return apperror.Forbidden.New().WithLogMessage(fmt.Sprintf(
				"per-portal webhook: unresolved payload org handle, portal=%s orgHandle=%q", portalUUID, env.OrgID))
		}
		return mapWebhookError(err)
	}
	log = log.With("orgId", env.OrgID)

	// Org binding: the HMAC proves "signed with this portal's secret"; it does
	// not prove the delivery is for this portal's org. Reject any event whose
	// resolved org uuid is not the one that owns this portal.
	if env.OrgID != keys.OrganizationUUID {
		log.Warn("Webhook org binding mismatch",
			"portalOrgId", keys.OrganizationUUID,
			"eventOrgId", env.OrgID)
		return apperror.Forbidden.New().
			WithLogMessage(fmt.Sprintf("per-portal webhook org mismatch: portal=%s portalOrg=%s eventOrg=%s", portalUUID, keys.OrganizationUUID, env.OrgID))
	}

	if r.rejectIfReadOnly(w, env, log) { // TEMP-READ-ONLY-MODE: remove with receiver_readonly.go
		return nil
	}

	handle, ok := r.handlers[env.EventType]
	if !ok {
		log.Warn("Unsupported webhook event type")
		return apperror.ValidationFailed.New("Unsupported event type")
	}

	ctx := contextWithDecryptor(req.Context(), requestDecryptor)
	if err := handle(ctx, env); err != nil {
		log.Error("Webhook event handling failed", "error", err)
		return mapWebhookError(err)
	}

	log.Info("Webhook event processed")
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "processed"})
	return nil
}

// resolveOrgUUID converts the organization handle carried in the envelope (org.ref_id, normalized
// onto env.OrgID by DecodeEnvelope) into the Platform API organization UUID and writes it back onto
// env.OrgID, so downstream handlers key their persistence by the UUID. An unknown handle returns
// ErrOrganizationNotFound.
func (r *Receiver) resolveOrgUUID(env *Envelope) error {
	org, err := r.orgs.GetOrganizationByHandle(env.OrgID)
	if err != nil {
		return err
	}
	if org == nil {
		return apperror.OrganizationNotFound.New()
	}
	env.OrgID = org.ID
	return nil
}

// mapWebhookError maps errors raised while processing a webhook event to an
// apperror catalog entry. The services this receiver reuses now build their
// failures from the catalog, so a domain error already carries the right code
// and status and passes through untouched; only the receiver's own envelope
// sentinels and unclassified failures need mapping here.
func mapWebhookError(err error) *apperror.Error {
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return appErr
	}
	// Envelope-level failures are the receiver's own sentinels, not catalog errors.
	if errors.Is(err, ErrInvalidEnvelope) || errors.Is(err, ErrUnsupportedEvent) || errors.Is(err, ErrDecryptionFailed) {
		return apperror.ValidationFailed.Wrap(err, "Failed to process event")
	}
	// Storage/EventHub failures and unexpected errors are retryable.
	return apperror.Internal.Wrap(err).WithLogMessage("failed to process webhook event")
}
