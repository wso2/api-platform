package eventhub

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// poller handles background polling for state changes and event delivery
type poller struct {
	store    *store
	registry *organizationRegistry
	config   Config
	logger   *zap.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// newPoller creates a new event poller
func newPoller(store *store, registry *organizationRegistry, config Config, logger *zap.Logger) *poller {
	return &poller{
		store:    store,
		registry: registry,
		config:   config,
		logger:   logger,
	}
}

// start begins the poller background worker
func (p *poller) start(ctx context.Context) {
	p.ctx, p.cancel = context.WithCancel(ctx)

	p.wg.Add(1)
	go p.pollLoop()

	p.logger.Info("Poller started", zap.Duration("interval", p.config.PollInterval))
}

// pollLoop runs the main polling loop
func (p *poller) pollLoop() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.pollAllOrganizations()
		}
	}
}

// pollAllOrganizations checks all organizations for state changes using single query
func (p *poller) pollAllOrganizations() {
	ctx := p.ctx

	// STEP 1: Single query for ALL organization states
	states, err := p.store.getAllStates(ctx)
	if err != nil {
		p.logger.Error("Failed to fetch all states", zap.Error(err))
		return
	}

	// STEP 2: Loop through each organization sequentially
	for _, state := range states {
		orgID := OrganizationID(state.Organization)

		// Get the organization from registry
		org, err := p.registry.get(orgID)
		if err != nil {
			// Organization not registered with subscribers, skip
			continue
		}

		// Check if version changed
		if state.VersionID == org.knownVersion {
			// No changes
			continue
		}

		p.logger.Debug("State change detected",
			zap.String("organization", string(orgID)),
			zap.String("oldVersion", org.knownVersion),
			zap.String("newVersion", state.VersionID),
		)

		// Fetch events since last poll
		events, err := p.store.getEventsSince(ctx, orgID, org.lastPolled)
		if err != nil {
			p.logger.Error("Failed to fetch events",
				zap.String("organization", string(orgID)),
				zap.Error(err))
			continue
		}

		if len(events) > 0 {
			// Deliver events to subscribers
			p.deliverEvents(org, events)
		}

		// Update poll state
		org.updatePollState(state.VersionID, time.Now())
	}
}

// deliverEvents sends events to all subscribers of an organization
func (p *poller) deliverEvents(org *organization, events []Event) {
	subscribers := org.getSubscribers()

	if len(subscribers) == 0 {
		p.logger.Debug("No subscribers for organization",
			zap.String("organization", string(org.id)),
			zap.Int("events", len(events)),
		)
		return
	}

	// Deliver ALL events (all event types) to subscribers
	// Consumers are responsible for filtering by EventType if needed
	for _, ch := range subscribers {
		select {
		case ch <- events:
			p.logger.Debug("Delivered events to subscriber",
				zap.String("organization", string(org.id)),
				zap.Int("events", len(events)),
			)
		default:
			p.logger.Warn("Subscriber channel full, dropping events",
				zap.String("organization", string(org.id)),
				zap.Int("events", len(events)),
			)
		}
	}
}

// stop gracefully stops the poller
func (p *poller) stop() {
	if p.cancel != nil {
		p.cancel()
	}
	p.wg.Wait()
	p.logger.Info("Poller stopped")
}
