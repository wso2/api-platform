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

package middleware

import "platform-api/src/config"

// OrgIDPRegistry maps org handles and JWT issuers to per-org IDP configurations.
type OrgIDPRegistry interface {
	GetByOrgHandle(handle string) (*config.OrgIDPConfig, bool)
	GetByIssuer(issuer string) (*config.OrgIDPConfig, bool)
}

// FileOrgIDPRegistry implements OrgIDPRegistry from a YAML-loaded slice of OrgIDPConfig.
type FileOrgIDPRegistry struct {
	byHandle map[string]*config.OrgIDPConfig
	byIssuer map[string]*config.OrgIDPConfig
}

// NewFileOrgIDPRegistry builds an OrgIDPRegistry from a slice of per-org IDP configs.
// Entries are indexed by OrgHandle and by Issuer (when non-empty).
func NewFileOrgIDPRegistry(configs []config.OrgIDPConfig) *FileOrgIDPRegistry {
	byHandle := make(map[string]*config.OrgIDPConfig, len(configs))
	byIssuer := make(map[string]*config.OrgIDPConfig)
	for i := range configs {
		c := &configs[i]
		byHandle[c.OrgHandle] = c
		if c.Issuer != "" {
			byIssuer[c.Issuer] = c
		}
	}
	return &FileOrgIDPRegistry{byHandle: byHandle, byIssuer: byIssuer}
}

func (r *FileOrgIDPRegistry) GetByOrgHandle(handle string) (*config.OrgIDPConfig, bool) {
	c, ok := r.byHandle[handle]
	return c, ok
}

func (r *FileOrgIDPRegistry) GetByIssuer(issuer string) (*config.OrgIDPConfig, bool) {
	c, ok := r.byIssuer[issuer]
	return c, ok
}
