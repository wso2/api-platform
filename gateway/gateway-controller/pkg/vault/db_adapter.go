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

package vault

import (
	"context"
	"errors"
)

// LocalStore is the encrypted database store behind the DB adapter.
// *secrets.SecretService satisfies it; its handles, ciphertext format, key
// files and management API stay unchanged.
type LocalStore interface {
	Resolve(handle string) (string, error)
	UpsertFromPlatform(handle, displayName, plaintext string) error
}

// DBAdapter reads gateway-local secrets from the encrypted database. It is
// the default adapter and the only one the gateway writes to.
type DBAdapter struct {
	store LocalStore
}

// NewDBAdapter wraps the gateway's encrypted secret store.
func NewDBAdapter(store LocalStore) *DBAdapter {
	return &DBAdapter{store: store}
}

// Type returns "db".
func (a *DBAdapter) Type() string { return "db" }

// Get decrypts the secret stored under ref.Handle.
func (a *DBAdapter) Get(_ context.Context, ref Ref) (string, error) {
	if ref.IsExternal() {
		return "", errors.New("the DB adapter reads only local handles")
	}
	return a.store.Resolve(ref.Handle)
}

// Upsert encrypts and stores a value under handle, replacing any existing value.
func (a *DBAdapter) Upsert(handle, displayName, plaintext string) error {
	return a.store.UpsertFromPlatform(handle, displayName, plaintext)
}
