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

package counters

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/encoding"

	"github.com/wso2/api-platform/gateway/common/counterproto"
)

func init() {
	encoding.RegisterCodec(counterproto.Codec{})
}

// Syncer keeps a Service in step with the controller: one request and its response
// every Interval on a long-lived stream, reconnecting with backoff when it breaks.
type Syncer struct {
	Service       *Service
	ServerAddress string
	Credentials   credentials.TransportCredentials
	Interval      time.Duration
	// Timeout bounds one request and its response.
	Timeout    time.Duration
	MaxBackoff time.Duration
	Logger     *slog.Logger
	// dialer replaces the network connection in tests.
	dialer func(context.Context, string) (net.Conn, error)
}

// Run syncs until ctx is done.
func (s *Syncer) Run(ctx context.Context) {
	backoff := s.Interval
	for ctx.Err() == nil {
		err := s.stream(ctx)
		if ctx.Err() != nil {
			return
		}
		s.Logger.Warn("Counter sync with the gateway-controller stopped; retrying",
			slog.Any("error", err), slog.Duration("retry_in", backoff))
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > s.MaxBackoff {
			backoff = s.MaxBackoff
		}
	}
}

// stream opens one stream and syncs on it until it fails.
func (s *Syncer) stream(ctx context.Context) error {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(s.Credentials),
		grpc.WithDefaultCallOptions(grpc.CallContentSubtype(counterproto.CodecName)),
	}
	if s.dialer != nil {
		opts = append(opts, grpc.WithContextDialer(s.dialer))
	}
	conn, err := grpc.NewClient(s.ServerAddress, opts...)
	if err != nil {
		return err
	}
	defer conn.Close()
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := conn.NewStream(streamCtx,
		&grpc.StreamDesc{StreamName: counterproto.SyncMethod, ServerStreams: true, ClientStreams: true},
		counterproto.FullSyncMethod)
	if err != nil {
		return err
	}
	ticker := time.NewTicker(s.Interval)
	defer ticker.Stop()
	for {
		if err := s.syncOnce(stream); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// syncOnce sends one request and applies its response.
func (s *Syncer) syncOnce(stream grpc.ClientStream) error {
	req := s.Service.nextRequest()
	if err := stream.SendMsg(req); err != nil {
		return fmt.Errorf("sending counters: %w", err)
	}
	resp := &counterproto.SyncResponse{}
	done := make(chan error, 1)
	go func() { done <- stream.RecvMsg(resp) }()
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("receiving counter totals: %w", err)
		}
	case <-time.After(s.Timeout):
		return fmt.Errorf("no counter totals within %s", s.Timeout)
	}
	s.Service.apply(req, resp)
	return nil
}
