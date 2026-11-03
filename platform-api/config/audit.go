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

package config

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/url"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/http/httpguts"
)

const (
	AuditOutputStdout = "stdout"
	AuditOutputFile   = "file"
	AuditOutputHTTP   = "http"
)

const (
	AuditAuthNone   = "none"
	AuditAuthBearer = "bearer"
	AuditAuthBasic  = "basic"
	AuditAuthHeader = "header"
)

type Audit struct {
	Enabled bool      `koanf:"enabled"`
	MaskPII bool      `koanf:"mask_pii"`
	Outputs []string  `koanf:"outputs"`
	File    AuditFile `koanf:"file"`
	HTTP    AuditHTTP `koanf:"http"`
}

type AuditFile struct {
	Path      string `koanf:"path"`
	MaxSizeMB int    `koanf:"max_size_mb"`
}

type AuditHTTP struct {
	// Endpoint must be https:// unless AllowInsecureTransport is set.
	Endpoint               string        `koanf:"endpoint"`
	AllowInsecureTransport bool          `koanf:"allow_insecure_transport"`
	BatchMaxEvents         int           `koanf:"batch_max_events"`
	BatchMaxBytes          int           `koanf:"batch_max_bytes"`
	FlushInterval          time.Duration `koanf:"flush_interval"`
	QueueCapacity          int           `koanf:"queue_capacity"`
	RequestTimeout         time.Duration `koanf:"request_timeout"`
	MaxRetries             int           `koanf:"max_retries"`
	RetryBackoff           time.Duration `koanf:"retry_backoff"`
	Auth                   AuditHTTPAuth `koanf:"auth"`
	TLS                    AuditHTTPTLS  `koanf:"tls"`
}

type AuditHTTPAuth struct {
	Type   string `koanf:"type"`
	Bearer struct {
		Token string `koanf:"token"`
	} `koanf:"bearer"`
	Basic struct {
		Username string `koanf:"username"`
		Password string `koanf:"password"`
	} `koanf:"basic"`
	Header struct {
		Name  string `koanf:"name"`
		Value string `koanf:"value"`
	} `koanf:"header"`
}

type AuditHTTPTLS struct {
	CAFile             string `koanf:"ca_file"`
	CertFile           string `koanf:"cert_file"`
	KeyFile            string `koanf:"key_file"`
	InsecureSkipVerify bool   `koanf:"insecure_skip_verify"`
}

// AuthHeader returns the header to send with every batch; an empty name means
// none.
func (a AuditHTTPAuth) AuthHeader() (name, value string, err error) {
	switch strings.ToLower(strings.TrimSpace(a.Type)) {
	case "", AuditAuthNone:
		return "", "", nil
	case AuditAuthBearer:
		if a.Bearer.Token == "" {
			return "", "", fmt.Errorf("bearer.token is required when type is %q", AuditAuthBearer)
		}
		name, value = "Authorization", "Bearer "+a.Bearer.Token
	case AuditAuthBasic:
		if a.Basic.Username == "" || a.Basic.Password == "" {
			return "", "", fmt.Errorf("basic.username and basic.password are both required when type is %q", AuditAuthBasic)
		}
		// RFC 7617: the first colon separates the user-id from the password.
		if strings.Contains(a.Basic.Username, ":") {
			return "", "", fmt.Errorf("basic.username must not contain a colon")
		}
		name, value = "Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(a.Basic.Username+":"+a.Basic.Password))
	case AuditAuthHeader:
		if a.Header.Name == "" || a.Header.Value == "" {
			return "", "", fmt.Errorf("header.name and header.value are both required when type is %q", AuditAuthHeader)
		}
		name, value = a.Header.Name, a.Header.Value
	default:
		return "", "", fmt.Errorf("unknown type %q (valid: %s, %s, %s, %s)",
			a.Type, AuditAuthNone, AuditAuthBearer, AuditAuthBasic, AuditAuthHeader)
	}
	// A bad name, or a value with a line break (e.g. from an {{ env }} token),
	// would fail every delivery; catch it at startup instead.
	if !httpguts.ValidHeaderFieldName(name) {
		return "", "", fmt.Errorf("header name %q is not a valid HTTP header name", name)
	}
	if !httpguts.ValidHeaderFieldValue(value) {
		return "", "", fmt.Errorf("the %s header value contains characters not allowed in HTTP", name)
	}
	return name, value, nil
}

func validateAuditConfig(cfg *Audit) error {
	if !cfg.Enabled {
		return nil
	}
	outputs, err := NormalizeAuditOutputs(cfg.Outputs)
	if err != nil {
		return err
	}
	if slices.Contains(outputs, AuditOutputFile) {
		if err := validateAuditFileConfig(cfg.File); err != nil {
			return fmt.Errorf("audit.file: %w", err)
		}
	}
	if slices.Contains(outputs, AuditOutputHTTP) {
		if err := validateAuditHTTPConfig(cfg.HTTP); err != nil {
			return fmt.Errorf("audit.http: %w", err)
		}
	}
	return nil
}

