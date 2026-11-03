/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package server

import (
	"context"
	"sync"
	"time"

	"ai-workspace-bff/internal/secure"
	"ai-workspace-bff/internal/session"
)

// fakeStore is a map-backed session.Store for tests about handler behaviour — token
// exchange, org discovery, org switching — where the point is what the handler does
// with a session, not how the session is carried.
//
// A test double, deliberately: the product ships exactly one store, the cookie one,
// and this is not it. The cookie store is exercised through the real HTTP path in
// multi_replica_test.go and refresh_exchange_replica_test.go, which is where the
// carrying mechanism belongs under test. Using it here too would make every one of
// these tests set up a request and a cookie jar to assert something unrelated to
// either.
type fakeStore struct {
	mu       sync.Mutex
	sessions map[string]*session.Session
}

func newFakeStore() *fakeStore {
	return &fakeStore{sessions: make(map[string]*session.Session)}
}

func (f *fakeStore) Put(_ context.Context, s *session.Session) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *s
	f.sessions[cp.ID] = &cp
	return nil
}

func (f *fakeStore) Get(_ context.Context, id string) (*session.Session, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.sessions[id]
	if !ok {
		return nil, false, nil
	}
	if s.Expired(time.Now()) {
		delete(f.sessions, id)
		return nil, false, nil
	}
	cp := *s
	return &cp, true, nil
}

func (f *fakeStore) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	delete(f.sessions, id)
	f.mu.Unlock()
	return nil
}

func (f *fakeStore) Touch(_ context.Context, id string, extendTo time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if s, ok := f.sessions[id]; ok && extendTo.After(s.AbsoluteExpiry) {
		s.AbsoluteExpiry = extendTo
	}
	return nil
}

func (f *fakeStore) Close() error { return nil }

// testTxSealer is the login-transaction sealer tests hand to auth.NewOIDC, which
// requires one — a login transaction has nowhere else to live.
func testTxSealer() *secure.Sealer {
	s, err := secure.NewSealer(secure.DeriveKey("test-key-material", "test/tx"))
	if err != nil {
		panic(err)
	}
	return s
}
