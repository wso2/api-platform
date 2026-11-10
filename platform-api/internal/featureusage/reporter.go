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

// Package featureusage reports backend feature usage to WSO2 Cloud's Moesif
// collector proxy as Moesif API events (POST {collector}/v1/events,
// authenticated with a Platform IdP M2M token): one event per call to a REST
// API route that serves a taxonomy feature — successful or failed, with the
// status it answered — carrying the feature code in metadata.growth_action
// and the taxonomy dimensions alongside it. UI
// interactions are not reported here; the console reports those as actions.
//
// The proxy only authenticates and forwards; it validates and sanitizes
// nothing. So everything that decides what leaves the process happens here,
// before an event is queued: the feature must be bound to a REST API operation
// (bindings.go) and in the embedded taxonomy as tracked from the API, every dimension must
// be declared by it with a value of the declared type (enum members, booleans,
// or codes), the call is described only by verb, route template and status,
// and the organization and user are sent only as canonical UUIDs. A rejected
// occurrence is logged by feature code and reason, never by value.
//
// Reporting never fails or slows the request that triggered it: Record only
// validates and enqueues onto a bounded queue, dropping the event when the
// queue is full, and delivery runs on background workers with short retries.
package featureusage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// Reporter records feature-usage occurrences.
type Reporter interface {
	// Record validates o and queues it for delivery. It never blocks on the
	// network and never returns an error to the caller: a problem is logged
	// and the occurrence dropped.
	Record(o Occurrence)
	// Close stops accepting occurrences and waits, until ctx is done, for the
	// queued ones to be delivered.
	Close(ctx context.Context) error
}

// Attributor maps the platform organization onto the key Moesif uses, so one
// organization is one Moesif record whichever WSO2 Cloud service reported it.
// It is called on a background worker, never on the request. The user needs
// no mapping: the occurrence carries the identity-provider subject itself.
type Attributor interface {
	// CompanyID returns the organization's Moesif company_id: its identity
	// provider organization UUID, or the platform UUID when it has none.
	CompanyID(orgUUID string) (string, error)
}

// noop is the Reporter used when reporting is disabled.
type noop struct{}

func (noop) Record(Occurrence)           {}
func (noop) Close(context.Context) error { return nil }

// Noop returns a Reporter that records nothing.
func Noop() Reporter { return noop{} }

// logParty tags this package's log lines, matching WSO2 Cloud's REPORT_DROPPED
// convention so drops from every publisher can be found the same way.
const logParty = "moesif"

// Options configures a Publisher. Every field is required unless noted.
type Options struct {
	CollectorURL   string
	HTTPClient     *http.Client
	Tokens         TokenSource
	Attributor     Attributor
	Logger         *slog.Logger
	QueueSize      int
	Workers        int
	MaxAttempts    int
	RetryBackoff   time.Duration // may be zero
	AttemptTimeout time.Duration
	OverallTimeout time.Duration
	// Environment, when set, is sent as every event's "environment" metadata.
	// It must be a code (see stringDimensionPattern).
	Environment string
	// Taxonomy defaults to the embedded API Platform taxonomy.
	Taxonomy *Taxonomy
	// Now defaults to time.Now.
	Now func() time.Time
}

// Publisher is the Reporter that delivers to the collector proxy.
type Publisher struct {
	taxonomy    *Taxonomy
	enabled     map[string]bool
	attributor  Attributor
	client      *collectorClient
	logger      *slog.Logger
	now         func() time.Time
	overall     time.Duration
	environment string

	mu     sync.RWMutex // guards closed and sends on queue
	closed bool
	queue  chan *validatedEvent

	baseCtx    context.Context
	cancelBase context.CancelFunc
	workers    sync.WaitGroup
}

