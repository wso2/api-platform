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
	registry *topicRegistry
	config   Config
	logger   *zap.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// newPoller creates a new event poller
func newPoller(store *store, registry *topicRegistry, config Config, logger *zap.Logger) *poller {
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
			p.pollAllTopics()
		}
	}
}

// pollAllTopics checks all registered topics for state changes
func (p *poller) pollAllTopics() {
	topics := p.registry.getAll()

	for _, t := range topics {
		if err := p.pollTopic(t); err != nil {
			p.logger.Error("Failed to poll topic",
				zap.String("topic", string(t.name)),
				zap.Error(err),
			)
		}
	}
}

// pollTopic checks a single topic for state changes and delivers events
func (p *poller) pollTopic(t *topic) error {
	ctx := p.ctx

	// Get current state from database
	state, err := p.store.getState(ctx, t.name)
	if err != nil {
		return err
	}
	if state == nil {
		// Topic state not initialized yet
		return nil
	}

	// Check if version has changed
	if state.VersionID == t.knownVersion {
		// No changes
		return nil
	}

	p.logger.Debug("State change detected",
		zap.String("topic", string(t.name)),
		zap.String("oldVersion", t.knownVersion),
		zap.String("newVersion", state.VersionID),
	)

	// Fetch events since last poll
	events, err := p.store.getEventsSince(ctx, t.name, t.lastPolled)
	if err != nil {
		return err
	}

	if len(events) > 0 {
		// Deliver events to subscribers
		p.deliverEvents(t, events)
	}

	// Update poll state
	t.updatePollState(state.VersionID, time.Now())

	return nil
}

// deliverEvents sends events to all subscribers of a topic
func (p *poller) deliverEvents(t *topic, events []Event) {
	subscribers := t.getSubscribers()

	if len(subscribers) == 0 {
		p.logger.Debug("No subscribers for topic",
			zap.String("topic", string(t.name)),
			zap.Int("events", len(events)),
		)
		return
	}

	// Deliver to all subscribers
	for _, ch := range subscribers {
		select {
		case ch <- events:
			p.logger.Debug("Delivered events to subscriber",
				zap.String("topic", string(t.name)),
				zap.Int("events", len(events)),
			)
		default:
			p.logger.Warn("Subscriber channel full, dropping events",
				zap.String("topic", string(t.name)),
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