// NormalizeAuditOutputs lower-cases and trims the output names, rejecting an
// unknown or repeated name. The outputs are built from the same list.
func NormalizeAuditOutputs(outputs []string) ([]string, error) {
	normalized := make([]string, 0, len(outputs))
	for _, raw := range outputs {
		out := strings.ToLower(strings.TrimSpace(raw))
		if out == "" {
			continue
		}
		switch out {
		case AuditOutputStdout, AuditOutputFile, AuditOutputHTTP:
		default:
			return nil, fmt.Errorf("audit.outputs: unknown output %q (valid: %s, %s, %s)",
				raw, AuditOutputStdout, AuditOutputFile, AuditOutputHTTP)
		}
		if slices.Contains(normalized, out) {
			return nil, fmt.Errorf("audit.outputs lists %q more than once", out)
		}
		normalized = append(normalized, out)
	}
	return normalized, nil
}

// maxAuditFileSizeMB (1 TiB) keeps the rotation threshold in bytes from overflowing.
const maxAuditFileSizeMB = 1 << 20

// maxAuditRetryBackoff keeps the doubling retry delay from overflowing.
const maxAuditRetryBackoff = time.Hour

func validateAuditFileConfig(cfg AuditFile) error {
	path, err := ResolveAuditFilePath(cfg.Path)
	if err != nil {
		return err
	}
	if cfg.MaxSizeMB < 0 || cfg.MaxSizeMB > maxAuditFileSizeMB {
		return fmt.Errorf("max_size_mb must be between 0 and %d, got %d", maxAuditFileSizeMB, cfg.MaxSizeMB)
	}
	if cfg.MaxSizeMB == 0 {
		slog.Warn("audit.file.max_size_mb is 0: rotation is disabled and the audit log grows "+
			"until the volume is full", "path", path)
	}
	return nil
}

// ResolveAuditFilePath validates and cleans the audit log file path.
func ResolveAuditFilePath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("path is required when the %q output is selected", AuditOutputFile)
	}
	if strings.ContainsRune(path, 0) {
		return "", fmt.Errorf("path must not contain a null byte")
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("path must be absolute, got %q", path)
	}
	// filepath.Clean resolves ".." rather than rejecting it, which would move the
	// file out of the directory the operator chose.
	for _, segment := range strings.Split(path, string(filepath.Separator)) {
		if segment == ".." {
			return "", fmt.Errorf("path must not contain %q segments, got %q", "..", path)
		}
	}
	return filepath.Clean(path), nil
}

func validateAuditHTTPConfig(cfg AuditHTTP) error {
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return fmt.Errorf("endpoint is required when the %q output is selected", AuditOutputHTTP)
	}
	u, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return fmt.Errorf("endpoint is not a valid URL: %w", err)
	}
	switch u.Scheme {
	case "https":
		if cfg.TLS.InsecureSkipVerify {
			slog.Warn("audit.http.tls.insecure_skip_verify is true: the receiver's certificate is not "+
				"verified, so audit records are exposed to anyone able to intercept the connection",
				"host", u.Host)
		}
	case "http":
		if !cfg.AllowInsecureTransport {
			return fmt.Errorf("endpoint uses plaintext http:// but allow_insecure_transport is false; " +
				"set it only for a trusted collector on the same network")
		}
		slog.Warn("audit.http endpoint is plaintext http://; audit records are sent unencrypted", "host", u.Host)
	default:
		return fmt.Errorf("endpoint scheme must be https (or http with allow_insecure_transport), got %q", u.Scheme)
	}
	if u.Hostname() == "" {
		return fmt.Errorf("endpoint must include a host")
	}
	if port := u.Port(); port != "" {
		if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
			return fmt.Errorf("endpoint port must be between 1 and 65535, got %q", port)
		}
	}

	if cfg.BatchMaxEvents <= 0 {
		return fmt.Errorf("batch_max_events must be positive, got %d", cfg.BatchMaxEvents)
	}
	if cfg.BatchMaxBytes <= 0 {
		return fmt.Errorf("batch_max_bytes must be positive, got %d", cfg.BatchMaxBytes)
	}
	if cfg.FlushInterval <= 0 {
		return fmt.Errorf("flush_interval must be positive, got %s", cfg.FlushInterval)
	}
	if cfg.QueueCapacity <= 0 {
		return fmt.Errorf("queue_capacity must be positive, got %d", cfg.QueueCapacity)
	}
	if cfg.RequestTimeout <= 0 {
		return fmt.Errorf("request_timeout must be positive, got %s", cfg.RequestTimeout)
	}
	if cfg.MaxRetries < 0 {
		return fmt.Errorf("max_retries must be >= 0, got %d", cfg.MaxRetries)
	}
	if cfg.MaxRetries > 0 && cfg.RetryBackoff <= 0 {
		return fmt.Errorf("retry_backoff must be positive when max_retries > 0, got %s", cfg.RetryBackoff)
	}
	if cfg.RetryBackoff > maxAuditRetryBackoff {
		return fmt.Errorf("retry_backoff must be at most %s, got %s", maxAuditRetryBackoff, cfg.RetryBackoff)
	}

	if _, _, err := cfg.Auth.AuthHeader(); err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	if (cfg.TLS.CertFile == "") != (cfg.TLS.KeyFile == "") {
		return fmt.Errorf("tls: cert_file and key_file must be set together")
	}
	return nil
}
