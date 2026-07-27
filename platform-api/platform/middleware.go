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

package platform

import "github.com/wso2/api-platform/platform-api/pdk"

// Thin aliases of the pdk middleware surface so a wrapper importing only
// platform can add request-chain middleware without importing pdk
// directly. A plugin contributes middleware by implementing Middleware() and
// returning PositionedMiddleware values at one of the two allowed positions.
type (
	// Middleware wraps one handler with another.
	Middleware = pdk.Middleware
	// ChainPosition selects where a middleware sits in the chain.
	ChainPosition = pdk.ChainPosition
	// PositionedMiddleware pairs a middleware with its chain position.
	PositionedMiddleware = pdk.PositionedMiddleware
)

const (
	// BeforePlatformChain is outermost — before CORS/auth, no identity yet.
	BeforePlatformChain = pdk.BeforePlatformChain
	// AfterPlatformChain is innermost — after auth + scope enforcement.
	AfterPlatformChain = pdk.AfterPlatformChain
)
