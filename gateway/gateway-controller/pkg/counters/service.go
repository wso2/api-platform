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
	"errors"
	"io"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"

	"github.com/wso2/api-platform/gateway/common/counterproto"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/tlsauth"
)

func init() {
	// The messages are JSON on the wire (see counterproto); the codec is chosen by the
	// content subtype the engine sends.
	encoding.RegisterCodec(counterproto.Codec{})
}

// service serves the counter stream.
type service struct {
	store   Store
	allowed map[string]bool // nil: no mTLS identity check
	logger  *slog.Logger
}

// Register adds the counter stream to the policy server. allowed restricts the peers
// as the policy xDS stream does; nil when mTLS is not configured.
func Register(server *grpc.Server, store Store, allowed map[string]bool, logger *slog.Logger) {
	s := &service{store: store, allowed: allowed, logger: logger}
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: counterproto.ServiceName,
		HandlerType: (*any)(nil),
		Streams: []grpc.StreamDesc{{
			StreamName:    counterproto.SyncMethod,
			Handler:       s.sync,
			ServerStreams: true,
			ClientStreams: true,
		}},
	}, s)
}

// sync answers each SyncRequest with the totals of its keys, after applying its adds.
// When the store fails, the response carries no ReportID, so the engine keeps the adds
// and sends them again.
func (s *service) sync(_ any, stream grpc.ServerStream) error {
	ctx := stream.Context()
	if s.allowed != nil {
		if err := tlsauth.VerifyStreamPeer(ctx, s.allowed); err != nil {
			return err
		}
	}
	for {
		req := &counterproto.SyncRequest{}
		if err := stream.RecvMsg(req); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		resp := &counterproto.SyncResponse{}
		totals, err := s.store.Sync(ctx, req.Source, req.ReportID, req.Adds, req.Keys)
		if err != nil {
			s.logger.Error("Failed to sync counters", slog.String("source", req.Source),
				slog.String("report_id", req.ReportID), slog.Any("error", err))
		} else {
			resp.ReportID = req.ReportID
			resp.Totals = totals
		}
		if err := stream.SendMsg(resp); err != nil {
			return err
		}
	}
}
