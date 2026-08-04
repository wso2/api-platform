/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

/*
 * URL path prefixes this app is wired with.
 *
 * None of these are configuration: each is a fixed contract with the BFF that serves
 * this SPA, so they live here as constants rather than in config.env.ts among the
 * runtime-configurable values. They mirror the BFF's own constants one-for-one
 * (bff/internal/paths/paths.go) and must be changed together with them.
 */

/**
 * URL path prefix this app is served under, with no trailing slash.
 *
 * Single source of truth for the frontend: `vite.config.ts` imports it for Vite's
 * `base`, so the prefix baked into the bundle and the prefix the code composes URLs
 * from cannot disagree. It has to be baked in at all because index.html references its
 * assets by absolute path — a bundle built for one prefix and served under another
 * 404s on every asset. Matches the BFF's `paths.Base`, which mounts every server route.
 *
 * Router paths do NOT need it — BrowserRouter is mounted with basename={BASE_PATH}, so
 * navigate()/<Link to> are already relative to it. It IS needed for anything the router
 * doesn't own: absolute fetch() paths to the BFF, window.location assignments, and OIDC
 * redirect URIs.
 */
export const BASE_PATH = '/ai-workspace';

/**
 * Same-origin prefix every Platform API call goes through: the BFF reverse proxy
 * (<base>/proxy/* → Platform API), so the browser only ever talks to the app origin,
 * never holds a token, and never sees the platform-api self-signed certificate. The
 * BFF strips exactly this prefix before forwarding, so it matches its `paths.Proxy`.
 */
const PROXY_BASE_URL = `${BASE_PATH}/proxy`;

/**
 * Platform API base URL — the proxy base plus the API's own versioned prefix. The
 * version is what this SPA is written against, so reaching a different one is a code
 * change, not a config value. Matches the BFF's `paths.PlatformAPI`.
 */
export const PLATFORM_API_BASE_URL = `${PROXY_BASE_URL}/api/v0.9`;

/**
 * Portal API base URL — the Platform API's portal routes, through the same proxy.
 * Matches the BFF's `paths.PortalAPI`.
 */
export const PORTAL_API_BASE_URL = `${PROXY_BASE_URL}/api/portal/v0.9`;

/**
 * Base URL for the BFF's own API — the routes it answers itself instead of forwarding:
 * session/login/logout, and the handful of creates that span two Platform API calls
 * (secret + resource) and need server-side compensation when the second fails. Callers
 * use it exactly like PLATFORM_API_BASE_URL; which of the two a resource lives under is
 * the only thing that says who handles the request.
 */
export const BFF_API_BASE_URL = `${BASE_PATH}/api`;
