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

package components

import (
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
)

// AliasFor returns the network alias for a component instance.
func AliasFor(d *Definition, ordinal, replicas int) (string, error) {
	if d == nil {
		return "", fmt.Errorf("instance: definition is required")
	}
	if replicas <= 1 {
		if ordinal != 0 {
			return "", fmt.Errorf("%s: ordinal %d out of range for one replica", d, ordinal)
		}
		return d.Alias, nil
	}
	if d.AliasIsFixed {
		return "", fmt.Errorf("%s: alias %q is fixed and cannot be suffixed, so replicas must be 1 (got %d)",
			d, d.Alias, replicas)
	}
	if ordinal < 0 || ordinal >= replicas {
		return "", fmt.Errorf("%s: ordinal %d out of range for %d replicas", d, ordinal, replicas)
	}
	return fmt.Sprintf("%s-%d", d.Alias, ordinal+1), nil
}

// Instance represents one running component instance.
type Instance struct {
	def      *Definition
	ordinal  int
	replicas int
	alias    string

	// host is the address used to reach mapped ports.
	host string

	// mapped is canonical in-container port -> host port.
	mapped map[int]int
}

// NewInstance creates an instance from resolved runtime values.
func NewInstance(def *Definition, ordinal, replicas int, host string, mapped map[int]int) (*Instance, error) {
	if def == nil {
		return nil, fmt.Errorf("instance: definition is required")
	}
	if replicas <= 0 {
		replicas = 1
	}
	alias, err := AliasFor(def, ordinal, replicas)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(host) == "" {
		return nil, fmt.Errorf("%s: instance host is required", def)
	}

	copied := make(map[int]int, len(mapped))
	for k, v := range mapped {
		copied[k] = v
	}

	return &Instance{
		def: def, ordinal: ordinal, replicas: replicas,
		alias: alias, host: host, mapped: copied,
	}, nil
}

// RefreshPorts replaces the instance's mapped host ports.
func (i *Instance) RefreshPorts(mapped map[int]int) {
	next := make(map[int]int, len(mapped))
	for k, v := range mapped {
		next[k] = v
	}
	i.mapped = next
}

// Definition returns the instance definition.
func (i *Instance) Definition() *Definition { return i.def }

// Name is the component name, shared by every replica.
func (i *Instance) Name() string { return i.def.Name }

// Alias is this instance's DNS name on its block's network.
func (i *Instance) Alias() string { return i.alias }

// Ordinal is the 0-based replica index.
func (i *Instance) Ordinal() int { return i.ordinal }

// Label identifies this instance in messages, disambiguating replicas.
func (i *Instance) Label() string {
	if i.replicas <= 1 {
		return i.def.Name
	}
	return fmt.Sprintf("%s#%d", i.def.Name, i.ordinal+1)
}

// Host returns the address used to reach mapped ports.
func (i *Instance) Host() string { return i.host }

// MappedPort returns the mapped host port for an endpoint.
func (i *Instance) MappedPort(endpoint string) (int, error) {
	e, ok := i.def.Endpoint(endpoint)
	if !ok {
		return 0, i.unknownEndpoint(endpoint)
	}
	port, ok := i.mapped[e.Port]
	if !ok {
		return 0, fmt.Errorf("%s: endpoint %q (container port %d) has no mapped host port; "+
			"the container may not be running or the port was not published",
			i.Label(), endpoint, e.Port)
	}
	return port, nil
}

// URL returns the external URL for an endpoint.
func (i *Instance) URL(endpoint string) (string, error) {
	e, ok := i.def.Endpoint(endpoint)
	if !ok {
		return "", i.unknownEndpoint(endpoint)
	}
	port, err := i.MappedPort(endpoint)
	if err != nil {
		return "", err
	}
	return buildURL(e.Scheme, i.host, port, e.PathPrefix), nil
}

// InternalURL returns the network URL for an endpoint.
func (i *Instance) InternalURL(endpoint string) (string, error) {
	e, ok := i.def.Endpoint(endpoint)
	if !ok {
		return "", i.unknownEndpoint(endpoint)
	}
	return buildURL(e.Scheme, i.alias, e.Port, e.PathPrefix), nil
}

func (i *Instance) unknownEndpoint(name string) error {
	available := make([]string, 0, len(i.def.Endpoints))
	for _, e := range i.def.Endpoints {
		available = append(available, e.Name)
	}
	sort.Strings(available)
	return fmt.Errorf("%s: no endpoint %q (available: %s)", i.Label(), name, strings.Join(available, ", "))
}

func buildURL(scheme, host string, port int, pathPrefix string) string {
	hostPort := net.JoinHostPort(host, strconv.Itoa(port))
	if scheme == "" {
		return hostPort + pathPrefix
	}
	return scheme + "://" + hostPort + pathPrefix
}

// Set stores component instances by name.
type Set struct {
	byName map[string][]*Instance
}

// NewSet returns an empty set.
func NewSet() *Set { return &Set{byName: make(map[string][]*Instance)} }

// Add adds an instance to the set.
func (s *Set) Add(inst *Instance) error {
	if inst == nil {
		return fmt.Errorf("set: cannot add a nil instance")
	}
	if s.byName == nil {
		s.byName = make(map[string][]*Instance)
	}
	existing := s.byName[inst.Name()]
	for _, e := range existing {
		if e.Ordinal() == inst.Ordinal() {
			return fmt.Errorf("set: %s is already present", inst.Label())
		}
	}
	existing = append(existing, inst)
	sort.Slice(existing, func(a, b int) bool { return existing[a].Ordinal() < existing[b].Ordinal() })
	s.byName[inst.Name()] = existing
	return nil
}

// Get returns the only instance with the given component name.
func (s *Set) Get(name string) (*Instance, error) {
	instances, ok := s.byName[name]
	if !ok || len(instances) == 0 {
		return nil, s.unknownComponent(name)
	}
	if len(instances) > 1 {
		return nil, fmt.Errorf("component %q has %d replicas in this block, so it must be addressed by ordinal",
			name, len(instances))
	}
	return instances[0], nil
}

// At returns a component replica by zero-based ordinal.
func (s *Set) At(name string, ordinal int) (*Instance, error) {
	instances, ok := s.byName[name]
	if !ok || len(instances) == 0 {
		return nil, s.unknownComponent(name)
	}
	for _, inst := range instances {
		if inst.Ordinal() == ordinal {
			return inst, nil
		}
	}
	if ordinal < 0 || ordinal >= len(instances) {
		return nil, fmt.Errorf("component %q has %d instance(s) in this block; ordinal %d is out of range",
			name, len(instances), ordinal)
	}
	return nil, fmt.Errorf("component %q has no instance with ordinal %d", name, ordinal)
}

// All returns all replicas of a component in ordinal order.
func (s *Set) All(name string) []*Instance {
	return append([]*Instance(nil), s.byName[name]...)
}

// Names returns component names in sorted order.
func (s *Set) Names() []string {
	names := make([]string, 0, len(s.byName))
	for n := range s.byName {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Len returns the total number of instances.
func (s *Set) Len() int {
	n := 0
	for _, instances := range s.byName {
		n += len(instances)
	}
	return n
}

func (s *Set) unknownComponent(name string) error {
	return fmt.Errorf("no component %q in this block (present: %s)", name, strings.Join(s.Names(), ", "))
}
