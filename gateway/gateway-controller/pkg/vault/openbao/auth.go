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

package openbao

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// renewFraction is how far into a login lease the token is replaced, so a
// request never goes out with a token about to expire.
const renewFraction = 0.8

// authenticator supplies the token for each request.
type authenticator interface {
	// token returns a cached token, obtaining a new one when none is cached.
	token(ctx context.Context) (string, error)
	// invalidate drops tok if it is still the cached token.
	invalidate(tok string)
	method() string
}

// tokenFileAuth reads a token from a file. The token is cached and the file
// re-read after a 403, so a token rotated on disk takes effect without a restart.
type tokenFileAuth struct {
	path string

	mu     sync.Mutex
	cached string
}

func (a *tokenFileAuth) method() string { return "token_file" }

func (a *tokenFileAuth) token(context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cached != "" {
		return a.cached, nil
	}
	tok, err := readCredentialFile(a.path)
	if err != nil {
		return "", fmt.Errorf("reading token_file: %w", err)
	}
	a.cached = tok
	return tok, nil
}

func (a *tokenFileAuth) invalidate(tok string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cached == tok {
		a.cached = ""
	}
}

// kubernetesAuth logs in through auth/{mount_path}/login with the pod's
// service account JWT, caches the client token and logs in again once
// renewFraction of the lease has passed or after a 403.
type kubernetesAuth struct {
	cfg KubernetesAuthConfig
	kv  *KV
	now func() time.Time

	mu        sync.Mutex
	cached    string
	refreshAt time.Time // zero when the token has no lease expiry
}

func (a *kubernetesAuth) method() string { return "kubernetes" }

func (a *kubernetesAuth) token(ctx context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cached != "" && (a.refreshAt.IsZero() || a.now().Before(a.refreshAt)) {
		return a.cached, nil
	}
	return a.login(ctx)
}

func (a *kubernetesAuth) invalidate(tok string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cached == tok {
		a.cached = ""
	}
}

// login must be called with a.mu held.
func (a *kubernetesAuth) login(ctx context.Context) (string, error) {
	// The projected service account token rotates, so read it on every login.
	jwt, err := readCredentialFile(a.cfg.JWTFile)
	if err != nil {
		return "", fmt.Errorf("reading kubernetes jwt_file: %w", err)
	}
	body, err := json.Marshal(map[string]string{"role": a.cfg.Role, "jwt": jwt})
	if err != nil {
		return "", fmt.Errorf("encoding kubernetes login: %w", err)
	}
	var resp struct {
		Auth struct {
			ClientToken   string `json:"client_token"`
			LeaseDuration int64  `json:"lease_duration"`
		} `json:"auth"`
	}
	status, err := a.kv.do(ctx, http.MethodPost, "auth/"+a.cfg.MountPath+"/login", nil, bytes.NewReader(body), "", &resp)
	if err != nil {
		return "", fmt.Errorf("kubernetes login: %w", err)
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("kubernetes login: openbao returned HTTP %d", status)
	}
	if resp.Auth.ClientToken == "" {
		return "", errors.New("kubernetes login: openbao returned no client token")
	}

	a.cached = resp.Auth.ClientToken
	a.refreshAt = time.Time{}
	if resp.Auth.LeaseDuration > 0 {
		lease := time.Duration(resp.Auth.LeaseDuration) * time.Second
		a.refreshAt = a.now().Add(time.Duration(float64(lease) * renewFraction))
	}
	a.kv.logger.Debug("Logged in to OpenBao with kubernetes auth",
		slog.String("resolver", a.kv.name),
		slog.String("role", a.cfg.Role),
		slog.Int64("lease_seconds", resp.Auth.LeaseDuration),
	)
	return a.cached, nil
}
