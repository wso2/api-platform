package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"
)

const (
	defaultPort            = 8090
	defaultReadTimeout     = 10 * time.Second
	defaultWriteTimeout    = 10 * time.Second
	defaultIdleTimeout     = 30 * time.Second
	defaultShutdownTimeout = 3 * time.Second
)

type listenerHandler struct {
	logMu sync.Mutex
}

func (h *listenerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleGet(w, r)
	case http.MethodPost:
		h.handlePost(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed.\n", http.StatusMethodNotAllowed)
	}
}

func (h *listenerHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	h.logRequest("Verification / GET Request", r, nil)

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)

	challenge := r.URL.Query().Get("hub.challenge")
	if challenge != "" {
		_, _ = w.Write([]byte(challenge))
		return
	}

	_, _ = w.Write([]byte("Listener is active. Awaiting requests...\n"))
}

func (h *listenerHandler) handlePost(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body.\n", http.StatusBadRequest)
		return
	}

	h.logRequest("New Webhook Received", r, body)

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("Webhook received successfully!\n"))
}

func (h *listenerHandler) logRequest(title string, r *http.Request, body []byte) {
	headers, err := json.MarshalIndent(r.Header, "", "  ")
	if err != nil {
		headers = []byte("{}")
	}

	h.logMu.Lock()
	defer h.logMu.Unlock()

	fmt.Fprintf(os.Stdout, "\n--- %s on %s ---\n", title, r.URL.RequestURI())
	fmt.Fprintln(os.Stdout, "Headers:")
	fmt.Fprintln(os.Stdout, string(headers))

	if body != nil {
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "Body:")
		fmt.Fprintln(os.Stdout, formatBody(body))
	}

	fmt.Fprintln(os.Stdout, "-------------------------------------------")
}

func formatBody(body []byte) string {
	if len(body) == 0 {
		return "<empty>"
	}

	trimmed := bytes.TrimSpace(body)
	if json.Valid(trimmed) {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, trimmed, "", "  "); err == nil {
			return pretty.String()
		}
	}

	if utf8.Valid(body) {
		return string(body)
	}

	return fmt.Sprintf("%x", body)
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	port := defaultPort
	if rawPort := os.Getenv("PORT"); rawPort != "" {
		parsedPort, err := strconv.Atoi(rawPort)
		if err != nil {
			logger.Error("Invalid PORT environment variable", "value", rawPort, "error", err)
			os.Exit(1)
		}
		port = parsedPort
	}

	shutdownTimeout := defaultShutdownTimeout
	if rawTimeout := os.Getenv("SHUTDOWN_TIMEOUT"); rawTimeout != "" {
		parsedTimeout, err := time.ParseDuration(rawTimeout)
		if err != nil {
			logger.Error("Invalid SHUTDOWN_TIMEOUT environment variable", "value", rawTimeout, "error", err)
			os.Exit(1)
		}
		shutdownTimeout = parsedTimeout
	}

	flag.IntVar(&port, "port", port, "Port to listen on")
	flag.DurationVar(&shutdownTimeout, "shutdown-timeout", shutdownTimeout, "Graceful shutdown timeout")
	flag.Parse()

	handler := &listenerHandler{}
	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           handler,
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       defaultReadTimeout,
		WriteTimeout:      defaultWriteTimeout,
		IdleTimeout:       defaultIdleTimeout,
	}

	serverErrCh := make(chan error, 1)
	go func() {
		logger.Info("Webhook listener started", "addr", fmt.Sprintf("0.0.0.0:%d", port))
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrCh <- err
			return
		}
		serverErrCh <- nil
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case err := <-serverErrCh:
		if err != nil {
			logger.Error("Webhook listener stopped unexpectedly", "error", err)
			os.Exit(1)
		}
		return
	case sig := <-sigCh:
		logger.Info("Shutdown signal received", "signal", sig.String())
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Warn("Graceful shutdown timed out, forcing close", "error", err)
		if closeErr := server.Close(); closeErr != nil && !errors.Is(closeErr, http.ErrServerClosed) {
			logger.Error("Forced close failed", "error", closeErr)
			os.Exit(1)
		}
	}

	if err := <-serverErrCh; err != nil {
		logger.Error("Webhook listener stopped with error", "error", err)
		os.Exit(1)
	}

	logger.Info("Webhook listener stopped")
}
