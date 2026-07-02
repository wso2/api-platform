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
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/wso2/go-httpkit/httputil"

	"platform-api/src/api"
	"platform-api/src/config"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
	"platform-api/src/internal/utils"
)

// RoutePath is the webhook endpoint under the resource API version prefix (constants.APIVersion,
// currently "v0.9"). It lives under the internal/control-plane prefix and is excluded from JWT/IDP
// auth via config.Auth.SkipPaths (authenticity comes from the HMAC signature).
const RoutePath = "/api/internal/" + constants.APIVersion + "/webhook/events"

// apiKeyService is the subset of *service.APIKeyService the receiver depends on. The existing
// CreateAPIKey/UpdateAPIKey/RevokeAPIKey already hash an externally provisioned key, persist it,
// and broadcast to the gateways where the API is deployed — exactly what the webhook needs.
type apiKeyService interface {
	CreateAPIKey(ctx context.Context, apiHandle, kind, orgID, userID string, req *api.CreateAPIKeyRequest) error
	UpdateAPIKey(ctx context.Context, apiHandle, kind, orgID, keyName, userID string, req *api.UpdateAPIKeyRequest) error
	RevokeAPIKey(ctx context.Context, apiHandle, kind, orgID, keyName, userID string) error
}

// subscriptionService is the subset of *service.SubscriptionService the receiver depends on.
type subscriptionService interface {
	CreateSubscription(apiID, kind, orgUUID, subscriberID string, applicationID, subscriptionPlanID *string, subscriptionToken, status string) (*model.Subscription, error)
	UpdateSubscription(subscriptionID, orgUUID, subscriberID, status string) (*model.Subscription, error)
	ChangePlan(subscriptionID, orgUUID, subscriberID, planUUID string) (*model.Subscription, error)
	RegenerateToken(subscriptionID, orgUUID, subscriberID, newToken string) (*model.Subscription, error)
	DeleteSubscription(subscriptionID, orgUUID, subscriberID string) error
	FindByArtifactKindAndSubscriber(orgUUID, apiHandle, kind, subscriberID string) (*model.Subscription, error)
}

// organizationResolver resolves the Developer Portal organization handle (delivered as org.ref_id)
// to the Platform API organization UUID that every downstream service and table is keyed by.
type organizationResolver interface {
	GetOrganizationByHandle(handle string) (*model.Organization, error)
}

// applicationService is the subset of *service.ApplicationService the receiver depends on to
// reconcile Developer Portal application events. The webhook-specific create/reconcile logic (default
// project, "genai" type, DP-id-as-handle, single-app key mapping) lives in the service.
type applicationService interface {
	CreateApplicationFromWebhook(handle, name, description, appType, orgID string) (*api.Application, error)
	UpdateApplication(appIDOrHandle string, req *api.Application, orgID, userID string) (*api.Application, error)
	DeleteApplication(appIDOrHandle, orgID, actor string) error
	SetAPIKeyApplication(keyName, artifactRef, kind, appIDOrHandle, orgID, userID string) error
}

// Receiver is the gin handler implementing the webhook processing flow.
type Receiver struct {
	cfg       config.Webhook
	verifier  *Verifier
	decryptor *Decryptor
	apiKeys   apiKeyService
	subs      subscriptionService
	apps      applicationService
	orgs      organizationResolver
	slogger   *slog.Logger

	handlers map[string]func(ctx context.Context, env *Envelope) error
}

// NewReceiver wires the receiver and its event-type dispatch table.
func NewReceiver(
	cfg config.Webhook,
	decryptor *Decryptor,
	apiKeys apiKeyService,
	subs subscriptionService,
	apps applicationService,
	orgs organizationResolver,
	slogger *slog.Logger,
) *Receiver {
	r := &Receiver{
		cfg:       cfg,
		verifier:  NewVerifier(cfg.Secret, cfg.SignatureTolerance),
		decryptor: decryptor,
		apiKeys:   apiKeys,
		subs:      subs,
		apps:      apps,
		orgs:      orgs,
		slogger:   slogger,
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
	return r
}

// RegisterRoutes registers the webhook endpoint on the mux. Only called when the webhook is enabled.
func (r *Receiver) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST "+RoutePath, r.ReceiveEvent)
}

