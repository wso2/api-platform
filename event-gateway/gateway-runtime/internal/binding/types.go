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

package binding

// Binding represents a configured channel with its entrypoint, endpoint, and policy bindings.
type Binding struct {
	Name       string         `yaml:"name"`
	Mode       string         `yaml:"mode"` // "websub" or "protocol-mediation"
	Context    string         `yaml:"context"`
	Version    string         `yaml:"version"`
	Vhost      string         `yaml:"vhost"`
	Entrypoint EntrypointSpec `yaml:"entrypoint"`
	Endpoint   EndpointSpec   `yaml:"endpoint"`
	Policies   PolicyBindings `yaml:"policies"`
}

// EntrypointSpec defines the entrypoint connector type and configuration.
type EntrypointSpec struct {
	Type         string `yaml:"type"` // "websub" or "websocket"
	Path         string `yaml:"path"`
	Backpressure string `yaml:"backpressure"` // "drop-oldest", "block", "close"
}

// EndpointSpec defines the endpoint connector type and configuration.
type EndpointSpec struct {
	Type     string                 `yaml:"type"` // "kafka"
	Topic    string                 `yaml:"topic"`
	Ordering string                 `yaml:"ordering"` // "ordered" or "unordered"
	Config   map[string]interface{} `yaml:"config"`   // endpoint-specific config (e.g. brokers, tls)
}

// PolicyBindings holds inbound and outbound policy configurations.
type PolicyBindings struct {
	Inbound  []PolicyRef `yaml:"inbound"`
	Outbound []PolicyRef `yaml:"outbound"`
}

// PolicyRef references a policy to include in a chain.
type PolicyRef struct {
	Name    string                 `yaml:"name"`
	Version string                 `yaml:"version"`
	Params  map[string]interface{} `yaml:"params"`
}

// ChannelsConfig is the top-level structure of the channels.yaml file.
type ChannelsConfig struct {
	Channels []Binding `yaml:"channels"`
}
