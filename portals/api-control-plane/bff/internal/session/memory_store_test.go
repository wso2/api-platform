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
	"testing"
	"time"
)

func TestMemoryStore_PutGetDelete(t *testing.T) {
	s := NewMemoryStore()
	defer s.Close()
	ctx := context.Background()

	sess := &Session{ID: "abc", Mode: ModeFileBased, AccessToken: "tok", AbsoluteExpiry: time.Now().Add(time.Hour)}
	if err := s.Put(ctx, sess); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, ok, err := s.Get(ctx, "abc")
	if err != nil || !ok {
		t.Fatalf("Get: ok=%v err=%v", ok, err)
	}
	if got.AccessToken != "tok" {
		t.Errorf("AccessToken = %q, want tok", got.AccessToken)
	}

	if err := s.Delete(ctx, "abc"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok, _ := s.Get(ctx, "abc"); ok {
		t.Error("expected session gone after Delete")
	}
}

func TestMemoryStore_ExpiredEvicted(t *testing.T) {
	s := NewMemoryStore()
	defer s.Close()
	ctx := context.Background()

	s.Put(ctx, &Session{ID: "old", AbsoluteExpiry: time.Now().Add(-time.Minute)})
	if _, ok, _ := s.Get(ctx, "old"); ok {
		t.Error("expected expired session to be evicted on Get")
	}
}

func TestMemoryStore_TouchExtends(t *testing.T) {
	s := NewMemoryStore()
	defer s.Close()
	ctx := context.Background()

	exp := time.Now().Add(time.Minute)
	s.Put(ctx, &Session{ID: "x", AbsoluteExpiry: exp})

	newExp := time.Now().Add(time.Hour)
	if err := s.Touch(ctx, "x", newExp); err != nil {
		t.Fatalf("Touch: %v", err)
	}
	got, _, _ := s.Get(ctx, "x")
	if !got.AbsoluteExpiry.Equal(newExp) {
		t.Errorf("AbsoluteExpiry = %v, want %v", got.AbsoluteExpiry, newExp)
	}

	// Touch must not shrink the expiry.
	_ = s.Touch(ctx, "x", time.Now().Add(time.Second))
	got, _, _ = s.Get(ctx, "x")
	if !got.AbsoluteExpiry.Equal(newExp) {
		t.Errorf("Touch shrank expiry to %v", got.AbsoluteExpiry)
	}
}
