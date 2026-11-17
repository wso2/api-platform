/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package starter makes sure the shared organisation holds what a cloud suite run needs before
// any scenario starts.
//
// The finalizer package removes what a run left behind; a Starter is its counterpart at the
// other end of the run. It holds the prerequisites of an organisation, such as the gateway the
// scenarios deploy to, and brings each of them into existence when it is missing. A prerequisite
// outlives the run: it carries no runner suffix, so the finalizer never sweeps it. The cloud
// suite's step packages supply the Prerequisites of the platform they drive.
package starter

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
)

// Prerequisite is one thing an organisation must hold before the scenarios run.
type Prerequisite struct {
	// Name identifies the prerequisite in diagnostics.
	Name string
	// Ensure brings the prerequisite into existence. It does nothing when the organisation
	// already holds it, and returns once it is ready to use.
	Ensure func(ctx context.Context) error
}

// Starter brings the prerequisites of each organisation into existence once, however many runners
// ask for it at the same time.
type Starter struct {
	mu      sync.Mutex
	outcome map[string]*outcome
	log     *slog.Logger
}

// outcome is the shared result of preparing one organisation.
type outcome struct {
	ready chan struct{}
	err   error
}

// New returns a Starter that has prepared no organisation yet. A nil logger selects the default
// logger.
func New(log *slog.Logger) *Starter {
	if log == nil {
		log = slog.Default()
	}
	return &Starter{outcome: map[string]*outcome{}, log: log}
}

// Ensure makes sure the organisation named by key holds every prerequisite. The first caller for a
// key runs them in order and stops at the first failure; every other caller for that key waits for
// that run and receives its result, so a failed preparation is reported to every runner instead of
// being tried again by each. Callers that give up waiting return the context's error and leave the
// preparation running.
func (s *Starter) Ensure(ctx context.Context, key string, prerequisites []Prerequisite) error {
	if key == "" {
		return errors.New("starter: an organisation key is required")
	}
	for _, prerequisite := range prerequisites {
		if prerequisite.Ensure == nil {
			return fmt.Errorf("starter: prerequisite %q needs an ensure function", prerequisite.Name)
		}
	}

	s.mu.Lock()
	current, running := s.outcome[key]
	if !running {
		current = &outcome{ready: make(chan struct{})}
		s.outcome[key] = current
	}
	s.mu.Unlock()

	if running {
		select {
		case <-current.ready:
			return current.err
		case <-ctx.Done():
			return fmt.Errorf("waiting for the organisation's prerequisites: %w", ctx.Err())
		}
	}

	defer close(current.ready)
	for _, prerequisite := range prerequisites {
		s.log.Info("ensuring a cloud run prerequisite", "prerequisite", prerequisite.Name)
		if err := prerequisite.Ensure(ctx); err != nil {
			current.err = fmt.Errorf("prerequisite %q: %w", prerequisite.Name, err)
			return current.err
		}
	}
	return nil
}
