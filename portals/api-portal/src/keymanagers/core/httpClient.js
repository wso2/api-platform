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
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

'use strict';

/*
 * The outbound HTTP client every key-manager driver and authenticator uses.
 *
 * The POC this was ported from carried its own node:http(s) helper to stay
 * dependency-free. Inside the portal that would be a second, unguarded HTTP
 * stack alongside the one every other outbound call already goes through, so it
 * is replaced here by the portal's own: axios, the shared TLS tuning and
 * connection pooling from config/httpClientOptions.js, and the SSRF guard from
 * utils/ssrfGuard.js.
 *
 * Why the SSRF guard applies to an operator-configured endpoint
 * (js-ssrf-prevention.md, directive 1 — "any admin-configurable endpoint"): a
 * key manager URL is not attacker-supplied, but it is the portal dialling an
 * arbitrary host on behalf of a request, and a mistyped or copy-pasted endpoint
 * pointed at 169.254.169.254 would otherwise hand a caller the instance
 * metadata service. Link-local and cloud-metadata ranges are refused
 * unconditionally; private/loopback ranges need an explicit opt-in, because a
 * key manager legitimately lives on localhost or a cluster IP in most
 * deployments.
 *
 * One client is built per key manager, not per call: TLS trust
 * (`insecure_skip_verify`) is fixed at Agent construction, and rebuilding an
 * Agent per request would also throw away keep-alive.
 */

const http = require('http');
const https = require('https');
const axios = require('axios');

const { buildOutboundAgents } = require('../../config/httpClientOptions');
const {
    createGuardedLookup,
    assertAllowedHost,
    assertAllowedScheme,
    isDenied,
} = require('../../utils/ssrfGuard');

/*
 * configLoader is required lazily, inside the functions below, rather than at
 * module top level. config/keyManagerConfig.js is required BY configLoader
 * during its own startup validation and pulls in the authenticators, which pull
 * in this module — a top-level require here would close that cycle and see a
 * half-initialised `config`. By the time any function below runs, configLoader
 * has finished loading and the require is a cache hit.
 */
function portalConfig() {
    return require('../../config/configLoader').config;
}

function clientConfig() {
    return portalConfig().keyManagerClient || {};
}

/**
 * The per-key-manager client policy: how to reach THAT key manager.
 *
 * These three live on the [[api_portal.key_manager]] entry, not in the global
 * [api_portal.key_manager_client] section, because each describes one key
 * manager's host rather than a deployment-wide posture. A local Thunder on
 * loopback with a self-signed certificate needs all three relaxed; a public
 * key manager configured alongside it must not inherit that.
 *
 * @typedef {object} ClientPolicy
 * @property {boolean} [insecureSkipVerify]     skip TLS verification for this key manager
 * @property {boolean} [allowPrivateEndpoints]  permit a private/loopback address
 * @property {boolean} [allowHttpEndpoints]     permit plain http://
 */

/** Normalize a policy, applying the deny-by-default posture for anything unset. */
function resolvePolicy(policy) {
    const p = policy || {};
    return {
        insecureSkipVerify: p.insecureSkipVerify === true,
        // Deny by default: an operator opts a specific key manager into a
        // private address, and never gets it implicitly.
        allowPrivate: p.allowPrivateEndpoints === true,
        // Permitted by default: an identity server commonly sits behind a
        // TLS-terminating ingress, and these endpoints are operator-supplied.
        allowHttp: p.allowHttpEndpoints !== false,
    };
}

/**
 * Assert a configured key-manager URL may be dialled.
 *
 * Called at startup for every configured endpoint (so a bad one fails the boot,
 * not the first request) and again per request for IP literals, which Node
 * connects to without ever consulting the Agent's `lookup` hook.
 *
 * The policy is passed in rather than read from configLoader: this runs inside
 * configLoader's own module body, before its `module.exports` has executed, so
 * even a lazy require would see an undefined `config` here.
 *
 * @param {string} rawUrl
 * @param {string} fieldPath      config path, for the startup error message only
 * @param {ClientPolicy} policy   the key manager's own client policy
 */
