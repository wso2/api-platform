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
	"math"
	"net"
	"os"
	"time"

	accesslogv3 "github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v3"
	v3 "github.com/envoyproxy/go-control-plane/envoy/service/accesslog/v3"
	"github.com/wso2/api-platform/common/collector"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/correlation"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/attemptverdict"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

// accessLogConsumer is one destination for the entries Envoy streams in.
//
// There is more than one, and they are independent: analytics/traffic logging wants every entry,
// while failover health grading wants only the handful carrying an upstream-failure flag. Fanning
// out through a list rather than calling each by name keeps a consumer that is not configured
// from costing anything per entry, and keeps the stream itself from being owned by any one of them.
type accessLogConsumer interface {
	Name() string
	Consume(entry *accesslogv3.HTTPAccessLogEntry)
}

// analyticsConsumer adapts Analytics to the accessLogConsumer interface.
type analyticsConsumer struct{ a *analytics.Analytics }

func (c analyticsConsumer) Name() string { return "analytics" }

func (c analyticsConsumer) Consume(entry *accesslogv3.HTTPAccessLogEntry) { c.a.Process(entry) }

// AccessLogServiceServer is the gRPC server for the Access Log Service.
type AccessLogServiceServer struct {
	cfg       *config.Config
	analytics *analytics.Analytics
	consumers []accessLogConsumer

	// verdicts is retained separately from consumers so the caller can reach its counters and
	// swap its sink; it is also present in consumers.
	verdicts *attemptverdict.Consumer
}

func checkedUInt32FromPositiveInt(fieldName string, value int) (uint32, error) {
	if value <= 0 {
		return 0, fmt.Errorf("%s must be positive, got %d", fieldName, value)
	}
	if value > math.MaxUint32 {
		return 0, fmt.Errorf("%s exceeds uint32 range: %d", fieldName, value)
	}
	return uint32(value), nil
}

// newAccessLogServiceServer creates a new instance of the Access Log Service Server.
// corrStore may be nil (collector disabled elsewhere) -- Analytics treats a nil
// store as "always miss", falling back to decoding headers from the access-log
// entry's own metadata exactly as it did before this store existed.
func newAccessLogServiceServer(cfg *config.Config, corrStore *correlation.Store) *AccessLogServiceServer {
	analyticsInstance := analytics.NewAnalytics(cfg)
	if corrStore != nil {
		analyticsInstance.SetCorrelationStore(corrStore)
	}

	verdicts := attemptverdict.NewConsumer(nil)
	consumers := []accessLogConsumer{verdicts}

	// Analytics is registered only when the collector is enabled. Process() builds a full
	// analytic event before it discovers there are no publishers to take it, and logs an error
	// for any entry that does not fit the shape it expects — so running it unconditionally
	// would spend real work per entry, and produce misleading errors, in a deployment whose
	// stream exists purely for failover grading.
	if cfg.IsCollectorEnabled() {
		consumers = append(consumers, analyticsConsumer{a: analyticsInstance})
	}

	names := make([]string, 0, len(consumers))
	for _, c := range consumers {
		names = append(names, c.Name())
	}
	slog.Info("Access log service consumers registered", "consumers", names)

	return &AccessLogServiceServer{
		cfg:       cfg,
		analytics: analyticsInstance,
		consumers: consumers,
		verdicts:  verdicts,
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
				for _, consumer := range s.consumers {
					consumer.Consume(logEntry)
				}
			}
		}
	}
}

// AccessLogService is a started ALS receiver and the handles its caller needs for shutdown.
type AccessLogService struct {
	Server    *grpc.Server
	Analytics *analytics.Analytics
	Verdicts  *attemptverdict.Consumer
}

// StartAccessLogServiceServer starts the Access Log Service Server.
//
// The caller owns shutdown ordering: stop the gRPC server first so no new events arrive, then
// call Analytics.Close to flush publishers that buffer (the traffic-log HTTP sink, Moesif).
// Without that flush, an in-flight batch is lost on every pod restart.
//
// Errors are returned rather than fatal because the two callers want different things from them.
// A deployment that enabled analytics or traffic logging and cannot stand up their only transport
// should fail loudly at boot. A deployment that is starting this receiver solely so failover can
// grade mid-stream failures should not: failover works without it, merely more conservatively
// (an attempt it cannot verify records nothing rather than a success — FR-030p), so an unusable
// collector transport there is a degraded capability, not a reason to refuse to serve traffic.
func StartAccessLogServiceServer(cfg *config.Config, corrStore *correlation.Store) (*AccessLogService, error) {
	// Create a new instance of the Access Log Service Server
	accessLogServiceServer := newAccessLogServiceServer(cfg, corrStore)

	kaParams := keepalive.ServerParameters{
		Time:    2 * time.Hour, // Ping the client if it is idle for 2 hours
		Timeout: 20 * time.Second,
	}
	maxHeaderListSize, err := checkedUInt32FromPositiveInt(
		"collector.server.max_header_limit",
		cfg.Collector.Server.ExtProcMaxHeaderLimit,
	)
	if err != nil {
		return nil, err
	}
	server, err := CreateGRPCServer(cfg.Collector.Server.PublicKeyPath,
		cfg.Collector.Server.PrivateKeyPath, cfg.Collector.Server.ALSPlainText,
		grpc.MaxRecvMsgSize(cfg.Collector.Server.ExtProcMaxMessageSize),
		grpc.MaxHeaderListSize(maxHeaderListSize),
		grpc.KeepaliveParams(kaParams))
	if err != nil {
		return nil, err
	}

	v3.RegisterAccessLogServiceServer(server, accessLogServiceServer)

	// Create listener based on mode (same pattern as ext_proc in main.go)
	var listener net.Listener
	alsMode := cfg.Collector.Server.Mode
	if alsMode == "" {
		alsMode = "uds"
	}

	switch alsMode {
	case "uds":
		socketPath := constants.DefaultALSSocketPath
		if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
			slog.Warn("Failed to remove existing ALS socket", "path", socketPath, "error", err)
		}

		listener, err = net.Listen("unix", socketPath)
		if err != nil {
			return nil, fmt.Errorf("listening on ALS Unix socket %s: %w", socketPath, err)
		}

		// Set socket permissions (readable/writable by owner and group)
		if err := os.Chmod(socketPath, 0660); err != nil {
			slog.Warn("Failed to set ALS socket permissions", "path", socketPath, "error", err)
		}

		go func() {
			slog.Info("Starting to serve access log service server", "mode", "uds", "path", socketPath)
			if err := server.Serve(listener); err != nil {
				slog.Error("ALS server exited", "error", err)
			}
		}()
	case "tcp":
		// cfg.Collector.Server.ServerPort is a deprecated override (see its doc comment);
		// the fixed collector.ServerPort constant is used unless a config already sets it.
		port := collector.ServerPort
		if cfg.Collector.Server.ServerPort != 0 {
			port = cfg.Collector.Server.ServerPort
		}
		listener, err = net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			return nil, fmt.Errorf("listening on ALS TCP port %d: %w", port, err)
		}

		go func() {
			slog.Info("Starting to serve access log service server", "mode", "tcp", "port", port)
			if err := server.Serve(listener); err != nil {
				slog.Error("ALS server exited", "error", err)
			}
		}()
	}

	return &AccessLogService{
		Server:    server,
		Analytics: accessLogServiceServer.analytics,
		Verdicts:  accessLogServiceServer.verdicts,
	}, nil
}
