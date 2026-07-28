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

// Thin aliases of the pdk route-override surface, so a wrapper importing only
// platform can decorate an existing core route without importing pdk directly —
// the same treatment the middleware surface gets in middleware.go.
//
// A plugin decorates a route by implementing RouteOverrides() and returning a
// RouteOverride per core pattern it claims. See the doc comment on
// pdk.RouteOverride for the auth-sensitive constraints that apply.
type (
	// RouteOverride declares that a plugin decorates one existing core route.
	RouteOverride = pdk.RouteOverride
	// RouteOverrideProvider is the optional interface a Plugin implements to
	// declare route overrides.
	RouteOverrideProvider = pdk.RouteOverrideProvider
	// CapturedResponse is what a core handler wrote, as captured by Invoke.
	CapturedResponse = pdk.CapturedResponse
)

var (
	// Invoke runs the original core handler and captures its response instead
	// of sending it, so a decorator can reshape it.
	Invoke = pdk.Invoke
	// WriteCaptured sends a captured response to the client unchanged.
	WriteCaptured = pdk.WriteCaptured
)
