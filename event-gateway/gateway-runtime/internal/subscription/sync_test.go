package subscription

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors"
)

func TestSyncProducerUsesBrokerDriver(t *testing.T) {
	broker := &recordingBrokerDriver{}
	producer := NewSyncProducer(broker, "runtime-1", "sync-topic")

	if err := producer.EnsureSyncTopic(context.Background()); err != nil {
		t.Fatalf("EnsureSyncTopic returned error: %v", err)
	}

	sub := &Subscription{
		Topic:       "/issues/ref",
		CallbackURL: "http://callback",
		State:       StateActive,
	}
	if err := producer.PublishSubscription(context.Background(), sub); err != nil {
		t.Fatalf("PublishSubscription returned error: %v", err)
	}
	if err := producer.PublishTombstone(context.Background(), "/issues/ref", "http://callback"); err != nil {
		t.Fatalf("PublishTombstone returned error: %v", err)
	}

	if !reflect.DeepEqual(broker.stateTopics, []string{"sync-topic"}) {
		t.Fatalf("expected state topic ensure call, got %v", broker.stateTopics)
	}
	if len(broker.published) != 2 {
		t.Fatalf("expected 2 published messages, got %d", len(broker.published))
	}
	if broker.published[0].topic != "sync-topic" {
		t.Fatalf("expected sync topic publish, got %q", broker.published[0].topic)
	}

	var publishedSub Subscription
	if err := json.Unmarshal(broker.published[0].msg.Value, &publishedSub); err != nil {
		t.Fatalf("failed to unmarshal published subscription: %v", err)
	}
	if publishedSub.RuntimeID != "runtime-1" {
		t.Fatalf("expected runtime id runtime-1, got %q", publishedSub.RuntimeID)
	}
	if broker.published[1].msg.Value != nil {
		t.Fatalf("expected tombstone publish with nil value")
	}
}

func TestReconcilerUsesReplay(t *testing.T) {
	store := NewInMemoryStore("runtime-1")

	sub1 := Subscription{
		ID:          "sub-1",
		Topic:       "/issues/ref",
		CallbackURL: "http://callback-1",
		State:       StateActive,
		RuntimeID:   "runtime-2",
	}
	sub1Value, err := json.Marshal(sub1)
	if err != nil {
		t.Fatalf("marshal sub1: %v", err)
	}

	sub2 := Subscription{
		ID:          "sub-2",
		Topic:       "/issues/ref",
		CallbackURL: "http://callback-2",
		State:       StateInactive,
		RuntimeID:   "runtime-2",
	}
	sub2Value, err := json.Marshal(sub2)
	if err != nil {
		t.Fatalf("marshal sub2: %v", err)
	}

	broker := &recordingBrokerDriver{
		replayMessages: []*connectors.Message{
			{Key: []byte(syncKey(sub1.Topic, sub1.CallbackURL)), Value: sub1Value},
			{Key: []byte(syncKey(sub2.Topic, sub2.CallbackURL)), Value: sub2Value},
			{Key: []byte(syncKey(sub2.Topic, sub2.CallbackURL)), Value: nil},
		},
	}

	reconciler := NewReconciler(broker, store, "runtime-1", "sync-topic")
	var callbacks []string
	reconciler.SetCallback(func(sub *Subscription, isTombstone bool) {
		if isTombstone {
			callbacks = append(callbacks, "tombstone:"+sub.CallbackURL)
			return
		}
		callbacks = append(callbacks, string(sub.State)+":"+sub.CallbackURL)
	})

	if err := reconciler.Reconcile(context.Background()); err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}

	if broker.replayTopic != "sync-topic" {
		t.Fatalf("expected replay topic sync-topic, got %q", broker.replayTopic)
	}
	active := store.GetActive()
	if len(active) != 1 || active[0].CallbackURL != "http://callback-1" {
		t.Fatalf("unexpected active subscriptions after reconcile: %+v", active)
	}

	wantCallbacks := []string{"active:http://callback-1", "inactive:http://callback-2", "tombstone:http://callback-2"}
	if !reflect.DeepEqual(callbacks, wantCallbacks) {
		t.Fatalf("expected callbacks %v, got %v", wantCallbacks, callbacks)
	}
}

type recordingBrokerDriver struct {
	published      []publishedMessage
	stateTopics    []string
	replayMessages []*connectors.Message
	replayTopic    string
}

type publishedMessage struct {
	topic string
	msg   *connectors.Message
}

func (r *recordingBrokerDriver) Publish(_ context.Context, topic string, msg *connectors.Message) error {
	cloned := &connectors.Message{
		Key:   append([]byte(nil), msg.Key...),
		Value: append([]byte(nil), msg.Value...),
	}
	r.published = append(r.published, publishedMessage{topic: topic, msg: cloned})
	return nil
}

func (r *recordingBrokerDriver) Subscribe(string, []string, connectors.MessageHandler) (connectors.Receiver, error) {
	return &noopReceiver{}, nil
}

func (r *recordingBrokerDriver) Replay(ctx context.Context, topics []string, handler connectors.MessageHandler) error {
	if len(topics) != 1 {
		return nil
	}
	r.replayTopic = topics[0]
	for _, msg := range r.replayMessages {
		if err := handler(ctx, msg); err != nil {
			return err
		}
	}
	return nil
}

func (r *recordingBrokerDriver) TopicExists(context.Context, string) (bool, error) {
	return true, nil
}

func (r *recordingBrokerDriver) EnsureTopics(context.Context, []string) error {
	return nil
}

func (r *recordingBrokerDriver) EnsureStateTopics(_ context.Context, topics []string) error {
	r.stateTopics = append(r.stateTopics, topics...)
	return nil
}

func (r *recordingBrokerDriver) DeleteTopics(context.Context, []string) error {
	return nil
}

func (r *recordingBrokerDriver) Close() error {
	return nil
}

type noopReceiver struct{}

func (n *noopReceiver) Start(context.Context) error { return nil }
func (n *noopReceiver) Stop(context.Context) error  { return nil }
