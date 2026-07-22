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

package server

import (
	"context"
	"net/http"

	"github.com/wso2/api-platform/platform-api/internal/plugin"
	"github.com/wso2/api-platform/platform-api/pdk"
)

// externalPlugin adapts an external pdk.Plugin so it looks like an internal
// plugin.Plugin. This lets the server run both tiers through a single startup and
// shutdown loop — the only difference is the Deps each tier receives: internal
// plugins get the full plugin.Deps (raw repos/services), external plugins get the
// capabilities-only pdk.Deps built once by the server.
type externalPlugin struct {
	p       pdk.Plugin
	pdkDeps *pdk.Deps // built once by the server from the internal services
}

// compile-time check that the wrapper satisfies the internal plugin contract.
var _ plugin.Plugin = (*externalPlugin)(nil)

func (e *externalPlugin) Name() string { return e.p.Name() }

// Init discards the internal plugin.Deps and hands the external plugin the
// pdk.Deps instead — external plugins only ever see capabilities, never raw
// repositories.
func (e *externalPlugin) Init(*plugin.Deps) error { return e.p.Init(e.pdkDeps) }

func (e *externalPlugin) RegisterRoutes(mux *http.ServeMux) { e.p.RegisterRoutes(mux) }

func (e *externalPlugin) OpenAPISpec() []byte { return e.p.OpenAPISpec() }

func (e *externalPlugin) Shutdown(ctx context.Context) error { return e.p.Shutdown(ctx) }

// AuthSkipPaths forwards the optional pdk.AuthSkipPathProvider so external plugins
// declare public paths through the same server hook as internal ones. It returns
// nil when the wrapped plugin does not implement the interface, making the
// server's plugin.AuthSkipPathProvider assertion a harmless no-op in that case.
func (e *externalPlugin) AuthSkipPaths() []string {
	if sp, ok := e.p.(pdk.AuthSkipPathProvider); ok {
		return sp.AuthSkipPaths()
	}
	return nil
}

// Middleware forwards the optional pdk.MiddlewareProvider so external plugins
// contribute chain middleware through the same server hook as internal ones. It
// returns nil when the wrapped plugin does not implement the interface, making the
// server's plugin.MiddlewareProvider assertion a harmless no-op in that case.
func (e *externalPlugin) Middleware() []pdk.PositionedMiddleware {
	if mp, ok := e.p.(pdk.MiddlewareProvider); ok {
		return mp.Middleware()
	}
	return nil
}
