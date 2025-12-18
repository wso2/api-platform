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
package authenticators

import (
	"errors"

	"github.com/gin-gonic/gin"
)

var (
	ErrAuthenticationFailed = errors.New("authentication failed")
	ErrNoAuthenticators     = errors.New("no authenticators registered")
)

// Credentials represents generic authentication credentials
type Credentials interface{}

// AuthResult contains the result of an authentication attempt
type AuthResult struct {
	Success      bool
	UserID       string
	Roles        []string
	Claims       map[string]interface{}
	ErrorMessage string
}

// Authenticator defines the interface for authentication methods
type Authenticator interface {
	// Authenticate verifies the provided credentials
	Authenticate(ctx *gin.Context) (*AuthResult, error)

	// Name returns the name of the authenticator
	Name() string

	// CanHandle checks if this authenticator can handle the given credentials
	CanHandle(ctx *gin.Context) bool
}
