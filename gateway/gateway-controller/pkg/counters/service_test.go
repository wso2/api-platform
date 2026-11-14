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

package counters

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/wso2/api-platform/gateway/common/counterproto"
)

// failingStore always fails.
type failingStore struct{}

func (failingStore) Sync(context.Context, string, string, []counterproto.Add, []counterproto.Key) ([]counterproto.Total, error) {
	return nil, errors.New("store down")
}
func (failingStore) Close() error { return nil }

// openStream serves store and opens a client stream to it.
func openStream(t *testing.T, store Store) grpc.ClientStream {
	t.Helper()
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	Register(server, store, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.CallContentSubtype(counterproto.CodecName)))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	stream, err := conn.NewStream(context.Background(),
		&grpc.StreamDesc{StreamName: counterproto.SyncMethod, ServerStreams: true, ClientStreams: true},
		counterproto.FullSyncMethod)
	require.NoError(t, err)
	return stream
}

func TestServiceAcknowledgesReportsAndReturnsTotals(t *testing.T) {
	stream := openStream(t, NewMemory(0))
	key := counterproto.KeyFor("budget", "alice", november)

	for i, want := range []float64{2, 2} { // the second send is the same report again
		require.NoError(t, stream.SendMsg(&counterproto.SyncRequest{
			Source: "engine-1", ReportID: "r1",
			Adds: []counterproto.Add{{Key: key, Delta: 2}}, Keys: []counterproto.Key{key},
		}))
		resp := &counterproto.SyncResponse{}
		require.NoError(t, stream.RecvMsg(resp), "round %d", i)
		assert.Equal(t, "r1", resp.ReportID)
		require.Len(t, resp.Totals, 1)
		assert.Equal(t, want, resp.Totals[0].Total)
	}
}

func TestServiceDoesNotAcknowledgeWhenTheStoreFails(t *testing.T) {
	stream := openStream(t, failingStore{})

	require.NoError(t, stream.SendMsg(&counterproto.SyncRequest{Source: "engine-1", ReportID: "r1",
		Adds: []counterproto.Add{{Key: counterproto.Key{Namespace: "budget", ID: "a"}, Delta: 1}}}))
	resp := &counterproto.SyncResponse{}
	require.NoError(t, stream.RecvMsg(resp))

	assert.Empty(t, resp.ReportID, "the engine must keep the adds and send them again")
	assert.Empty(t, resp.Totals)
}
