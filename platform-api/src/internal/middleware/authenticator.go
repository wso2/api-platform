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

import "github.com/gin-gonic/gin"

// Authenticator abstracts credential validation. Implementations populate the Gin
// context with identity values (user_id, organization, scope, platform_roles, …)
// when authentication succeeds, and write an error response then abort on failure.
//
// JWTAuthenticator is the default; add new implementations (e.g. BasicAuthAuthenticator)
// without touching server wiring.
type Authenticator interface {
	// Middleware returns the ordered Gin handler chain that validates credentials
	// and populates the request context with identity values. Multiple handlers
	// may be returned (e.g. JWKS validation followed by claims extraction).
	Middleware() []gin.HandlerFunc
}

// JWTAuthenticator wraps one or more Gin middleware functions that perform JWT
// validation and claims extraction behind the Authenticator interface.
type JWTAuthenticator struct {
	handlers []gin.HandlerFunc
}

// NewJWTAuthenticator creates an Authenticator backed by the given Gin handlers.
// Pass ThunderAuthMiddleware or the IDP auth+claims pair here.
func NewJWTAuthenticator(handlers ...gin.HandlerFunc) *JWTAuthenticator {
	return &JWTAuthenticator{handlers: handlers}
}

func (a *JWTAuthenticator) Middleware() []gin.HandlerFunc {
	return a.handlers
}
