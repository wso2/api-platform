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
 * Moesif's management API, called with the billing configuration's management API key. The `~` in each
 * path is the Moesif organization the key belongs to.
 *
 * Only the calls the portal makes are here.
 */

const { billingConfig, createHttpClient, BillingProviderError, networkError, redactSecrets } = require('../../http');

const PROVIDER = 'Moesif';

/** Moesif's reason for refusing a request, from whichever field its reply carries it in. */
function errorDetail(res) {
    const data = res.data;
    if (typeof data === 'string' && data.trim()) return data.trim().slice(0, 300);
    if (data && typeof data === 'object') {
        const text = [data.message, data.error, data.msg, data.moesif_error && data.moesif_error.msg]
            .find((v) => typeof v === 'string' && v);
        if (text) return text;
        const errors = Array.isArray(data.errors) ? data.errors : null;
        if (errors && errors.length) return errors.map((e) => (typeof e === 'string' ? e : JSON.stringify(e))).join('; ').slice(0, 300);
        const json = JSON.stringify(data);
        if (json && json !== '{}') return json.slice(0, 300);
    }
    return `request failed with HTTP ${res.status}`;
}

function createMoesifClient({ managementKey, http }) {
    const cfg = billingConfig().moesif || {};
    const baseUrl = String(cfg.baseUrl || 'https://api.moesif.com').replace(/\/+$/, '');
    const client = http || createHttpClient();

    async function call(method, path, { params, body } = {}) {
        let res;
        try {
            res = await client.request({
                method,
                url: `${baseUrl}${path}`,
                params,
                data: body,
                headers: { Authorization: `Bearer ${managementKey}`, Accept: 'application/json' },
            });
        } catch (err) {
            throw networkError(PROVIDER, err);
        }
        if (res.status >= 200 && res.status < 300) return res.data;
        const error = new BillingProviderError(PROVIDER, res.status, `${PROVIDER}: ${redactSecrets(errorDetail(res), [managementKey])}`);
        // The call Moesif refused, for the logs: method and path only, never the body or key.
        error.request = `${method} ${path}`;
        throw error;
    }

    return {
        /** The billing catalog plans of a billing provider ("stripe"). */
        listCatalogPlans(provider) {
            return call('GET', '/v1/~/billing/catalog/plans', { params: { provider } });
        },

        /** Creates a catalog plan (a product in the provider); resolves to the plan with its `id`. */
        createCatalogPlan(provider, body) {
            return call('POST', '/v1/~/billing/catalog/plans', { params: { provider }, body });
        },

        /** Creates a catalog price on a plan; resolves to the price with its `id`. */
        createCatalogPrice(provider, body) {
            return call('POST', '/v1/~/billing/catalog/prices', { params: { provider }, body });
        },

        /**
         * The application's connection to a billing provider ("stripe"), or null when there is
         * none. Not in Moesif's public API reference.
         * Once a connection is removed (in the dashboard, say), Moesif answers 400 "Unable to get
         * the Partner account" rather than 404 or {}: that is none as well.
         */
        async getPartnerAccount(provider) {
            try {
                return await call('GET', `/v1/~/billing/partneraccount/${encodeURIComponent(provider)}`);
            } catch (err) {
                if (err instanceof BillingProviderError && err.status === 404) return null;
                if (err instanceof BillingProviderError && err.status === 400
                    && /Unable to get the Partner account/i.test(err.message)) return null;
                throw err;
            }
        },

        /** Saves the application's connection to a billing provider. The body carries its key. */
        savePartnerAccount(provider, body) {
            return call('POST', `/v1/~/billing/partneraccount/${encodeURIComponent(provider)}`, { body });
        },

        /**
         * Removes the application's connection to a billing provider, its key included. The route
         * is the one the Moesif dashboard's "disconnect" uses; the partneraccount route has no
         * DELETE. Not in Moesif's public API reference. A connection already gone is not an error.
         */
        async deletePartnerAccount(provider) {
            try {
                await call('DELETE', `/v1/alerting/~/partners/${encodeURIComponent(provider)}`);
            } catch (err) {
                if (err instanceof BillingProviderError && err.status === 404) return;
                throw err;
            }
        },

        /**
         * Archives a catalog plan (undoing a failed setup or enable). It cannot be deleted: Stripe
         * does not delete a product that has prices, even archived ones.
         */
        archiveCatalogPlan(provider, id) {
            return call('PUT', `/v1/~/billing/catalog/plans/${encodeURIComponent(id)}`,
                { params: { provider }, body: { provider, status: 'archived' } });
        },

        /** Deletes a catalog price (undoing a failed setup or enable). */
        deleteCatalogPrice(provider, id) {
            return call('DELETE', `/v1/~/billing/catalog/prices/${encodeURIComponent(id)}`, { params: { provider } });
        },

        /** Deletes a billing meter (undoing a failed setup or enable). */
        deleteBillingMeter(id) {
            return call('DELETE', `/v1/~/billing/meters/${encodeURIComponent(id)}`);
        },

        /**
         * Searches API call events with an Elasticsearch query (`body`: query, aggs, size) over
         * the time range `from`..`to` (ISO 8601).
         */
        searchEvents({ from, to, body }) {
            return call('POST', '/v1/search/~/search/events', { params: { from, to }, body });
        },

        /**
         * Creates a billing meter; resolves to it with its `id` (Moesif returns it as `_id`).
         * Moesif does not document this call or its response.
         */
        async createBillingMeter(body) {
            const created = await call('POST', '/v1/~/billing/meters', { body });
            const id = created && (created._id || created.id);
            if (typeof id === 'string' && id) return { ...created, id };
            // Every meter has the same slug, so one that comes back without its id is found by its
            // name, the newest first.
            const meters = await call('GET', '/v1/~/billing/meters');
            const list = Array.isArray(meters) ? meters : (meters && meters.hits) || [];
            const found = list.filter((m) => m && m.name === body.name)
                .sort((a, b) => String(b.created_at || '').localeCompare(String(a.created_at || '')))[0];
            const foundId = found && (found._id || found.id);
            if (!foundId) {
                throw new BillingProviderError(PROVIDER, 0, `${PROVIDER}: the billing meter "${body.name}" was created but could not be found.`);
            }
            return { ...found, id: foundId };
        },
    };
}

