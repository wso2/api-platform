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
	"runtime/debug"
	"strings"
	"time"

	"github.com/wso2/api-platform/httpkit/httputil"

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
	CreateAPIKey(ctx context.Context, apiHandle, kind, orgID, userID string, req *api.CreateAPIKeyRequest) (*api.CreateAPIKeyResponse, error)
	UpdateAPIKey(ctx context.Context, apiHandle, kind, orgID, keyName, userID string, keyAdmin, trustedOrigin bool, req *api.UpdateAPIKeyRequest) error
	RevokeAPIKey(ctx context.Context, apiHandle, kind, orgID, keyName, userID string, keyAdmin, trustedOrigin bool) error
}

// subscriptionService is the subset of *service.SubscriptionService the receiver depends on.
type subscriptionService interface {
	CreateSubscriptionWithBilling(apiID, kind, orgUUID, subscriberID string, applicationID, subscriptionPlanID *string, subscriptionToken, status, actor string, billing *model.SubscriptionBilling) (*model.Subscription, error)
	ApplyBillingFromWebhook(orgUUID, apiHandle, kind, subscriberID string, billing *model.SubscriptionBilling) error
	UpdateSubscription(subscriptionID, orgUUID, subscriberID, status, actor string) (*model.Subscription, error)
	ChangePlan(subscriptionID, orgUUID, subscriberID, planUUID string) (*model.Subscription, error)
	RegenerateToken(subscriptionID, orgUUID, subscriberID, newToken string) (*model.Subscription, error)
	DeleteSubscription(subscriptionID, orgUUID, subscriberID, actor string) error
	FindByArtifactKindAndSubscriber(orgUUID, apiHandle, kind, subscriberID string) (*model.Subscription, error)
}

// organizationResolver resolves the API Portal organization handle (delivered as org.ref_id)
// to the Platform API organization UUID that every downstream service and table is keyed by.
// ErrSecretLookupFailed marks a failure to read a portal's secret from the store, as opposed to
// the store answering that no such secret exists.
var ErrSecretLookupFailed = errors.New("webhook secret lookup failed")

// maxPortalSecretCandidates bounds how many of an organization's portal secrets one delivery
// that carries no secret_hash is verified against, so an unauthenticated caller cannot make each
// request cost one HMAC per retained secret of a large organization.
const maxPortalSecretCandidates = 32

// portalSecretSource resolves the PORTAL webhook secrets of an organization. A portal provisioned
// with its own secret signs with it instead of cfg.Secret.
type portalSecretSource interface {
	// PortalSecretByHash returns the retained secret whose SecretHash is hash, or "" if none.
	PortalSecretByHash(ctx context.Context, orgID, hash string) (string, error)
	// PortalSecrets returns every retained secret, for portals that send no secret_hash.
	PortalSecrets(ctx context.Context, orgID string) ([]string, error)
}

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

// subscriptionPlanService is the subset of *service.SubscriptionPlanService the receiver depends
// on to copy plans created or changed in the API Portal.
type subscriptionPlanService interface {
	UpsertPlanFromWebhook(orgUUID, handle, name, description string, conditions *[]model.SubscriptionPlanCondition, requestWindows []model.ConditionWindow, pricing *model.SubscriptionPlanPricing, occurredAt time.Time) (*model.SubscriptionPlan, error)
	DeletePlanFromWebhook(orgUUID, handle string, occurredAt time.Time) error
}

// monetizationService is the subset of *service.MonetizationService the receiver depends on to
// keep copies of the meters made in the API Portal.
type monetizationService interface {
	UpsertMeterFromWebhook(meter *model.Meter) error
	DeleteMeterFromWebhook(orgUUID, handle string, occurredAt time.Time) error
}

// Receiver is the gin handler implementing the webhook processing flow.
type Receiver struct {
	cfg           config.Webhook
	verifier      *Verifier
	decryptor     *Decryptor
	portalSecrets portalSecretSource // optional; nil verifies against cfg.Secret only
	apiKeys       apiKeyService
	subs          subscriptionService
	apps          applicationService
	plans         subscriptionPlanService
	monetization  monetizationService
	orgs          organizationResolver
	readOnly      readOnlyMode // TEMP-READ-ONLY-MODE: remove with receiver_readonly.go
	slogger       *slog.Logger

	handlers map[string]func(ctx context.Context, env *Envelope) error
}

