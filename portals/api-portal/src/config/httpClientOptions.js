/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

'use strict';

/**
 * Shared OUTBOUND HTTP(S) client configuration for this portal's own
 * server-side calls to other services — Platform API login (authController),
 * IDP/key-manager token endpoints (tokenUtil, oauthTokenService), and webhook
 * delivery (deliveryWorker). Mirrors config/tlsOptions.js's inbound TLS
 * options — same [minimumProtocolVersion, maximumProtocolVersion, ciphers,
 * ecdhCurves] vocabulary, reused as-is via buildTLSOptions() since its output
 * shape ({ minVersion, maxVersion, ecdhCurve, ciphers? }) is exactly what
 * Node's `https.Agent` constructor accepts too — but for outbound
 * connections, plus connection pooling so repeated calls to the same host
 * reuse TCP+TLS handshakes instead of paying a fresh one per request.
 *
 * A call site that needs its own `rejectUnauthorized` value (authController's
 * auth.local.tlsSkipVerify, tryoutProxyService's tryout.tlsSkipVerify) builds
 * its own `https.Agent` by spreading `tlsOptions` from this module's return
 * value alongside that value, rather than reusing `httpsAgent` outright,
 * since an Agent's TLS options are fixed at construction time.
 */

const http = require('http');
const https = require('https');
const { buildTLSOptions } = require('./tlsOptions');

let cached = null;

/**
 * Builds (and memoizes) the shared outbound http.Agent/https.Agent pair from
 * config.httpClient. Throws on an invalid TLS field (same fail-closed
 * behavior as the inbound listener's buildTLSOptions) — callers should let
 * that abort startup rather than run with a silently-degraded TLS posture.
 */
function buildOutboundAgents(config) {
    if (cached) {
        return cached;
    }

    const httpClientCfg = config.httpClient || {};
    const tlsOptions = buildTLSOptions(httpClientCfg.tls || {});

    const pooling = {
        keepAlive: httpClientCfg.keepAlive !== false,
        maxSockets: httpClientCfg.maxSockets || 50,
        maxFreeSockets: httpClientCfg.maxFreeSockets || 10,
        timeout: httpClientCfg.timeoutMs || 10000,
    };

    cached = {
        httpAgent: new http.Agent(pooling),
        httpsAgent: new https.Agent({ ...pooling, ...tlsOptions }),
        tlsOptions,
        pooling,
    };
    return cached;
}

module.exports = { buildOutboundAgents };
