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

// Package common contains reusable integration-suite step mechanics.
package common

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strings"
	"sync"

	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
)

var configureExpansion sync.Once

// ConfigureExpansion installs context-backed expansion for suite placeholders.
func ConfigureExpansion() {
	configureExpansion.Do(func() {
		unique.ContextValue = func(ctx context.Context, name string) (string, error) {
			return tcontext.ResolveString(ctx, name)
		}
	})
}

// Expand resolves unique-name and context-value placeholders in a suite value.
func Expand(ctx context.Context, value string) (string, error) {
	ConfigureExpansion()
	return unique.Expand(ctx, value)
}

// GenerateAndStore creates a unique value and stores it in the runner's local scope.
func GenerateAndStore(ctx context.Context, base, key string) error {
	return generateAndStore(ctx, key, func() (string, error) {
		return unique.Unique(ctx, base)
	})
}

// GenerateResourceAndStore creates a unique name accepted by DNS-style resource APIs.
func GenerateResourceAndStore(ctx context.Context, base, key string) error {
	return generateAndStore(ctx, key, func() (string, error) {
		value, err := unique.Unique(ctx, base)
		if err != nil {
			return "", err
		}
		return strings.ToLower(strings.ReplaceAll(value, "_", "-")), nil
	})
}

// GenerateAPIVersionAndStore stores a unique semantic API version.
func GenerateAPIVersionAndStore(ctx context.Context, base, key string) error {
	return generateAndStore(ctx, key, func() (string, error) {
		value, err := unique.Unique(ctx, base)
		if err != nil {
			return "", err
		}
		checksum := sha256.Sum256([]byte(value))
		patch := binary.BigEndian.Uint64(checksum[:8])%999999 + 1
		return fmt.Sprintf("v1.0.%d", patch), nil
	})
}

// GenerateContextAndStore creates a unique API context and stores it in runner-local scope.
func GenerateContextAndStore(ctx context.Context, base, key string) error {
	return generateAndStore(ctx, key, func() (string, error) {
		return unique.UniqueContext(ctx, base)
	})
}

func generateAndStore(ctx context.Context, key string, generate func() (string, error)) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("cannot store a generated value with an empty key")
	}
	local, ok := tcontext.LocalOf(ctx)
	if !ok || local == nil {
		return fmt.Errorf("cannot store generated value %q without runner context", key)
	}
	value, err := generate()
	if err != nil {
		return err
	}
	local.Set(key, value)
	return nil
}

// StoredValue returns a generated or otherwise runner-scoped string value.
func StoredValue(ctx context.Context, key string) (string, error) {
	return tcontext.ResolveString(ctx, key)
}