/** A base64url JSON object (a segment of a Moesif id or key), or null when it is not one. */
function decodeSegment(segment) {
    try {
        const value = JSON.parse(Buffer.from(segment, 'base64url').toString('utf8'));
        return value && typeof value === 'object' && !Array.isArray(value) ? value : null;
    } catch (_err) {
        return null;
    }
}

/**
 * The Moesif application ("524:75") a Collector Application ID belongs to, or null when the value
 * is not a Collector Application ID. The ID's first segment is JSON naming the application.
 */
function collectorApplication(applicationId) {
    const claims = decodeSegment(String(applicationId || '').trim().split('.')[0]);
    return claims && typeof claims.app === 'string' && claims.app ? claims.app : null;
}

/** The Moesif application a management key is for (its `app` claim), or null when it does not say. */
function keyApplication(managementKey) {
    const parts = String(managementKey || '').trim().split('.');
    const claims = parts.length === 3 ? decodeSegment(parts[1]) : null;
    return claims && typeof claims.app === 'string' && claims.app ? claims.app : null;
}

/** The URL of Moesif's endpoint for Stripe events, for one Moesif application. */
function stripeWebhookUrl(applicationId) {
    const base = (billingConfig().moesif || {}).stripeWebhookUrl || 'https://api.moesif.net/v1/partners/stripe';
    return `${base}?moesif_application_id=${encodeURIComponent(applicationId)}`;
}

module.exports = { PROVIDER, createMoesifClient, stripeWebhookUrl, collectorApplication, keyApplication };