// ReceiveEvent runs the full webhook flow: size-limited read -> signature verify -> envelope
// decode/validate -> gateway_type filter -> idempotency -> dispatch -> mark processed.
func (r *Receiver) ReceiveEvent(w http.ResponseWriter, req *http.Request) {
	if !r.cfg.Enabled {
		httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(http.StatusNotFound, "Not Found", "Webhook endpoint is disabled"))
		return
	}

	// Read the raw body with a size cap. The raw bytes are needed verbatim for HMAC verification.
	limited := http.MaxBytesReader(w, req.Body, r.cfg.MaxBodySize)
	body, err := io.ReadAll(limited)
	if err != nil {
		r.slogger.Warn("Webhook body read failed (possibly too large)", "limit", r.cfg.MaxBodySize, "error", err)
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(http.StatusBadRequest, "Bad Request", "Request body too large or unreadable"))
		return
	}

	// 1. Verify HMAC signature over the raw body.
	if err := r.verifier.Verify(req.Header.Get(r.cfg.SignatureHeader), body, time.Now()); err != nil {
		r.slogger.Warn("Webhook signature verification failed", "clientIP", req.RemoteAddr, "error", err)
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(http.StatusUnauthorized, "Unauthorized", "Signature verification failed"))
		return
	}

	// 2. Decode + validate the envelope.
	env, err := DecodeEnvelope(body)
	if err == nil {
		err = env.Validate()
	}
	if err != nil {
		r.slogger.Warn("Webhook envelope invalid", "error", err)
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(http.StatusBadRequest, "Bad Request", "Invalid event envelope"))
		return
	}

	log := r.slogger.With("eventId", env.EventID, "eventType", env.EventType, "orgHandle", env.OrgID)

	// 3. Resolve the organization handle (org.ref_id) to the Platform API organization UUID that all
	//    downstream persistence is keyed by. The Developer Portal sends the handle; the services expect
	//    the UUID. An unknown handle is a terminal 404 (not retryable) since the event references an
	//    organization that does not exist in the control plane.
	if err := r.resolveOrgUUID(env); err != nil {
		status := statusForError(err)
		log.Warn("Webhook organization resolution failed", "status", status, "error", err)
		httputil.WriteJSON(w, status, utils.NewErrorResponse(status, http.StatusText(status), "Unknown organization"))
		return
	}
	log = log.With("orgId", env.OrgID)

	// 4. gateway_type filter — events for other gateway types are a no-op (accepted, not processed).
	if r.cfg.GatewayType != "" && env.GatewayType != "" && env.GatewayType != r.cfg.GatewayType {
		log.Info("Webhook event for a different gateway_type; accepting as no-op", "eventGatewayType", env.GatewayType)
		httputil.WriteJSON(w, http.StatusAccepted, map[string]string{"status": "ignored", "reason": "gateway_type mismatch"})
		return
	}

	// 5. Dispatch to the matching handler. Duplicate (at-least-once) deliveries are made safe by
	//    each handler being idempotent by domain identity, so no envelope-level dedup is needed.
	handle, ok := r.handlers[env.EventType]
	if !ok {
		log.Warn("Unsupported webhook event type")
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(http.StatusBadRequest, "Bad Request", "Unsupported event type"))
		return
	}

	if err := handle(req.Context(), env); err != nil {
		status := statusForError(err)
		log.Error("Webhook event handling failed", "status", status, "error", err)
		httputil.WriteJSON(w, status, utils.NewErrorResponse(status, http.StatusText(status), "Failed to process event"))
		return
	}

	log.Info("Webhook event processed")
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "processed"})
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
		return constants.ErrOrganizationNotFound
	}
	env.OrgID = org.ID
	return nil
}

// statusForError maps domain errors returned by the reused services to HTTP status codes.
func statusForError(err error) int {
	switch {
	case errors.Is(err, ErrInvalidEnvelope), errors.Is(err, ErrUnsupportedEvent), errors.Is(err, ErrDecryptionFailed):
		return http.StatusBadRequest
	case errors.Is(err, constants.ErrAPINotFound),
		errors.Is(err, constants.ErrOrganizationNotFound),
		errors.Is(err, constants.ErrSubscriptionNotFound),
		errors.Is(err, constants.ErrSubscriptionPlanNotFound),
		errors.Is(err, constants.ErrSubscriptionPlanNotFoundOrInactive),
		errors.Is(err, constants.ErrAPIKeyNotFound):
		return http.StatusNotFound
	default:
		// Storage/EventHub failures and unexpected errors are retryable.
		return http.StatusInternalServerError
	}
}
