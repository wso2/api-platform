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

package model

import (
	"time"
)

// GraphQLAPI represents a GraphQL API artifact entity. GraphQL is a core
// artifact kind (like RestApi/LlmProvider/LlmProxy/Mcp), so this type lives
// directly in the core model package.
type GraphQLAPI struct {
	ID              string           `json:"id" db:"uuid"`
	Handle          string           `json:"handle" db:"handle"`
	Name            string           `json:"displayName" db:"display_name"`
	Kind            string           `json:"kind" db:"kind"`
	Description     string           `json:"description,omitempty" db:"description"`
	Version         string           `json:"version" db:"version"`
	CreatedBy       string           `json:"createdBy,omitempty" db:"created_by"`
	UpdatedBy       string           `json:"updatedBy,omitempty" db:"updated_by"`
	ProjectID       string           `json:"projectId" db:"project_uuid"`
	OrganizationID  string           `json:"organizationId" db:"organization_uuid"`
	CreatedAt       time.Time        `json:"createdAt,omitempty" db:"created_at"`
	UpdatedAt       time.Time        `json:"updatedAt,omitempty" db:"updated_at"`
	Configuration   GraphQLAPIConfig `json:"configuration" db:"-"`
	Origin          string           `json:"origin,omitempty" db:"origin"`
	DataVersion     string           `json:"dataVersion,omitempty" db:"data_version"`
}

// GraphQLAPIConfig holds the GraphQL API configuration stored as JSON in the
// DB. Deliberately absent compared to RestAPIConfig: Transport and
// Operations — a GraphQL API has exactly one logical endpoint, not a
// per-resource/per-verb operation list.
type GraphQLAPIConfig struct {
	Name    string  `json:"name,omitempty"`
	Version string  `json:"version,omitempty"`
	Context *string `json:"context,omitempty"` // e.g. "/countries/$version" — same $version substitution as REST

	// SDL is the GraphQL schema, always stored resolved — never a
	// document-supplied schemaLocation (xxe-xml-processing.md §3 applies by
	// analogy: the server never auto-dereferences a secondary location).
	SDL string `json:"sdl"`

	// IntrospectionMode records how SDL was obtained: "SDL" (supplied
	// directly) or "ENDPOINT" (derived by introspecting upstream.main.url at
	// creation/update time). Informational only; storage is identical either way.
	IntrospectionMode string `json:"introspectionMode,omitempty"`

	// Upstream is reused as-is from model/upstream.go — a GraphQL API has a
	// single endpoint (no per-operation paths), so upstream.main is the one
	// GraphQL endpoint.
	Upstream          UpstreamConfig `json:"upstream,omitempty"`
	Policies          []Policy       `json:"policies,omitempty"`
	SubscriptionPlans []string       `json:"subscriptionPlans,omitempty"`
}
