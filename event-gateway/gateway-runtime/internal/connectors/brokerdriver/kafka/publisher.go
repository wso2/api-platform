/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package kafka

import (
	"context"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors"
)

// Publisher publishes events to Kafka topics.
type Publisher struct {
	client *kgo.Client
}

// NewPublisher creates a new Kafka publisher.
func NewPublisher(brokers []string, opts ...kgo.Opt) (*Publisher, error) {
	allOpts := append([]kgo.Opt{kgo.SeedBrokers(brokers...)}, opts...)
	client, err := kgo.NewClient(allOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka publisher: %w", err)
	}
	return &Publisher{client: client}, nil
}

// Publish sends a message to the specified Kafka topic.
func (p *Publisher) Publish(ctx context.Context, topic string, msg *connectors.Message) error {
	record := &kgo.Record{
		Topic: topic,
		Key:   msg.Key,
		Value: msg.Value,
	}

	// Map message headers to Kafka headers
	for k, vs := range msg.Headers {
		for _, v := range vs {
			record.Headers = append(record.Headers, kgo.RecordHeader{
				Key:   k,
				Value: []byte(v),
			})
		}
	}

	results := p.client.ProduceSync(ctx, record)
	if err := results.FirstErr(); err != nil {
		return fmt.Errorf("failed to publish to topic %s: %w", topic, err)
	}

	return nil
}

// Close closes the publisher.
func (p *Publisher) Close() {
	p.client.Close()
}
