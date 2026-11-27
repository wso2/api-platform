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

package conversionwebhook

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

// Timeouts of the webhook's HTTP server. The kube-apiserver gives a conversion call
// 30 seconds by default, so nothing legitimate needs longer.
const (
	serverReadTimeout       = 30 * time.Second
	serverReadHeaderTimeout = 10 * time.Second
	serverWriteTimeout      = 30 * time.Second
	serverIdleTimeout       = 90 * time.Second
	serverMaxHeaderBytes    = 1 << 20
)

// curvePreferences returns the TLS key-exchange groups in preference order. With
// pqcKeyExchange the hybrid X25519MLKEM768 group comes first; the classical groups
// follow so a peer without it still connects.
func curvePreferences(pqcKeyExchange bool) []tls.CurveID {
	classical := []tls.CurveID{tls.X25519, tls.CurveP256}
	if !pqcKeyExchange {
		return classical
	}
	return append([]tls.CurveID{tls.X25519MLKEM768}, classical...)
}

// tlsConfig is the webhook's TLS configuration: the in-memory serving certificate,
// the key-exchange preference, and HTTP/1.1 only unless enableHTTP2 is set (HTTP/2 is
// off by default for the Rapid Reset and Stream Cancellation CVEs, as for the metrics
// server).
func tlsConfig(store *certStore, pqcKeyExchange, enableHTTP2 bool, logger *slog.Logger) *tls.Config {
	protos := []string{"http/1.1"}
	if enableHTTP2 {
		protos = []string{"h2", "http/1.1"}
	}
	return &tls.Config{
		MinVersion:       tls.VersionTLS12,
		GetCertificate:   store.GetCertificate,
		CurvePreferences: curvePreferences(pqcKeyExchange),
		NextProtos:       protos,
		// The client is the kube-apiserver; there is no client certificate to verify.
		// The hook only reports which key exchange a connection negotiated.
		VerifyConnection: func(cs tls.ConnectionState) error {
			logger.Debug("Conversion webhook TLS connection",
				slog.String("version", tls.VersionName(cs.Version)),
				slog.String("keyExchange", cs.CurveID.String()))
			return nil
		},
	}
}

// limitBody reads at most maxBytes of the request body into memory and rejects a
// larger body with a generic 413, before the conversion handler decodes it.
func limitBody(maxBytes int64, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.ContentLength > maxBytes {
			http.Error(w, "request entity too large", http.StatusRequestEntityTooLarge)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, maxBytes+1))
		if err != nil {
			http.Error(w, "cannot read request", http.StatusBadRequest)
			return
		}
		if int64(len(body)) > maxBytes {
			http.Error(w, "request entity too large", http.StatusRequestEntityTooLarge)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		next.ServeHTTP(w, r)
	})
}

// server serves ConversionReview requests over TLS on every replica: the API server
// can reach any pod behind the webhook Service.
type server struct {
	port            int
	handler         http.Handler
	maxRequestBytes int64
	tls             *tls.Config
	logger          *slog.Logger
	listening       atomic.Bool
}

func (s *server) NeedLeaderElection() bool { return false }

func (s *server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.Handle(ConvertPath, limitBody(s.maxRequestBytes, s.handler))
	srv := &http.Server{
		Handler:           mux,
		TLSConfig:         s.tls,
		ReadTimeout:       serverReadTimeout,
		ReadHeaderTimeout: serverReadHeaderTimeout,
		WriteTimeout:      serverWriteTimeout,
		IdleTimeout:       serverIdleTimeout,
		MaxHeaderBytes:    serverMaxHeaderBytes,
		ErrorLog:          slog.NewLogLogger(s.logger.Handler(), slog.LevelDebug),
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
	ln, err := net.Listen("tcp", net.JoinHostPort("", strconv.Itoa(s.port)))
	if err != nil {
		return fmt.Errorf("conversion webhook listen on port %d: %w", s.port, err)
	}
	s.listening.Store(true)
	s.logger.Info("Serving the conversion webhook", slog.Int("port", s.port), slog.String("path", ConvertPath))

	errCh := make(chan error, 1)
	go func() {
		// Empty file names: the certificate comes from TLSConfig.GetCertificate.
		errCh <- srv.ServeTLS(ln, "", "")
	}()
	select {
	case <-ctx.Done():
		s.listening.Store(false)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("conversion webhook shutdown: %w", err)
		}
		return nil
	case err := <-errCh:
		s.listening.Store(false)
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("conversion webhook server: %w", err)
	}
}
