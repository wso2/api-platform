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

// Package openbao implements a read-only gateway vault adapter for the KV v2
// secrets engine of OpenBao or HashiCorp Vault. The gateway reads an entry
// that its owner (AMP or the customer) has already written; it never writes,
// rotates or deletes it. The identity it uses needs only "read" on
// {mount}/data/<paths it is assigned>.
package openbao

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/vault"
)

const (
	// Type is the adapter type name used in configuration.
	Type = "openbao"

	// DefaultMount is the KV v2 mount used when none is configured.
	DefaultMount = "secret"
	// DefaultKubernetesMountPath is the Kubernetes auth mount used when none is configured.
	DefaultKubernetesMountPath = "kubernetes"
	// DefaultKubernetesJWTFile is the projected service account token path in a pod.
	DefaultKubernetesJWTFile = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	// DefaultRequestTimeout bounds every call to the server.
	DefaultRequestTimeout = 10 * time.Second

	// maxResponseBytes caps how much of a response is read.
	maxResponseBytes = 1 << 20
	// maxCredentialFileBytes caps the token and JWT files read from disk.
	maxCredentialFileBytes = 64 << 10
)

// KubernetesAuthConfig configures login through the Kubernetes auth method.
type KubernetesAuthConfig struct {
	Role      string // role bound to the pod's service account
	MountPath string // auth method mount path, default "kubernetes"
	JWTFile   string // service account token file, re-read on every login
}

// Config holds the settings for one KV resolver.
type Config struct {
	Name           string // resolver name used in kv://<name>/... references
	Address        string // base URL, e.g. https://openbao.example:8200
	Mount          string // KV v2 mount path, default "secret"
	Namespace      string // optional namespace sent as X-Vault-Namespace
	CAFile         string // optional PEM bundle used to verify the server certificate
	TLSSkipVerify  bool   // disables certificate verification (testing only)
	RequestTimeout time.Duration

	// Exactly one of TokenFile and Kubernetes must be set.
	TokenFile  string
	Kubernetes *KubernetesAuthConfig

	// HTTPClient overrides the client built from the TLS settings. Used by tests.
	HTTPClient *http.Client
}

// KV reads entries from one KV v2 mount.
type KV struct {
	name      string
	baseURL   *url.URL
	mount     string
	namespace string
	timeout   time.Duration
	client    *http.Client
	auth      authenticator
	logger    *slog.Logger
}

var _ vault.Adapter = (*KV)(nil)

// New validates cfg and builds the adapter. It does not contact the server,
// so a gateway starts even when the vault is briefly unavailable; artifacts
// that reference it fail to render until it is reachable.
func New(cfg Config, logger *slog.Logger) (*KV, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if !vault.ValidResolverName(cfg.Name) {
		return nil, fmt.Errorf("openbao resolver name %q must be a lower-case DNS label", cfg.Name)
	}
	base, err := parseAddress(cfg.Address)
	if err != nil {
		return nil, err
	}
	mount := strings.Trim(strings.TrimSpace(cfg.Mount), "/")
	if mount == "" {
		mount = DefaultMount
	}
	if err := vault.ValidatePath(mount); err != nil {
		return nil, fmt.Errorf("openbao mount: %w", err)
	}
	namespace := strings.Trim(strings.TrimSpace(cfg.Namespace), "/")
	if namespace != "" {
		if err := vault.ValidatePath(namespace); err != nil {
			return nil, fmt.Errorf("openbao namespace: %w", err)
		}
	}
	timeout := cfg.RequestTimeout
	if timeout <= 0 {
		timeout = DefaultRequestTimeout
	}
	client := cfg.HTTPClient
	if client == nil {
		client, err = newHTTPClient(cfg.CAFile, cfg.TLSSkipVerify)
		if err != nil {
			return nil, err
		}
	}

	kv := &KV{
		name:      cfg.Name,
		baseURL:   base,
		mount:     mount,
		namespace: namespace,
		timeout:   timeout,
		client:    client,
		logger:    logger,
	}

	hasToken := strings.TrimSpace(cfg.TokenFile) != ""
	hasKube := cfg.Kubernetes != nil && strings.TrimSpace(cfg.Kubernetes.Role) != ""
	switch {
	case hasToken && hasKube:
		return nil, errors.New("openbao auth must set exactly one of token_file and kubernetes, not both")
	case hasToken:
		kv.auth = &tokenFileAuth{path: cfg.TokenFile}
	case hasKube:
		k := *cfg.Kubernetes
		k.MountPath = strings.Trim(strings.TrimSpace(k.MountPath), "/")
		if k.MountPath == "" {
			k.MountPath = DefaultKubernetesMountPath
		}
		if err := vault.ValidatePath(k.MountPath); err != nil {
			return nil, fmt.Errorf("openbao kubernetes mount_path: %w", err)
		}
		if strings.TrimSpace(k.JWTFile) == "" {
			k.JWTFile = DefaultKubernetesJWTFile
		}
		kv.auth = &kubernetesAuth{cfg: k, kv: kv, now: time.Now}
	default:
		return nil, errors.New("openbao auth requires token_file or kubernetes.role")
	}

	if cfg.TLSSkipVerify {
		logger.Warn("OpenBao resolver is not verifying the server certificate (tls_skip_verify = true)",
			slog.String("resolver", cfg.Name))
	}
	if base.Scheme == "http" {
		logger.Warn("OpenBao resolver is using plaintext HTTP; use https outside local development",
			slog.String("resolver", cfg.Name), slog.String("address", base.Host))
	}
	logger.Info("OpenBao KV resolver initialized",
		slog.String("resolver", cfg.Name),
		slog.String("address", base.Host),
		slog.String("mount", mount),
		slog.String("auth", kv.auth.method()),
	)
	return kv, nil
}

