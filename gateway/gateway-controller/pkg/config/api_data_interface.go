/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package config

import (
	"fmt"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
)

// APIData provides a common interface to access name, context and version
// across REST (APIConfigData) and Async (WebhookAPIData) configurations.
type APIData interface {
	GetName() string
	GetContext() string
	GetVersion() string
}

// RestData wraps api.APIConfigData and implements APIData.
type RestData struct {
	api.APIConfigData
}

func (r RestData) GetName() string    { return r.Name }
func (r RestData) GetContext() string { return r.Context }
func (r RestData) GetVersion() string { return r.Version }

// WebSubData wraps api.WebhookAPIData and implements APIData.
type WebSubData struct {
	api.WebhookAPIData
}

func (w WebSubData) GetName() string    { return w.Name }
func (w WebSubData) GetContext() string { return w.Context }
func (w WebSubData) GetVersion() string { return w.Version }

// APIDataFactory builds APIData from the union-typed APIConfiguration.
type APIDataFactory struct{}

// NewAPIDataFactory returns a new factory.
func NewAPIDataFactory() *APIDataFactory { return &APIDataFactory{} }

// FromConfiguration extracts the typed data from APIConfiguration and wraps it
// with a concrete APIData implementation.
func (f *APIDataFactory) FromConfiguration(cfg *api.APIConfiguration) (APIData, error) {
	switch cfg.Kind {
	case api.APIConfigurationKindHttprest:
		rest, err := cfg.Data.AsAPIConfigData()
		if err != nil {
			return nil, fmt.Errorf("failed to parse REST data: %w", err)
		}
		return RestData{APIConfigData: rest}, nil
	case api.APIConfigurationKindAsyncwebsub, api.APIConfigurationKindAsyncwebsocket, api.APIConfigurationKindAsyncsse:
		ws, err := cfg.Data.AsWebhookAPIData()
		if err != nil {
			return nil, fmt.Errorf("failed to parse async data: %w", err)
		}
		return WebSubData{WebhookAPIData: ws}, nil
	default:
		return nil, fmt.Errorf("unsupported API kind: %s", cfg.Kind)
	}
}

// NameContextVersion is a convenience helper to return (name, context, version)
// directly from a configuration.
func (f *APIDataFactory) NameContextVersion(cfg *api.APIConfiguration) (string, string, string, error) {
	d, err := f.FromConfiguration(cfg)
	if err != nil {
		return "", "", "", err
	}
	return d.GetName(), d.GetContext(), d.GetVersion(), nil
}
