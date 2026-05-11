package websub

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/subscription"
)

type deltaTestBrokerDriver struct {
	publishErrs map[string]error
	published   []string
}

func (d *deltaTestBrokerDriver) Publish(_ context.Context, topic string, msg *connectors.Message) error {
	key := string(msg.Key)
	d.published = append(d.published, key)
	if err, ok := d.publishErrs[key]; ok {
		return err
	}
	return nil
}

func (d *deltaTestBrokerDriver) Subscribe(string, []string, connectors.MessageHandler) (connectors.Receiver, error) {
	return nil, nil
}

func (d *deltaTestBrokerDriver) SubscribeManual(string, []string, connectors.MessageHandler) (connectors.Receiver, error) {
	return nil, nil
}

func (d *deltaTestBrokerDriver) Replay(context.Context, string, connectors.MessageHandler) error {
	return nil
}

func (d *deltaTestBrokerDriver) TopicExists(context.Context, string) (bool, error) {
	return false, nil
}

func (d *deltaTestBrokerDriver) EnsureTopics(context.Context, []string) error {
	return nil
}

func (d *deltaTestBrokerDriver) EnsureCompactedTopic(context.Context, string) error {
	return nil
}

func (d *deltaTestBrokerDriver) DeleteTopics(context.Context, []string) error {
	return nil
}

func (d *deltaTestBrokerDriver) Close() error {
	return nil
}

type deltaTestReceiver struct {
	stopCalls int
}

func (r *deltaTestReceiver) Start(context.Context) error {
	return nil
}

func (r *deltaTestReceiver) Stop(context.Context) error {
	r.stopCalls++
	return nil
}

func TestApplyBindingDeltaContinuesAfterTombstoneFailure(t *testing.T) {
	store := subscription.NewInMemoryStore("runtime-1")
	require.NoError(t, store.Add(&subscription.Subscription{
		ID:          "sub-1",
		Topic:       "removed-a",
		CallbackURL: "https://callback-a.example",
		State:       subscription.StateActive,
	}))
	require.NoError(t, store.Add(&subscription.Subscription{
		ID:          "sub-2",
		Topic:       "removed-b",
		CallbackURL: "https://callback-b.example",
		State:       subscription.StateActive,
	}))

	topics := NewTopicRegistry()
	topics.Register("removed-a")
	topics.Register("removed-b")

	consumerA := &deltaTestReceiver{}
	consumerB := &deltaTestReceiver{}
	consumerMgr := &ConsumerManager{
		consumers: map[string]*managedConsumer{
			"https://callback-a.example": {
				consumer: consumerA,
				topics:   map[string]bool{"kafka-a": true},
			},
			"https://callback-b.example": {
				consumer: consumerB,
				topics:   map[string]bool{"kafka-b": true},
			},
		},
	}

	driver := &deltaTestBrokerDriver{
		publishErrs: map[string]error{
			"removed-a:https://callback-a.example": errors.New("boom"),
		},
	}

	receiver := &WebSubReceiver{
		topics:      topics,
		store:       store,
		consumerMgr: consumerMgr,
		syncProducer: subscription.NewSyncProducer(
			driver,
			"runtime-1",
			"internal-sub-topic",
		),
		channel: connectors.ChannelInfo{
			Name: "test-api",
			Channels: map[string]string{
				"removed-a": "kafka-a",
				"removed-b": "kafka-b",
			},
		},
	}

	err := receiver.ApplyBindingDelta(context.Background(),
		map[string]string{
			"removed-a": "kafka-a",
			"removed-b": "kafka-b",
		},
		map[string]string{
			"added-c": "kafka-c",
		},
	)

	require.Error(t, err)
	require.ErrorContains(t, err, `failed to tombstone subscription for removed channel "removed-a"`)

	require.ElementsMatch(t, []string{
		"removed-a:https://callback-a.example",
		"removed-b:https://callback-b.example",
	}, driver.published)

	require.Nil(t, store.GetByTopic("removed-a"))
	require.Nil(t, store.GetByTopic("removed-b"))

	require.Equal(t, 1, consumerA.stopCalls)
	require.Equal(t, 1, consumerB.stopCalls)
	require.Empty(t, consumerMgr.consumers)

	require.NotContains(t, receiver.channel.Channels, "removed-a")
	require.NotContains(t, receiver.channel.Channels, "removed-b")
	require.Equal(t, "kafka-c", receiver.channel.Channels["added-c"])

	require.False(t, topics.IsRegistered("removed-a"))
	require.False(t, topics.IsRegistered("removed-b"))
	require.True(t, topics.IsRegistered("added-c"))
}
