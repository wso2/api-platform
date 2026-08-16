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
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"
)

// healthPath is exposed on every service port.
const healthPath = "/testbench/health"

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

	services := reg.Services()
	if len(services) == 0 {
		return errors.New("testbench: no services registered")
	}

	listeners := make([]net.Listener, 0, len(services))
	servers := make([]*http.Server, 0, len(services))

	for _, svc := range services {
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", svc.Port()))
		if err != nil {
			for _, opened := range listeners {
				_ = opened.Close()
			}
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
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
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
			log.Info("testbench service listening", "service", svc.Name(), "port", svc.Port())
			if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- fmt.Errorf("testbench: service %q: %w", svc.Name(), err)
			}
		}(svc, server)
	}

	shutdown := func() error {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var shutdownErrs []error
		for _, srv := range servers {
			if err := srv.Shutdown(shutdownCtx); err != nil {
				shutdownErrs = append(shutdownErrs, err)
			}
		}
		wg.Wait()
		return errors.Join(shutdownErrs...)
	}

	select {
	case <-ctx.Done():
		return shutdown()
	case err := <-errCh:
		return errors.Join(err, shutdown())
	}
}