function assertDialable(rawUrl, fieldPath, policy) {
    const { allowHttp, allowPrivate } = resolvePolicy(policy);
    try {
        assertAllowedScheme(rawUrl, { allowHttp });
    } catch (err) {
        const remedy = allowHttp
            ? 'http and https are permitted'
            : 'only https is permitted; set allow_http_endpoints = true on this ' +
              'key manager to allow http';
        throw new Error(`${fieldPath}: ${err.message} (${remedy})`);
    }
    // Only IP literals can be judged here: assertAllowedHost is a no-op for a
    // hostname, since Node resolves those at connection time. A hostname that
    // resolves into a blocked range is refused then, by the guarded lookup on
    // the Agent — which is also what closes the DNS-rebinding window. So this
    // catches "https://127.0.0.1:8090" at startup but not "https://localhost:8090".
    const { hostname } = new URL(rawUrl);
    try {
        assertAllowedHost(hostname, { allowPrivate });
    } catch (err) {
        // Distinguish "you could opt into this" from "never permitted": an
        // operator told to flip allow_private_endpoints for a link-local or
        // cloud-metadata address would flip it and still be refused.
        const wouldPassIfPrivateAllowed = !isDenied(stripBrackets(hostname), { allowPrivate: true });
        const remedy = wouldPassIfPrivateAllowed
            ? 'Set allow_private_endpoints = true on this key manager to permit a ' +
              'private/loopback address.'
            : 'Link-local and cloud-metadata addresses are never permitted, ' +
              'whatever this key manager is configured with.';
        throw new Error(
            `${fieldPath}: ${err.message} — "${hostname}" is in a blocked address range. ${remedy}`
        );
    }
}

function stripBrackets(hostname) {
    // URL.hostname keeps the brackets on an IPv6 literal ("[::1]").
    return String(hostname || '').replace(/^\[|\]$/g, '');
}

/**
 * Build the axios instance for one key manager.
 *
 * Address/scheme/TLS policy comes from the key manager's own entry; the
 * time and byte bounds come from the global [api_portal.key_manager_client]
 * section, since those are deployment-wide resource limits rather than
 * statements about one host.
 *
 * @param {ClientPolicy & { clientCert?: object }} [policy]
 * @returns {import('axios').AxiosInstance}
 */
function buildClient(policy = {}) {
    const { insecureSkipVerify, allowPrivate } = resolvePolicy(policy);
    const clientCert = policy.clientCert || null;
    const cfg = clientConfig();
    const lookup = createGuardedLookup({ allowPrivate });
    // Same cipher/curve/version tuning as every other outbound client in this
    // portal; rejectUnauthorized and the guarded lookup are specific to this
    // client, so it builds its own Agent rather than reusing the shared pair.
    const { tlsOptions, pooling } = buildOutboundAgents(portalConfig());

    return axios.create({
        timeout: cfg.timeoutMs || 10000,
        // A 3xx must not walk the request to a host that never passed the checks
        // above; redirects are surfaced as the response instead of followed.
        maxRedirects: 0,
        maxContentLength: cfg.maxResponseBytes || 1048576,
        maxBodyLength: cfg.maxRequestBytes || 1048576,
        // A key manager's 4xx is a meaningful answer (invalid_redirect_uri and
        // friends), not a transport failure — the driver inspects the status.
        validateStatus: () => true,
        httpAgent: new http.Agent({ ...pooling, lookup }),
        httpsAgent: new https.Agent({
            ...pooling,
            ...tlsOptions,
            lookup,
            rejectUnauthorized: !insecureSkipVerify,
            // mTLS: this Agent is pinned to one client identity. Spread last so
            // the cert material cannot be clobbered by the shared tlsOptions.
            ...(clientCert || {}),
        }),
    });
}

module.exports = { buildClient, assertDialable };
