/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package utils

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"time"

	v3 "github.com/envoyproxy/go-control-plane/envoy/service/accesslog/v3"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

// AccessLogServiceServer is the gRPC server for the Access Log Service.
type AccessLogServiceServer struct {
	cfg       *config.Config
	analytics *analytics.Analytics
}

// newAccessLogServiceServer creates a new instance of the Access Log Service Server.
func newAccessLogServiceServer(cfg *config.Config) *AccessLogServiceServer {
	analytics := analytics.NewAnalytics(cfg)
	return &AccessLogServiceServer{
		cfg:       cfg,
		analytics: analytics,
	}
}

// StreamAccessLogs streams access logs to the server.
func (s *AccessLogServiceServer) StreamAccessLogs(stream v3.AccessLogService_StreamAccessLogsServer) error {
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		httpLogs := in.GetHttpLogs()
		if httpLogs != nil {
			slog.Debug("Received a stream of access logs", "count", len(httpLogs.LogEntry))
			for _, logEntry := range httpLogs.LogEntry {
				s.analytics.Process(logEntry)
			}
		}
	}
}

// StartAccessLogServiceServer starts the Access Log Service Server.
func StartAccessLogServiceServer(cfg *config.Config) *grpc.Server {
	// Create a new instance of the Access Log Service Server
	accessLogServiceServer := newAccessLogServiceServer(cfg)

	kaParams := keepalive.ServerParameters{
		Time:    2 * time.Hour, // Ping the client if it is idle for 2 hours
		Timeout: 20 * time.Second,
	}
	server, err := CreateGRPCServer(cfg.Analytics.AccessLogsServiceCfg.PublicKeyPath,
		cfg.Analytics.AccessLogsServiceCfg.PrivateKeyPath, cfg.Analytics.AccessLogsServiceCfg.ALSPlainText,
		grpc.MaxRecvMsgSize(cfg.Analytics.AccessLogsServiceCfg.ExtProcMaxMessageSize),
		grpc.MaxHeaderListSize(uint32(cfg.Analytics.AccessLogsServiceCfg.ExtProcMaxHeaderLimit)),
		grpc.KeepaliveParams(kaParams))
	if err != nil {
		panic(err)
	}

	v3.RegisterAccessLogServiceServer(server, accessLogServiceServer)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Analytics.AccessLogsServiceCfg.ALSServerPort))
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to listen on port: %d", cfg.Analytics.AccessLogsServiceCfg.ALSServerPort))
		panic(err)
	}
	go func() {
		slog.Info("Starting to serve access log service server", "port", cfg.Analytics.AccessLogsServiceCfg.ALSServerPort)
		if err := server.Serve(listener); err != nil {
			slog.Error("ALS server exited", "error", err)
		}
	}()

	return server
}
