/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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

package policyv1alpha2

import (
	"context"
	"testing"
)

// fakeUpstreamAttemptPolicy proves any type implementing UpstreamAttemptPolicy
// compiles against the real context.Context/UpstreamAttemptContext/
// UpstreamAttemptAction types — a compile-time contract test. oauth2-generator's
// own tests (Task 9) cover real refresh behavior.
type fakeUpstreamAttemptPolicy struct{}

func (fakeUpstreamAttemptPolicy) OnUpstreamAttemptRequestHeaders(_ context.Context, actx *UpstreamAttemptContext) UpstreamAttemptAction {
	if actx.AttemptCount <= 1 {
		return UpstreamAttemptHeaderModifications{}
	}
	return UpstreamAttemptHeaderModifications{HeadersToSet: map[string]string{"Authorization": "Bearer refreshed"}}
}

func TestUpstreamAttemptContext_AttemptCountGatesRefresh(t *testing.T) {
	var p UpstreamAttemptPolicy = fakeUpstreamAttemptPolicy{}

	attemptOne := &UpstreamAttemptContext{AttemptCount: 1, Headers: NewHeaders(nil)}
	action := p.OnUpstreamAttemptRequestHeaders(context.Background(), attemptOne)
	mods, ok := action.(UpstreamAttemptHeaderModifications)
	if !ok || len(mods.HeadersToSet) != 0 {
		t.Fatalf("attempt 1 must not mutate headers, got %#v", action)
	}

	attemptTwo := &UpstreamAttemptContext{AttemptCount: 2, Headers: NewHeaders(nil)}
	action2 := p.OnUpstreamAttemptRequestHeaders(context.Background(), attemptTwo)
	mods2, ok := action2.(UpstreamAttemptHeaderModifications)
	if !ok || mods2.HeadersToSet["Authorization"] != "Bearer refreshed" {
		t.Fatalf("attempt 2 must carry the refreshed token, got %#v", action2)
	}
}

// Compile-time interface satisfaction check, mirroring action.go's own convention.
var _ UpstreamAttemptAction = UpstreamAttemptHeaderModifications{}
