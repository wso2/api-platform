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

package session

import (
	"context"
	"sync"
	"time"
)

// MemoryStore is the default, single-replica, in-process session store ([session]
// store = "memory"). Sessions are lost on restart (users simply re-login). It must not
// be used with more than one replica: a request routed to another instance finds no
// session, so logins fail at the IDP callback and org/exchanged-token state is lost at
// random — store = "cookie" is what a scaled-out deployment uses instead. A background
// sweeper evicts expired sessions.
type MemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	stop     chan struct{}
}

// NewMemoryStore creates a store and starts its eviction sweeper.
func NewMemoryStore() *MemoryStore {
	m := &MemoryStore{
		sessions: make(map[string]*Session),
		stop:     make(chan struct{}),
	}
	go m.sweep(5 * time.Minute)
	return m
}

func (m *MemoryStore) Put(_ context.Context, s *Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Store a copy so the store never shares the caller-owned pointer.
	cp := *s
	m.sessions[cp.ID] = &cp
	return nil
}

func (m *MemoryStore) Get(_ context.Context, id string) (*Session, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return nil, false, nil
	}
	// Evaluate expiry on the locked entry; evict and report a miss if expired.
	if s.Expired(time.Now()) {
		delete(m.sessions, id)
		return nil, false, nil
	}
	// Return a copy so callers never hold the internal pointer.
	cp := *s
	return &cp, true, nil
}

func (m *MemoryStore) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	delete(m.sessions, id)
	m.mu.Unlock()
	return nil
}

func (m *MemoryStore) Touch(_ context.Context, id string, extendTo time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[id]; ok && extendTo.After(s.AbsoluteExpiry) {
		s.AbsoluteExpiry = extendTo
	}
	return nil
}

func (m *MemoryStore) Close() error {
	close(m.stop)
	return nil
}

func (m *MemoryStore) sweep(interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-m.stop:
			return
		case now := <-t.C:
			m.mu.Lock()
			for id, s := range m.sessions {
				if s.Expired(now) {
					delete(m.sessions, id)
				}
			}
			m.mu.Unlock()
		}
	}
}
