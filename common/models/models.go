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
package models

// AuthConfig holds configuration for the authentication middleware
type AuthConfig struct {
	// Basic Auth Configuration
	BasicAuth *BasicAuth

	// JWT/Bearer Auth Configuration
	JWTConfig *IDPConfig

	// Paths to skip authentication
	SkipPaths []string

	// Allow either basic or bearer (if true), require both (if false and both configured)
	AllowEither bool

	// ResourceRoles holds the mapping of resource -> allowed local roles.
	// Keys may be either "METHOD /path" (preferred) or just "/path".
	ResourceRoles map[string][]string `json:"resource_roles"`
}

type BasicAuth struct {
	Enabled bool   `json:"enabled"`
	Users   []User `json:"users"`
}

// User represents a user in the system
type User struct {
	Username       string   `json:"username"`
	Password       string   `json:"password"`
	PasswordHashed bool     `json:"password_hashed"`
	Roles          []string `json:"roles"`
}

// IDPConfig holds identity provider configuration
type IDPConfig struct {
	Enabled           bool                 `json:"enabled"`
	IssuerURL         string               `json:"issuer_url"`
	JWKSUrl           string               `json:"jwks_url"`
	ScopeClaim        string               `json:"scope_claim"`
	UsernameClaim     string               `json:"username_claim"`
	Audience          *string              `json:"audience"`
	Certificate       *string              `json:"certificate"`
	ClaimMapping      *map[string]string   `json:"claim_mapping"`
	PermissionMapping *map[string][]string `json:"permission_mapping"`
}
