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

// Package testbench hosts the mock services used by the integration suites.
package testbench

import (
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"sync"
)

// Service is a mock service hosted by the testbench.
type Service interface {
	// Name identifies the service.
	Name() string

	// Port returns the container port on which the service listens.
	Port() int

	// Handler returns the service HTTP handler.
	Handler() http.Handler

	// Stateful reports whether the service retains state between requests.
	Stateful() bool
}

// PartitionByBlock identifies the framework-guaranteed block partition.
const PartitionByBlock = "block"

// Partitioned identifies a stateful service whose state is isolated by a framework-guaranteed key.
type Partitioned interface {
	// PartitionKey returns the name of the partition guarantee.
	PartitionKey() string
}

// Registry is the single table of hosted services.
type Registry struct {
	mu       sync.RWMutex
	services []Service
}

// Register adds a service, rejecting anything that would make the testbench unsafe to share.
func (r *Registry) Register(s Service) error {
	if isNil(s) {
		return fmt.Errorf("testbench: nil service")
	}
	name := s.Name()
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("testbench: service has no name")
	}
	port := s.Port()
	if port <= 0 {
		return fmt.Errorf("testbench: service %q has no port", name)
	}
	if isNil(s.Handler()) {
		return fmt.Errorf("testbench: service %q has no handler", name)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.services {
		if existing.Port() == port {
			return fmt.Errorf("testbench: services %q and %q both claim port %d",
				existing.Name(), name, port)
		}
		if existing.Name() == name {
			return fmt.Errorf("testbench: duplicate service name %q", name)
		}
	}

	if s.Stateful() {
		partitioned, ok := s.(Partitioned)
		if !ok {
			return fmt.Errorf("testbench: service %q declares itself stateful and cannot be "+
				"hosted in the shared testbench — give it its own component, make it derive its "+
				"response from the request, or partition its state by implementing "+
				"testbench.Partitioned", s.Name())
		}
		if key := partitioned.PartitionKey(); key != PartitionByBlock {
			return fmt.Errorf("testbench: stateful service %q partitions by %q, which the "+
				"framework does not guarantee is unique per block; the only guaranteed key is %q",
				name, key, PartitionByBlock)
		}
	}
	r.services = append(r.services, s)
	return nil
}

// Services returns the registered services ordered by port, for deterministic startup logs.
func (r *Registry) Services() []Service {
	r.mu.RLock()
	out := append([]Service(nil), r.services...)
	r.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Port() < out[j].Port() })
	return out
}

func isNil(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}