// NewReceiver wires the receiver and its event-type dispatch table.
//
// The verifier and the decryptor are both built from cfg.Secret here rather than passed in
// separately: the producer signs with that secret and derives its field-encryption key from the
// same secret, so keying the two off one config value is what keeps them from drifting apart.
func NewReceiver(
	cfg config.Webhook,
	apiKeys apiKeyService,
	subs subscriptionService,
	apps applicationService,
	plans subscriptionPlanService,
	monetization monetizationService,
	orgs organizationResolver,
	slogger *slog.Logger,
) (*Receiver, error) {
	decryptor, err := NewDecryptor(cfg.Secret)
	if err != nil {
		return nil, err
	}
	r := &Receiver{
		cfg:          cfg,
		verifier:     NewVerifier(cfg.Secret, cfg.SignatureTolerance),
		decryptor:    decryptor,
		apiKeys:      apiKeys,
		subs:         subs,
		apps:         apps,
		plans:        plans,
		orgs:         orgs,
		monetization: monetization,
		slogger:      slogger,
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
		EventSubscriptionPlanCreated:      r.handleSubscriptionPlanUpserted,
		EventSubscriptionPlanUpdated:      r.handleSubscriptionPlanUpserted,
		EventSubscriptionPlanDeleted:      r.handleSubscriptionPlanDeleted,
		EventSubscriptionBillingUpdated:   r.handleSubscriptionBillingUpdated,
		EventMeterCreated:                 r.handleMeterUpserted,
		EventMeterUpdated:                 r.handleMeterUpserted,
		EventMeterDeleted:                 r.handleMeterDeleted,
	}
	return r, nil
}

// SetPortalSecrets lets the receiver also accept deliveries signed with a per-portal secret.
// cfg.Secret stays accepted, so portals provisioned before per-portal secrets keep working.
func (r *Receiver) SetPortalSecrets(src portalSecretSource) { r.portalSecrets = src }

// RegisterRoutes registers the webhook endpoint on the mux. Only called when the webhook is enabled.
func (r *Receiver) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST "+RoutePath, middleware.MapErrors(r.slogger, r.rejectUniformly(r.ReceiveEvent)))
}

// rejectUniformly answers every failed delivery with one response, the unified 401, whatever the
// reason: a bad signature, an unknown organization or secret, an invalid envelope, a read-only
// organization, a store outage or a panic. The endpoint is reachable without authentication, so a
// status or code that differed by cause would tell a caller which organizations and secret hashes
// exist, how far a forged delivery got, and when the store is failing. The producer gains nothing
// from the difference: it marks any non-2xx delivery failed without retrying.
//
// The real reason still reaches the log only: every failure carries its original code, status and
// detail into the mapper's line, and a fault on this side (5xx) is also logged at ERROR with its
// stack, as the mapper would have logged it.
func (r *Receiver) rejectUniformly(next middleware.ErrorHandlerFunc) middleware.ErrorHandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) (err error) {
		defer func() {
			if rec := recover(); rec != nil {
				r.slogger.Error("Webhook delivery panicked", "panic", rec, "stack", string(debug.Stack()))
				err = apperror.Unauthorized.New().WithLogMessage("webhook delivery rejected: panic recovered")
			}
		}()

		if err = next(w, req); err == nil {
			return nil
		}
		cause := apperror.FromError(err)
		// A read-only rejection is an expected maintenance outcome, already logged at WARN.
		if cause.HTTPStatus >= http.StatusInternalServerError && !apperror.OrganizationReadOnly.Is(cause) {
			r.slogger.Error("Webhook delivery failed", "code", cause.Code, "status", cause.HTTPStatus,
				"detail", cause.LogMessage, "error", err, "origin", cause.Origin(), "stack", cause.StackString())
		}
		return apperror.Unauthorized.Wrap(err).WithLogMessage(fmt.Sprintf(
			"webhook delivery rejected (code=%s, status=%d): %s", cause.Code, cause.HTTPStatus, cause.LogMessage))
	}
}

// ReceiveEvent runs the full webhook flow: size-limited read -> signature verify -> envelope
// decode/validate -> idempotency -> dispatch -> mark processed. The errors it returns are
// classified for the log; the route answers every one of them alike (see rejectUniformly).
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
	decryptor, err := r.authenticate(req.Context(), req.Header.Get(r.cfg.SignatureHeader), body)
	if errors.Is(err, ErrSecretLookupFailed) {
		return apperror.Internal.Wrap(err).
			WithLogMessage("webhook portal secret lookup failed")
	}
	if err != nil {
		return apperror.Unauthorized.Wrap(err).
			WithLogMessage(fmt.Sprintf("webhook signature verification failed, clientIP=%s", req.RemoteAddr))
	}
	ctx := withDecryptor(req.Context(), decryptor)

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
	//    the UUID. An unknown handle is classified not-found, since the event references an
	//    organization that does not exist in the control plane.
	if err := r.resolveOrgUUID(env); err != nil {
		log.Warn("Webhook organization resolution failed", "error", err)
		return mapWebhookError(err)
	}
	log = log.With("orgId", env.OrgID)
	if err := r.rejectIfReadOnly(env, log); err != nil { // TEMP-READ-ONLY-MODE: remove with receiver_readonly.go
		return err
	}

	// 4. Dispatch to the matching handler. Duplicate (at-least-once) deliveries are made safe by
	//    each handler being idempotent by domain identity, so no envelope-level dedup is needed.
	handle, ok := r.handlers[env.EventType]
	if !ok {
		log.Warn("Unsupported webhook event type")
		return apperror.ValidationFailed.New("Unsupported event type")
	}

	if err := handle(ctx, env); err != nil {
		log.Error("Webhook event handling failed", "error", err)
		return mapWebhookError(err)
	}

	log.Info("Webhook event processed")
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "processed"})
	return nil
}

