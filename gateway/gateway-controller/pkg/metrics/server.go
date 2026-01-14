/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package metrics

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"go.uber.org/zap"
)

// Server is the metrics HTTP server
type Server struct {
	cfg        *config.MetricsConfig
	httpServer *http.Server
	log        *zap.Logger
}

// NewServer creates a new metrics server
func NewServer(cfg *config.MetricsConfig, log *zap.Logger) *Server {
	// Initialize metrics registry
	registry := Init()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	}))

	// Health endpoint for the metrics server itself
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return &Server{
		cfg:        cfg,
		httpServer: httpServer,
		log:        log,
	}
}

// Start starts the metrics HTTP server
func (s *Server) Start() error {
	s.log.Info("Starting metrics HTTP server", zap.Int("port", s.cfg.Port))

	ln, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return fmt.Errorf("metrics server failed to bind: %w", err)
	}

	go func() {
		if err := s.httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			s.log.Error("Metrics server failed", zap.Error(err))
		}
	}()

	return nil
}

// Stop gracefully stops the metrics HTTP server
func (s *Server) Stop(ctx context.Context) error {
	s.log.Info("Stopping metrics HTTP server")
	return s.httpServer.Shutdown(ctx)
}

// StartMemoryMetricsUpdater starts a goroutine that periodically updates memory metrics
func StartMemoryMetricsUpdater(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				UpdateMemoryMetrics()
			}
		}
	}()
}
