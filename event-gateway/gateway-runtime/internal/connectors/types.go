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

package connectors

import "context"

// Message represents an event message flowing through the event gateway.
type Message struct {
	Key      []byte
	Value    []byte
	Headers  map[string][]string
	Topic    string
	Metadata map[string]interface{}
}

// MessageHandler is a callback invoked when a message is received.
type MessageHandler func(ctx context.Context, msg *Message) error

// EntrypointConnector accepts external client connections (WebSub HTTP, WebSocket).
type EntrypointConnector interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// EndpointConnector connects to backend event systems (e.g. Kafka).
type EndpointConnector interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Publish(ctx context.Context, topic string, msg *Message) error
}
