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

// Package vault resolves the credentials that {{ secret "..." }} placeholders
// name. A plain handle reads the gateway's encrypted database through the DB
// adapter, exactly as before. A kv://<resolver>/<path>#<field> reference reads
// an existing entry from a configured external store, such as an OpenBao KV v2
// mount, and keeps the value in memory only.
package vault

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
)

var (
	// ErrNotFound means the referenced secret, entry, version or field does not exist.
	ErrNotFound = errors.New("secret not found")
	// ErrAccessDenied means the store refused the gateway's identity.
	ErrAccessDenied = errors.New("access to secret denied")
	// ErrUnavailable means the store could not be reached or answered with a
	// server-side failure. The same read may succeed later, so callers retry
	// instead of treating the artifact as invalid.
	ErrUnavailable = errors.New("secret store unavailable")
)

// Adapter is one backing store of the gateway vault.
type Adapter interface {
	// Type names the backend, such as "db" or "openbao".
	Type() string
	// Get returns the plaintext credential that ref names.
	Get(ctx context.Context, ref Ref) (string, error)
}

// Vault routes each reference to the adapter that owns it. It satisfies the
// template engine's SecretResolver and the control-plane secret syncer, so it
// replaces the secret service wherever artifacts are rendered.
type Vault struct {
	local     *DBAdapter
	resolvers map[string]Adapter
	logger    *slog.Logger
}

// New builds a vault. local may be nil when the gateway has no encrypted
// store; plain handles then fail to resolve. resolvers maps each configured
// resolver name to its adapter.
func New(local *DBAdapter, resolvers map[string]Adapter, logger *slog.Logger) *Vault {
	if logger == nil {
		logger = slog.Default()
	}
	r := make(map[string]Adapter, len(resolvers))
	for name, a := range resolvers {
		r[name] = a
	}
	return &Vault{local: local, resolvers: r, logger: logger}
}

// ResolverNames lists the configured external resolvers in sorted order.
func (v *Vault) ResolverNames() []string {
	names := make([]string, 0, len(v.resolvers))
	for name := range v.resolvers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Resolve returns the plaintext value for a {{ secret "..." }} argument.
func (v *Vault) Resolve(reference string) (string, error) {
	return v.Get(context.Background(), reference)
}

// Get parses reference and reads it from the owning adapter.
func (v *Vault) Get(ctx context.Context, reference string) (string, error) {
	ref, err := ParseRef(reference)
	if err != nil {
		return "", err
	}
	if !ref.IsExternal() {
		if v.local == nil {
			return "", errors.New("the gateway's local secret store is not configured")
		}
		return v.local.Get(ctx, ref)
	}

	adapter, ok := v.resolvers[ref.Resolver]
	if !ok {
		return "", fmt.Errorf("secret resolver %q is not configured on this gateway", ref.Resolver)
	}
	value, err := adapter.Get(ctx, ref)
	if err != nil {
		v.logger.Warn("Failed to read secret from external resolver",
			slog.String("resolver", ref.Resolver),
			slog.String("type", adapter.Type()),
			slog.String("path", ref.Path),
			slog.String("field", ref.Field),
			slog.Int("version", ref.Version),
			slog.Any("error", err),
		)
		return "", err
	}
	v.logger.Debug("Resolved secret from external resolver",
		slog.String("resolver", ref.Resolver),
		slog.String("type", adapter.Type()),
		slog.String("path", ref.Path),
		slog.Int("version", ref.Version),
	)
	return value, nil
}

// UpsertFromPlatform stores a control-plane secret in the local encrypted
// store. External references are never written: their owners provision them.
func (v *Vault) UpsertFromPlatform(handle, displayName, plaintext string) error {
	if IsExternalRef(handle) {
		return fmt.Errorf("refusing to store external reference %q locally", handle)
	}
	if v.local == nil {
		return errors.New("the gateway's local secret store is not configured")
	}
	return v.local.Upsert(handle, displayName, plaintext)
}
