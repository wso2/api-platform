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

package testbench

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// healthPath is exposed on every service port.
const healthPath = "/testbench/health"

// Server timeout, header, and body limits, applied uniformly to every mock service.
const (
	readTimeout    = 30 * time.Second
	writeTimeout   = 30 * time.Second
	idleTimeout    = 120 * time.Second
	maxHeaderBytes = 1 << 20  // 1 MiB
	maxBodyBytes   = 10 << 20 // 10 MiB
)

// limitBody caps the request body every mock service reads, then consumes whatever the handler
// left behind. net/http cannot reuse a connection whose request body was not read, and closing a
// socket that still holds unread data emits a TCP reset - so a client POSTing a body to a handler
// that ignores it (a /debug/reset, say) intermittently reads that reset instead of the response
// the server already wrote. The drain is bounded by the MaxBytesReader installed above.
func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		next.ServeHTTP(w, r)
		drained, err := io.Copy(io.Discard, r.Body)
		info, ok := requestInfoFrom(r.Context())
		if ok {
			info.bodyBytes += drained
		}
		if err != nil {
			// Reported rather than swallowed: this is where an oversized body surfaces for
			// a handler that never read it, and the only place it is observable at all.
			if ok {
				info.truncated = true
			}
			ServiceLogger(r.Context()).Warn("testbench could not drain the request body",
				"drained_bytes", drained, "limit_bytes", maxBodyBytes, "error", err)
		}
	})
}

// Serve starts every registered service on its own port and blocks until ctx is cancelled.
func Serve(ctx context.Context, reg *Registry, log *slog.Logger) error {
	if ctx == nil {
		return errors.New("testbench: nil context")
	}
	if reg == nil {
		return errors.New("testbench: nil registry")
	}
	if log == nil {
		return errors.New("testbench: nil logger")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	// The testbench is its own binary, so making the configured logger the default lets any
	// service log through slog without threading a logger into all thirteen of them.
	slog.SetDefault(log)

	services := reg.Services()
	if len(services) == 0 {
		return errors.New("testbench: no services registered")
	}
	names := make([]string, 0, len(services))
	for _, svc := range services {
		names = append(names, svc.Name())
	}
	log.Info("testbench starting",
		"services", strings.Join(names, ","),
		"service_count", len(services),
		"read_timeout", readTimeout,
		"write_timeout", writeTimeout,
		"idle_timeout", idleTimeout,
		"max_body_bytes", maxBodyBytes,
		"max_header_bytes", maxHeaderBytes,
	)

	listeners := make([]net.Listener, 0, len(services))
	servers := make([]*http.Server, 0, len(services))

	for _, svc := range services {
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", svc.Port()))
		if err != nil {
			for _, opened := range listeners {
				_ = opened.Close()
			}
			log.Error("testbench could not listen",
				"service", svc.Name(), "port", svc.Port(), "error", err)
			return fmt.Errorf("testbench: service %q: listening on port %d: %w",
				svc.Name(), svc.Port(), err)
		}
		listeners = append(listeners, listener)

		mux := http.NewServeMux()
		mux.HandleFunc(healthPath, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if _, err := w.Write([]byte(`{"status":"healthy"}`)); err != nil {
				log.Warn("testbench health response failed", "error", err)
			}
		})
		mux.Handle("/", svc.Handler())

		srv := &http.Server{
			Addr:              fmt.Sprintf(":%d", svc.Port()),
			Handler:           observability(svc.Name(), limitBody(mux)),
			ReadTimeout:       readTimeout,
			WriteTimeout:      writeTimeout,
			IdleTimeout:       idleTimeout,
			ReadHeaderTimeout: 10 * time.Second,
			MaxHeaderBytes:    maxHeaderBytes,
		}
		servers = append(servers, srv)
	}

	errCh := make(chan error, len(servers))
	var wg sync.WaitGroup
	for i, svc := range services {
		server := servers[i]
		listener := listeners[i]
		wg.Add(1)
		go func(svc Service, srv *http.Server) {
			defer wg.Done()
			log.Info("testbench service listening",
				"service", svc.Name(), "port", svc.Port(), "stateful", svc.Stateful())
			if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- fmt.Errorf("testbench: service %q: %w", svc.Name(), err)
			}
		}(svc, server)
	}

	shutdown := func(reason string) error {
		log.Info("testbench shutting down", "reason", reason, "servers", len(servers))
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var shutdownErrs []error
		for i, srv := range servers {
			if err := srv.Shutdown(shutdownCtx); err != nil {
				log.Error("testbench service shutdown failed",
					"service", services[i].Name(), "port", services[i].Port(), "error", err)
				shutdownErrs = append(shutdownErrs, err)
				continue
			}
			log.Debug("testbench service stopped",
				"service", services[i].Name(), "port", services[i].Port())
		}
		wg.Wait()
		if len(shutdownErrs) == 0 {
			log.Info("testbench stopped cleanly", "reason", reason)
		}
		return errors.Join(shutdownErrs...)
	}

	select {
	case <-ctx.Done():
		return shutdown("context cancelled")
	case err := <-errCh:
		log.Error("testbench service failed; stopping every service", "error", err)
		return errors.Join(err, shutdown("service failure"))
	}
}