// Type returns "openbao".
func (k *KV) Type() string { return Type }

// kvReadResponse is the KV v2 read response.
type kvReadResponse struct {
	Data struct {
		Data     map[string]any `json:"data"`
		Metadata struct {
			Version      int    `json:"version"`
			DeletionTime string `json:"deletion_time"`
			Destroyed    bool   `json:"destroyed"`
		} `json:"metadata"`
	} `json:"data"`
}

// Get reads ref.Field from the entry at ref.Path, pinned to ref.Version when set.
func (k *KV) Get(ctx context.Context, ref vault.Ref) (string, error) {
	if ref.Resolver != k.name {
		return "", fmt.Errorf("reference for resolver %q sent to resolver %q", ref.Resolver, k.name)
	}
	if err := vault.ValidatePath(ref.Path); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, k.timeout)
	defer cancel()

	var query url.Values
	if ref.Version > 0 {
		query = url.Values{"version": []string{strconv.Itoa(ref.Version)}}
	}

	var resp kvReadResponse
	status, err := k.call(ctx, http.MethodGet, k.mount+"/data/"+ref.Path, query, &resp)
	if err != nil {
		return "", err
	}
	switch status {
	case http.StatusOK:
	case http.StatusNotFound:
		return "", fmt.Errorf("%w: no entry at %s", vault.ErrNotFound, ref.String())
	case http.StatusForbidden:
		return "", fmt.Errorf("%w: %s", vault.ErrAccessDenied, ref.String())
	default:
		return "", fmt.Errorf("openbao returned HTTP %d for %s", status, ref.String())
	}

	// A soft-deleted or destroyed version comes back with no data.
	if resp.Data.Data == nil || resp.Data.Metadata.Destroyed || resp.Data.Metadata.DeletionTime != "" {
		return "", fmt.Errorf("%w: %s is deleted", vault.ErrNotFound, ref.String())
	}
	raw, ok := resp.Data.Data[ref.Field]
	if !ok {
		return "", fmt.Errorf("%w: field %q is not in %s", vault.ErrNotFound, ref.Field, ref.String())
	}
	value, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("field %q in %s is not a string", ref.Field, ref.String())
	}
	return value, nil
}

// call sends one authenticated request. A 403 means the token expired, was
// revoked or rotated on disk, so the token is dropped and the request retried
// once with a fresh one. It returns the final HTTP status.
func (k *KV) call(ctx context.Context, method, path string, query url.Values, out any) (int, error) {
	for attempt := 0; ; attempt++ {
		token, err := k.auth.token(ctx)
		if err != nil {
			return 0, fmt.Errorf("openbao authentication failed: %w", err)
		}
		status, err := k.do(ctx, method, path, query, nil, token, out)
		if err != nil {
			return 0, err
		}
		if status == http.StatusForbidden && attempt == 0 {
			k.logger.Debug("OpenBao returned 403; refreshing credentials and retrying",
				slog.String("resolver", k.name), slog.String("auth", k.auth.method()))
			k.auth.invalidate(token)
			continue
		}
		return status, nil
	}
}

// do sends one request and decodes out only on 200. Response bodies are never
// logged or put into errors because they can hold credentials.
func (k *KV) do(ctx context.Context, method, path string, query url.Values, body io.Reader, token string, out any) (int, error) {
	endpoint := k.baseURL.JoinPath("v1", path)
	if query != nil {
		endpoint.RawQuery = query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return 0, fmt.Errorf("building openbao request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("X-Vault-Token", token)
	}
	if k.namespace != "" {
		req.Header.Set("X-Vault-Namespace", k.namespace)
	}
	resp, err := k.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("request to openbao failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return 0, fmt.Errorf("reading openbao response: %w", err)
	}
	if len(data) > maxResponseBytes {
		return 0, errors.New("openbao response exceeds the maximum allowed size")
	}
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, nil
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return 0, errors.New("openbao response is not valid JSON")
		}
	}
	return resp.StatusCode, nil
}

func parseAddress(address string) (*url.URL, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return nil, errors.New("openbao address is required")
	}
	u, err := url.Parse(address)
	if err != nil {
		return nil, fmt.Errorf("openbao address is not a valid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("openbao address must use http or https, got %q", u.Scheme)
	}
	if u.Host == "" {
		return nil, errors.New("openbao address must include a host")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("openbao address must not contain credentials, a query or a fragment")
	}
	return u, nil
}

func newHTTPClient(caFile string, skipVerify bool) (*http.Client, error) {
	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: skipVerify} //nolint:gosec // opt-in, warned at startup
	if caFile != "" {
		pem, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("reading openbao ca_file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, errors.New("openbao ca_file contains no PEM certificates")
		}
		tlsCfg.RootCAs = pool
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsCfg
	return &http.Client{
		Transport: transport,
		// Never follow redirects: a redirect would resend the token elsewhere.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}, nil
}

func readCredentialFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxCredentialFileBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxCredentialFileBytes {
		return "", errors.New("file is too large")
	}
	value := strings.TrimSpace(string(data))
	if value == "" {
		return "", errors.New("file is empty")
	}
	return value, nil
}
