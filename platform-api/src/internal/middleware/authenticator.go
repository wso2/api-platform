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

import "net/http"

// Authenticator abstracts credential validation. Implementations populate the request
// context with identity values (user_id, organization, scope, platform_roles, …)
// when authentication succeeds, and write an error response on failure.
type Authenticator interface {
	// Middleware returns the ordered handler chain that validates credentials
	// and populates the request context with identity values.
	Middleware() []func(http.Handler) http.Handler
}

// JWTAuthenticator wraps one or more middleware functions that perform JWT
// validation and claims extraction behind the Authenticator interface.
type JWTAuthenticator struct {
	handlers []func(http.Handler) http.Handler
}

// NewJWTAuthenticator creates an Authenticator backed by the given middleware handlers.
func NewJWTAuthenticator(handlers ...func(http.Handler) http.Handler) *JWTAuthenticator {
	return &JWTAuthenticator{handlers: handlers}
}

func (a *JWTAuthenticator) Middleware() []func(http.Handler) http.Handler {
	return a.handlers
}