// authenticate verifies the signature and returns the decryptor keyed by the secret that matched,
// so field decryption uses the same secret the producer signed with.
//
// The envelope's org and secret_hash are read before anything is verified, so they only SELECT the
// secret and are never trusted: a delivery that names a secret_hash is checked against exactly the
// secret with that hash (a per-portal secret of that organization, or cfg.Secret), and is rejected
// when no such secret is held. Only a delivery with no secret_hash, from a portal that predates it,
// falls back to trying the organization's portal secrets and then cfg.Secret.
func (r *Receiver) authenticate(ctx context.Context, header string, body []byte) (*Decryptor, error) {
	now := time.Now()
	env, err := DecodeEnvelope(body)
	if err != nil {
		env = &Envelope{} // unreadable body: nothing selects a secret; the global verifier decides
	}

	if env.SecretHash != "" {
		secret, ok, err := r.secretForHash(ctx, env)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrSignatureInvalid
		}
		if err := NewVerifier(secret, r.cfg.SignatureTolerance).Verify(header, body, now); err != nil {
			return nil, err
		}
		return NewDecryptor(secret)
	}

	if !r.cfg.LegacySharedSecret {
		// Legacy (hash-less) deliveries are switched off: nothing selects a secret, so fail now.
		return nil, ErrSignatureInvalid
	}

	if r.portalSecrets != nil && strings.TrimSpace(header) != "" {
		candidates := r.portalCandidates(ctx, env)
		if len(candidates) > maxPortalSecretCandidates {
			candidates = candidates[:maxPortalSecretCandidates]
		}
		for _, secret := range candidates {
			if NewVerifier(secret, r.cfg.SignatureTolerance).Verify(header, body, now) != nil {
				continue
			}
			return NewDecryptor(secret)
		}
	}
	if err := r.verifier.Verify(header, body, now); err != nil {
		return nil, err
	}
	return r.decryptor, nil
}

// secretForHash finds the secret an envelope's secret_hash names: cfg.Secret when the hash is its
// hash (a portal seeded with the shared secret), else the organization's retained portal secret
// with that hash.
//
// A failed lookup is an error of its own (ErrSecretLookupFailed), not "no such secret", so the log
// tells a store outage from a bad signature. The caller sees the same response for both.
func (r *Receiver) secretForHash(ctx context.Context, env *Envelope) (string, bool, error) {
	if r.cfg.Secret != "" && env.SecretHash == SecretHash(r.cfg.Secret) {
		return r.cfg.Secret, true, nil
	}
	if r.portalSecrets == nil || env.OrgID == "" {
		return "", false, nil
	}
	org, err := r.orgs.GetOrganizationByHandle(env.OrgID)
	if err != nil {
		return "", false, fmt.Errorf("%w: %v", ErrSecretLookupFailed, err)
	}
	if org == nil {
		return "", false, nil // unknown organization: no secret to match
	}
	secret, err := r.portalSecrets.PortalSecretByHash(ctx, org.ID, env.SecretHash)
	if err != nil {
		return "", false, fmt.Errorf("%w: %v", ErrSecretLookupFailed, err)
	}
	return secret, secret != "", nil
}

// portalCandidates returns the retained portal secrets of the organization an envelope names, or
// nil when it names none or the lookup fails (cfg.Secret is still checked by the caller).
func (r *Receiver) portalCandidates(ctx context.Context, env *Envelope) []string {
	if env.OrgID == "" {
		return nil
	}
	org, err := r.orgs.GetOrganizationByHandle(env.OrgID)
	if err != nil || org == nil {
		return nil
	}
	secrets, err := r.portalSecrets.PortalSecrets(ctx, org.ID)
	if err != nil {
		r.slogger.Warn("Webhook portal secret lookup failed", "error", err)
		return nil
	}
	return secrets
}

type decryptorCtxKey struct{}

// withDecryptor carries the decryptor chosen during authentication to the event handlers.
func withDecryptor(ctx context.Context, d *Decryptor) context.Context {
	return context.WithValue(ctx, decryptorCtxKey{}, d)
}

// decryptorFor returns the decryptor authentication selected, falling back to the configured one.
func (r *Receiver) decryptorFor(ctx context.Context) *Decryptor {
	if d, ok := ctx.Value(decryptorCtxKey{}).(*Decryptor); ok && d != nil {
		return d
	}
	return r.decryptor
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
