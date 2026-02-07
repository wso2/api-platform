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
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
)

// Server is the metrics HTTP server
type Server struct {
	cfg        *config.MetricsConfig
	httpServer *http.Server
}

// NewServer creates a new metrics server
func NewServer(cfg *config.MetricsConfig) *Server {
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
	}
}

// Start starts the metrics HTTP server
func (s *Server) Start(ctx context.Context) error {
	slog.InfoContext(ctx, "Starting metrics HTTP server", "port", s.cfg.Port)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("metrics server error: %w", err)
	}

	return nil
}

// Stop gracefully stops the metrics HTTP server
func (s *Server) Stop(ctx context.Context) error {
	slog.InfoContext(ctx, "Stopping metrics HTTP server")
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