// NewPublisher validates opts, checks every bound feature (bindings.go)
// against the taxonomy, and starts the delivery workers.
func NewPublisher(opts Options) (*Publisher, error) {
	switch {
	case opts.HTTPClient == nil, opts.Tokens == nil, opts.Attributor == nil, opts.Logger == nil:
		return nil, errors.New("feature usage: HTTPClient, Tokens, Attributor and Logger are required")
	case opts.QueueSize < 1, opts.Workers < 1, opts.MaxAttempts < 1:
		return nil, errors.New("feature usage: QueueSize, Workers and MaxAttempts must be positive")
	case opts.AttemptTimeout <= 0, opts.OverallTimeout < opts.AttemptTimeout, opts.RetryBackoff < 0:
		return nil, errors.New("feature usage: invalid timeouts")
	}
	base := strings.TrimRight(strings.TrimSpace(opts.CollectorURL), "/")
	if base == "" {
		return nil, errors.New("feature usage: CollectorURL is required")
	}
	if opts.Environment != "" && !stringDimensionPattern.MatchString(opts.Environment) {
		return nil, errors.New("feature usage: Environment must be a code")
	}

	tax := opts.Taxonomy
	if tax == nil {
		var err error
		if tax, err = DefaultTaxonomy(); err != nil {
			return nil, fmt.Errorf("feature usage: embedded taxonomy: %w", err)
		}
	}
	if err := checkEnabledFeatures(tax, enabledFeatures); err != nil {
		return nil, err
	}
	if err := checkBindings(tax, bindings); err != nil {
		return nil, err
	}

	now := opts.Now
	if now == nil {
		now = time.Now
	}
	ctx, cancel := context.WithCancel(context.Background())
	p := &Publisher{
		taxonomy:   tax,
		enabled:    enabledFeatures,
		attributor: opts.Attributor,
		client: &collectorClient{
			httpClient:     opts.HTTPClient,
			eventsURL:      base + eventsPath,
			tokens:         opts.Tokens,
			maxAttempts:    opts.MaxAttempts,
			retryBackoff:   opts.RetryBackoff,
			attemptTimeout: opts.AttemptTimeout,
		},
		logger:      opts.Logger,
		now:         now,
		overall:     opts.OverallTimeout,
		environment: opts.Environment,
		queue:       make(chan *validatedEvent, opts.QueueSize),
		baseCtx:     ctx,
		cancelBase:  cancel,
	}
	for i := 0; i < opts.Workers; i++ {
		p.workers.Add(1)
		go p.work()
	}
	return p, nil
}

// checkEnabledFeatures makes a reported code that the taxonomy does not
// define, defines under a different event name, or does not track from the
// API a startup error, not a silent stream of rejections.
func checkEnabledFeatures(tax *Taxonomy, enabled map[string]bool) error {
	for code := range enabled {
		f, ok := tax.Feature(code)
		if !ok {
			return fmt.Errorf("feature usage: enabled feature %q is not in the taxonomy", code)
		}
		if f.EventName != f.Code {
			return fmt.Errorf("feature usage: enabled feature %q has event_name %q; expected it to equal the code", code, f.EventName)
		}
		if !f.Platforms[PlatformAPI] {
			return fmt.Errorf("feature usage: enabled feature %q is not tracked from the API in the taxonomy", code)
		}
	}
	return nil
}

// Record implements Reporter.
func (p *Publisher) Record(o Occurrence) {
	event, err := validate(p.taxonomy, p.enabled, o, p.now())
	if err != nil {
		p.dropped(o.Feature, 0, err)
		return
	}
	if p.environment != "" {
		event.metadata["environment"] = p.environment
	}

	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		p.dropped(o.Feature, 0, errors.New("reporter is shutting down"))
		return
	}
	select {
	case p.queue <- event:
	default:
		p.dropped(o.Feature, 0, errors.New("delivery queue is full"))
	}
}

// Close implements Reporter. Queued events keep being delivered until ctx is
// done; then in-flight deliveries are cancelled, what is still queued is
// skipped, and Close returns at once. It does not wait for a worker held up
// in an identity lookup (those take no context), so shutdown always ends by
// its deadline; such a worker exits as soon as the lookup returns.
func (p *Publisher) Close(ctx context.Context) error {
	p.mu.Lock()
	if !p.closed {
		p.closed = true
		close(p.queue)
	}
	p.mu.Unlock()

	done := make(chan struct{})
	go func() {
		p.workers.Wait()
		close(done)
	}()
	select {
	case <-done:
		p.cancelBase()
		return nil
	case <-ctx.Done():
		p.cancelBase()
		if n := len(p.queue); n > 0 {
			p.logger.Warn("REPORT_DROPPED",
				slog.String("party", logParty),
				slog.Int("count", n),
				slog.String("error", "shutdown deadline reached before delivery"))
		}
		return fmt.Errorf("feature usage: shutdown cut delivery short: %w", ctx.Err())
	}
}

func (p *Publisher) work() {
	defer p.workers.Done()
	for event := range p.queue {
		if p.baseCtx.Err() != nil {
			continue // shutdown cut delivery short; Close has logged what was left
		}
		p.deliver(event)
	}
}

