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
	// abandoned is set when the preparation failed because its caller's context ended. Such an
	// outcome is dropped, and the callers that waited for it prepare the organisation again.
	abandoned bool
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
// being tried again by each. A run that failed only because its own caller's context ended is not
// kept: callers that waited for it, and later ones, prepare the organisation again under their own
// context. Callers that give up waiting return the context's error and leave the preparation
// running.
func (s *Starter) Ensure(ctx context.Context, key string, prerequisites []Prerequisite) error {
	if key == "" {
		return errors.New("starter: an organisation key is required")
	}
	for _, prerequisite := range prerequisites {
		if prerequisite.Ensure == nil {
			return fmt.Errorf("starter: prerequisite %q needs an ensure function", prerequisite.Name)
		}
	}

	for {
		s.mu.Lock()
		current, running := s.outcome[key]
		if !running {
			current = &outcome{ready: make(chan struct{})}
			s.outcome[key] = current
		}
		s.mu.Unlock()

		if !running {
			return s.prepare(ctx, key, current, prerequisites)
		}
		select {
		case <-current.ready:
			if current.abandoned && ctx.Err() == nil {
				continue
			}
			return current.err
		case <-ctx.Done():
			return fmt.Errorf("waiting for the organisation's prerequisites: %w", ctx.Err())
		}
	}
}

// prepare runs the prerequisites for the first caller of a key and publishes the result.
func (s *Starter) prepare(ctx context.Context, key string, current *outcome, prerequisites []Prerequisite) error {
	defer close(current.ready)
	for _, prerequisite := range prerequisites {
		s.log.Info("ensuring a cloud run prerequisite", "prerequisite", prerequisite.Name)
		if err := prerequisite.Ensure(ctx); err != nil {
			current.err = fmt.Errorf("prerequisite %q: %w", prerequisite.Name, err)
			if ctx.Err() != nil && errors.Is(err, ctx.Err()) {
				current.abandoned = true
				s.mu.Lock()
				if s.outcome[key] == current {
					delete(s.outcome, key)
				}
				s.mu.Unlock()
			}
			return current.err
		}
	}
	return nil
}
