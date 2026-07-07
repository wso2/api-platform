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

	"github.com/wso2/go-httpkit/httputil"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/model"
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
	CreateSubscription(apiID, kind, orgUUID, subscriberID string, applicationID, subscriptionPlanID *string, subscriptionToken, status, actor string) (*model.Subscription, error)
	UpdateSubscription(subscriptionID, orgUUID, subscriberID, status, actor string) (*model.Subscription, error)
	ChangePlan(subscriptionID, orgUUID, subscriberID, planUUID string) (*model.Subscription, error)
	RegenerateToken(subscriptionID, orgUUID, subscriberID, newToken string) (*model.Subscription, error)
	DeleteSubscription(subscriptionID, orgUUID, subscriberID, actor string) error
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
	mux.HandleFunc("POST "+RoutePath, middleware.MapErrors(r.slogger, r.ReceiveEvent))
}

// ReceiveEvent runs the full webhook flow: size-limited read -> signature verify -> envelope
// decode/validate -> gateway_type filter -> idempotency -> dispatch -> mark processed.
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
	//    downstream persistence is keyed by. The Developer Portal sends the handle; the services expect
	//    the UUID. An unknown handle is a terminal 404 (not retryable) since the event references an
	//    organization that does not exist in the control plane.
	if err := r.resolveOrgUUID(env); err != nil {
		log.Warn("Webhook organization resolution failed", "error", err)
		return mapWebhookError(err)
	}
	log = log.With("orgId", env.OrgID)

	// 4. gateway_type filter — events for other gateway types are a no-op (accepted, not processed).
	if r.cfg.GatewayType != "" && env.GatewayType != "" && env.GatewayType != r.cfg.GatewayType {
		log.Info("Webhook event for a different gateway_type; accepting as no-op", "eventGatewayType", env.GatewayType)
		httputil.WriteJSON(w, http.StatusAccepted, map[string]string{"status": "ignored", "reason": "gateway_type mismatch"})
		return nil
	}

	// 5. Dispatch to the matching handler. Duplicate (at-least-once) deliveries are made safe by
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

// mapWebhookError maps domain errors returned by the reused services to the matching apperror
// catalog entry, preserving the same HTTP status classification the old statusForError gave them.
func mapWebhookError(err error) *apperror.Error {
	switch {
	case errors.Is(err, ErrInvalidEnvelope), errors.Is(err, ErrUnsupportedEvent), errors.Is(err, ErrDecryptionFailed):
		return apperror.ValidationFailed.Wrap(err, "Failed to process event")
	case errors.Is(err, constants.ErrOrganizationNotFound):
		return apperror.OrganizationNotFound.Wrap(err)
	case errors.Is(err, constants.ErrSubscriptionNotFound):
		return apperror.SubscriptionNotFound.Wrap(err)
	case errors.Is(err, constants.ErrSubscriptionPlanNotFound), errors.Is(err, constants.ErrSubscriptionPlanNotFoundOrInactive):
		return apperror.SubscriptionPlanNotFound.Wrap(err)
	case errors.Is(err, constants.ErrAPINotFound), errors.Is(err, constants.ErrAPIKeyNotFound):
		return apperror.ArtifactNotFound.Wrap(err)
	default:
		// Storage/EventHub failures and unexpected errors are retryable.
		return apperror.Internal.Wrap(err).WithLogMessage("failed to process webhook event")
	}
}