func (p *Publisher) deliver(event *validatedEvent) {
	ctx, cancel := context.WithTimeout(p.baseCtx, p.overall)
	defer cancel()
	started := time.Now()

	payload, err := p.render(event)
	if err != nil {
		p.dropped(event.feature.Code, 0, err)
		return
	}
	if ctx.Err() != nil {
		// The identity lookup outlasted the delivery deadline or shutdown.
		p.dropped(event.feature.Code, 0, errors.New("delivery deadline reached during the identity lookup"))
		return
	}
	attempts, err := p.client.send(ctx, payload)
	if err != nil {
		p.dropped(event.feature.Code, attempts, err)
		return
	}
	p.logger.Debug("Reported feature usage",
		slog.String("party", logParty),
		slog.String("featureCode", event.feature.Code),
		slog.Int("attempts", attempts),
		slog.Int64("elapsedMs", time.Since(started).Milliseconds()))
}

// render maps the event's identifiers to Moesif's keys and builds the body.
// company_id is required, so an organization that does not map to a UUID
// drops the event; a user whose subject is not a UUID is simply left out — an
// unattributed event is acceptable, an identifying one is not.
func (p *Publisher) render(event *validatedEvent) ([]byte, error) {
	companyID, err := p.attributor.CompanyID(event.orgUUID)
	if err != nil {
		return nil, errors.New("could not resolve the organization")
	}
	if !canonicalUUID(companyID) {
		return nil, errors.New("the organization does not map to a UUID")
	}

	userID := ""
	if canonicalUUID(event.actorSubject) {
		userID = event.actorSubject
	}

	inv := event.invocation
	payload, err := json.Marshal(eventBody{
		Request: eventRequest{
			Time:    inv.RequestTime.Format(time.RFC3339Nano),
			URI:     inv.URI,
			Verb:    inv.Method,
			Headers: map[string]string{},
		},
		Response: eventResponse{
			Time:    inv.ResponseTime.Format(time.RFC3339Nano),
			Status:  inv.Status,
			Headers: map[string]string{},
		},
		UserID:        userID,
		CompanyID:     companyID,
		TransactionID: event.transactionID,
		Metadata:      event.metadata,
	})
	if err != nil {
		return nil, errors.New("could not encode the event")
	}
	return payload, nil
}

// dropped logs a lost event by feature code and reason only — never the
// occurrence's identifiers or dimension values, and never a response body.
func (p *Publisher) dropped(feature string, attempts int, err error) {
	p.logger.Warn("REPORT_DROPPED",
		slog.String("party", logParty),
		slog.String("featureCode", feature),
		slog.Int("attempts", attempts),
		slog.String("error", err.Error()))
}

// NewFromConfig returns the Reporter cfg describes: Noop when reporting is
// turned off or no collector is configured, otherwise a Publisher whose token endpoint and collector calls go
// through the process's shared, SSRF-guarded outbound client policy, with
// redirects refused (both requests carry credentials). It must run after the
// shared client is initialized.
func NewFromConfig(cfg config.FeatureUsage, attributor Attributor, logger *slog.Logger) (Reporter, error) {
	if !cfg.Enabled {
		logger.Info("Feature usage reporting is turned off (feature_usage.enabled = false)", slog.String("party", logParty))
		return Noop(), nil
	}
	if !cfg.Configured() {
		logger.Info("Feature usage reporting is on, but no collector is configured; nothing will be reported",
			slog.String("party", logParty))
		return Noop(), nil
	}
	httpClient, err := utils.NewUpstreamClientWithTimeout(cfg.RequestTimeout)
	if err != nil {
		return nil, fmt.Errorf("feature usage: outbound HTTP client: %w", err)
	}
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	publisher, err := NewPublisher(Options{
		CollectorURL:   cfg.CollectorURL,
		HTTPClient:     httpClient,
		Tokens:         newClientCredentialsTokenSource(httpClient, strings.TrimSpace(cfg.Token.TokenURL), cfg.Token.ClientID, cfg.Token.ClientSecret),
		Attributor:     attributor,
		Logger:         logger,
		QueueSize:      cfg.QueueSize,
		Workers:        cfg.Workers,
		MaxAttempts:    cfg.MaxAttempts,
		RetryBackoff:   cfg.RetryBackoff,
		AttemptTimeout: cfg.RequestTimeout,
		OverallTimeout: cfg.OverallTimeout,
		Environment:    cfg.Environment,
	})
	if err != nil {
		return nil, err
	}
	logger.Info("Feature usage reporting enabled",
		slog.String("party", logParty),
		slog.Int("features", len(enabledFeatures)),
		slog.Int("operations", len(bindings)),
		slog.String("environment", cfg.Environment),
		slog.Bool("plaintextCollector", strings.HasPrefix(strings.ToLower(strings.TrimSpace(cfg.CollectorURL)), "http://")))
	return publisher, nil
}
