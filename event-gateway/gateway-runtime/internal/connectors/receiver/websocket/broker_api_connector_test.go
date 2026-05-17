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

package websocket

import (
	"context"
	"testing"
	"time"

	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/connectors"
)

func TestInboundLoopReturnsWhenInboundChannelClosed(t *testing.T) {
	conn := &brokerApiConnection{
		connID:      "test-connection",
		inbound:     make(chan *connectors.Message),
		channelName: "orders",
	}
	close(conn.inbound)

	assertLoopReturnsWithoutPanic(t, func() {
		(&WebBrokerApiReceiver{}).inboundLoop(context.Background(), conn)
	})
}

func TestOutboundLoopReturnsWhenOutboundChannelClosed(t *testing.T) {
	conn := &brokerApiConnection{
		connID:      "test-connection",
		outbound:    make(chan *connectors.Message),
		channelName: "orders",
	}
	close(conn.outbound)

	assertLoopReturnsWithoutPanic(t, func() {
		(&WebBrokerApiReceiver{}).outboundLoop(context.Background(), conn)
	})
}

func assertLoopReturnsWithoutPanic(t *testing.T, run func()) {
	t.Helper()

	done := make(chan any, 1)
	go func() {
		defer func() {
			done <- recover()
		}()
		run()
	}()

	select {
	case recovered := <-done:
		if recovered != nil {
			t.Fatalf("loop panicked after channel close: %v", recovered)
		}
	case <-time.After(time.Second):
		t.Fatal("loop did not return after channel close")
	}
}
