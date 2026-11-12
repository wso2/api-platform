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

package funcs

import (
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"
)

// envNamePattern matches an allow list entry: a POSIX-style environment variable
// name, optionally followed by a single trailing "*" marking a prefix match.
var envNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*\*?$`)

// EnvAccessPolicy decides which environment variables the {{ env "NAME" }}
// template function may read while rendering an artifact spec.
//
// An unrestricted policy (UnrestrictedEnvAccessPolicy) reads any variable, which
// is the behaviour when no allow list is configured. A restricted policy reads
// only the names on its allow list. A nil *EnvAccessPolicy denies every name.
type EnvAccessPolicy struct {
	unrestricted bool
	exact        map[string]struct{}
	prefixes     []string
}

// UnrestrictedEnvAccessPolicy returns a policy that permits every variable.
func UnrestrictedEnvAccessPolicy() *EnvAccessPolicy {
	return &EnvAccessPolicy{unrestricted: true}
}

// NewEnvAccessPolicy builds a restricted policy from allow list entries. Each
// entry is an exact variable name ("BACKEND_URL") or a prefix ending in "*"
// ("BACKEND_*"). Blank entries are ignored; an empty list permits nothing.
func NewEnvAccessPolicy(entries []string) (*EnvAccessPolicy, error) {
	p := &EnvAccessPolicy{exact: make(map[string]struct{})}
	for _, raw := range entries {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}
		if !envNamePattern.MatchString(entry) {
			return nil, fmt.Errorf("invalid env allow list entry %q: must be a variable name, optionally ending in a single '*'", entry)
		}
		if prefix, isPrefix := strings.CutSuffix(entry, "*"); isPrefix {
			p.prefixes = append(p.prefixes, prefix)
			continue
		}
		p.exact[entry] = struct{}{}
	}
	return p, nil
}

// Allows reports whether name may be read.
func (p *EnvAccessPolicy) Allows(name string) bool {
	if p == nil {
		return false
	}
	if p.unrestricted {
		return true
	}
	if _, ok := p.exact[name]; ok {
		return true
	}
	for _, prefix := range p.prefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// Unrestricted reports whether the policy permits every variable.
func (p *EnvAccessPolicy) Unrestricted() bool {
	return p != nil && p.unrestricted
}

// Counts returns the number of exact and prefix entries, for startup logging
// that does not reveal the configured names.
func (p *EnvAccessPolicy) Counts() (exact, prefixes int) {
	if p == nil {
		return 0, 0
	}
	return len(p.exact), len(p.prefixes)
}

// envAccessPolicy is the process-wide policy, installed once at startup. While it
// is unset (nil), every read is denied, so a code path that renders before the
// controller has installed its policy fails closed.
var envAccessPolicy atomic.Pointer[EnvAccessPolicy]

// SetEnvAccessPolicy installs the process-wide env access policy and returns the
// previously installed one (useful for restoring it in tests).
func SetEnvAccessPolicy(p *EnvAccessPolicy) *EnvAccessPolicy {
	return envAccessPolicy.Swap(p)
}

func currentEnvAccessPolicy() *EnvAccessPolicy {
	return envAccessPolicy.Load()
}

// EnvAccessError is returned when a template references an environment variable
// that the env access policy does not permit. Its message is fixed and never
// includes the requested name or any value.
type EnvAccessError struct{}

func (e *EnvAccessError) Error() string {
	return "environment variable is not permitted in artifact templates"
}
