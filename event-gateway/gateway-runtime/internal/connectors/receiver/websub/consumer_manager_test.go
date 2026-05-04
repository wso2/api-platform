package websub

import (
	"context"
	"slices"
	"testing"

	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors"
)

func TestConsumerManagerUsesBrokerDriverSubscribe(t *testing.T) {
	broker := &recordingBrokerDriver{}
	manager := NewConsumerManager(
		broker,
		"event-gateway",
		noopProcessor{},
		"binding-1",
		NewDeliverer(DeliveryConfig{}),
	)
	manager.SetContext(context.Background())

	if err := manager.AddSubscription("http://callback", "secret", "topic-a"); err != nil {
		t.Fatalf("AddSubscription returned error: %v", err)
	}
	if err := manager.AddSubscription("http://callback", "secret", "topic-b"); err != nil {
		t.Fatalf("AddSubscription second call returned error: %v", err)
	}

	if len(broker.subscribeCalls) != 2 {
		t.Fatalf("expected 2 subscribe calls, got %d", len(broker.subscribeCalls))
	}
	if broker.subscribeCalls[0].groupID != manager.consumerGroupID("http://callback") {
		t.Fatalf("unexpected group id: %q", broker.subscribeCalls[0].groupID)
	}
	gotTopics := append([]string(nil), broker.subscribeCalls[1].topics...)
	slices.Sort(gotTopics)
	wantTopics := []string{"topic-a", "topic-b"}
	if !slices.Equal(gotTopics, wantTopics) {
		t.Fatalf("expected topics %v, got %v", wantTopics, gotTopics)
	}
	if broker.stopCount != 1 {
		t.Fatalf("expected previous consumer to be stopped once, got %d", broker.stopCount)
	}
}

type recordingBrokerDriver struct {
	subscribeCalls []subscribeCall
	stopCount      int
}

type subscribeCall struct {
	groupID string
	topics  []string
}

func (r *recordingBrokerDriver) Publish(context.Context, string, *connectors.Message) error {
	return nil
}

func (r *recordingBrokerDriver) Subscribe(groupID string, topics []string, _ connectors.MessageHandler) (connectors.Receiver, error) {
	r.subscribeCalls = append(r.subscribeCalls, subscribeCall{
		groupID: groupID,
		topics:  append([]string(nil), topics...),
	})
	return &recordingReceiver{broker: r}, nil
}

func (r *recordingBrokerDriver) Replay(context.Context, []string, connectors.MessageHandler) error {
	return nil
}

func (r *recordingBrokerDriver) TopicExists(context.Context, string) (bool, error) {
	return true, nil
}

func (r *recordingBrokerDriver) EnsureTopics(context.Context, []string) error {
	return nil
}

func (r *recordingBrokerDriver) EnsureStateTopics(context.Context, []string) error {
	return nil
}

func (r *recordingBrokerDriver) DeleteTopics(context.Context, []string) error {
	return nil
}

func (r *recordingBrokerDriver) Close() error {
	return nil
}

type recordingReceiver struct {
	broker *recordingBrokerDriver
}

func (r *recordingReceiver) Start(context.Context) error { return nil }

func (r *recordingReceiver) Stop(context.Context) error {
	r.broker.stopCount++
	return nil
}

type noopProcessor struct{}

func (noopProcessor) ProcessSubscribe(context.Context, string, *connectors.Message) (*connectors.Message, bool, error) {
	return nil, false, nil
}

func (noopProcessor) ProcessInbound(context.Context, string, *connectors.Message) (*connectors.Message, bool, error) {
	return nil, false, nil
}

func (noopProcessor) ProcessOutbound(_ context.Context, _ string, msg *connectors.Message) (*connectors.Message, bool, error) {
	return msg, false, nil
}
