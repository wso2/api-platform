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

package connectors

import (
	"fmt"
	"sync"
)

// ReceiverFactory creates a receiver from per-channel config.
// Receiver-specific configuration is captured in the closure at registration time.
type ReceiverFactory func(cfg ReceiverConfig) (Receiver, error)

// BrokerDriverFactory creates a broker-driver from per-channel configuration.
// The config map comes from broker-driver.config in channels.yaml.
type BrokerDriverFactory func(cfg map[string]interface{}) (BrokerDriver, error)

// Registry holds factories for creating receivers and broker-drivers by type name.
// New types are added by registering factories — no changes to the runtime or main required.
type Registry struct {
	mu            sync.RWMutex
	receivers     map[string]ReceiverFactory
	brokerDrivers map[string]BrokerDriverFactory
}

// NewRegistry creates an empty connector registry.
func NewRegistry() *Registry {
	return &Registry{
		receivers:     make(map[string]ReceiverFactory),
		brokerDrivers: make(map[string]BrokerDriverFactory),
	}
}

// RegisterReceiver registers a receiver factory by type name (e.g. "websub", "websocket").
func (r *Registry) RegisterReceiver(name string, factory ReceiverFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.receivers[name] = factory
}

// RegisterBrokerDriver registers a broker-driver factory by type name (e.g. "kafka").
func (r *Registry) RegisterBrokerDriver(name string, factory BrokerDriverFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.brokerDrivers[name] = factory
}

// CreateReceiver creates a receiver using the registered factory.
func (r *Registry) CreateReceiver(name string, cfg ReceiverConfig) (Receiver, error) {
	r.mu.RLock()
	factory, ok := r.receivers[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown receiver type: %s", name)
	}
	return factory(cfg)
}

// CreateBrokerDriver creates a broker-driver using the registered factory.
func (r *Registry) CreateBrokerDriver(name string, cfg map[string]interface{}) (BrokerDriver, error) {
	r.mu.RLock()
	factory, ok := r.brokerDrivers[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown broker-driver type: %s", name)
	}
	return factory(cfg)
}
