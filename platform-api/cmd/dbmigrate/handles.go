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

package main

import (
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// handleGen produces v2 handles deterministically across runs (§B): generate once
// via the product's utils.GenerateHandle, persist to the file checkpoint, replay
// verbatim on resume. Uniqueness is scoped per (table, org); event tables get
// their own scope, so a websub handle and a rest_api handle may coincide.
type handleGen struct {
	run  *Run
	used map[string]map[string]bool // scopeKey(table,org) -> set of used handles
}

func newHandleGen(run *Run) *handleGen {
	return &handleGen{run: run, used: map[string]map[string]bool{}}
}

func scopeKey(table, org string) string { return table + "\x00" + org }

func (h *handleGen) set(table, org string) map[string]bool {
	k := scopeKey(table, org)
	s := h.used[k]
	if s == nil {
		s = map[string]bool{}
		h.used[k] = s
	}
	return s
}

// seed records a handle already present in v2 (for the resume case) so freshly
// generated handles avoid colliding with it.
func (h *handleGen) seed(table, org, handle string) {
	h.set(table, org)[handle] = true
}

// generate returns the v2 handle for (table, org, v1uuid) built from source.
//
//   - If a handle was already recorded for (table, v1uuid), reuse it VERBATIM —
//     no GenerateHandle, no existsCheck (on resume the value may already be in v2;
//     re-checking would falsely see "exists" and regenerate).
//   - Otherwise call utils.GenerateHandle(source, existsCheck) exactly once, record
//     it into the checkpoint, and mark it used in the (table, org) scope.
func (h *handleGen) generate(table, org, v1uuid, source string) (string, error) {
	if hv, ok := h.run.getHandle(table, v1uuid); ok {
		h.set(table, org)[hv] = true
		return hv, nil
	}
	scope := h.set(table, org)
	existsCheck := func(cand string) bool { return scope[cand] }
	hv, err := utils.GenerateHandle(source, existsCheck)
	if err != nil {
		return "", err
	}
	scope[hv] = true
	h.run.putHandle(table, v1uuid, hv)
	return hv, nil
}
