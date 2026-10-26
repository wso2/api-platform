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
 * The billing catalog: what the billing provider needs to sell one paid plan on one API. With
 * Moesif, each part of the plan's pricing is its own Moesif plan with one price: a flat rate plan's
 * price, or each usage price. A usage price also gets a billing meter, which counts the calls its
 * meter counts that match the price's filters. Moesif creates them in the payment provider (Stripe),
 * where they become the items of one subscription, billed on one invoice.
 *
 * The request that makes a plan sellable on an API (publishing or updating the API, or setting up
 * billing) creates it with the provider before it saves anything (build), part by part (buildItem):
 * when all of it exists, the request saves the API plans with its own changes (saveApiPlans) and the
 * plan is offered; when any of it fails, everything created is removed again (undo) and the request
 * fails, so the provider is never left with part of a plan and the plan is never half on sale. A
 * request whose own save then fails removes what it created as well (the undo that build returns).
 *
 * A usage price whose API type filter rules out the API's type gets no item: nothing is created for
 * it, so the subscription, its invoices and its usage never show it (priceAppliesToApiType). A usage
 * plan none of whose prices apply to the API is refused.
 *
 * An API plan is never changed afterwards: the plan's pricing and its meters' definitions are locked
 * once it exists (planPricingService, meterService). When its plan stops being sold on its API, it
 * is taken off sale with the provider (retire).
 */

const billingApiPlanDao = require('../../dao/billingApiPlanDao');
const subscriptionPlanDao = require('../../dao/subscriptionPlanDao');
const planPricingDao = require('../../dao/planPricingDao');
const meterDao = require('../../dao/meterDao');
const { countedFilters, gatewayApiType, priceAppliesToApiType } = require('./planPricingValidation');
const billingProvider = require('../../billing/billingProvider');
const { BillingProviderError } = require('../../billing/http');
const logger = require('../../config/logger');

// When a request fails after creating some of a plan's objects with the provider, they are deleted
// again (undo). A delete refused for the provider's rate limit is tried again at the start of the next
// calendar minute (Moesif allows 500 management calls per calendar minute); one that cannot reach the
// provider, or that the provider fails, is tried again after 2 and then 4 seconds. Each delete is tried
// at most UNDO_ATTEMPTS times; what still fails is logged as left behind.
const UNDO_ATTEMPTS = 3;
const undoWaits = {
    rateLimited: () => 60000 - (Date.now() % 60000) + 1000,
    unavailable: (attempt) => 2000 * (attempt + 1),
};

/** The portal's own refusal to sell a paid plan on an API, before the provider is called; its message is shown as it is. */
class CatalogRefusal extends Error {}

/** A part not started because another part of the same request had already failed. */
class BuildStopped extends Error {}

/**
 * A paid plan could not be created with the billing provider; what was created for it has been
 * deleted again (undo logs any object it could not delete). The message is the portal's own: the
 * provider's reason can name internals, so callers log `cause` instead.
 */
class BillingCatalogError extends Error {
    constructor(item, cause) {
        const fromProvider = cause instanceof BillingProviderError;
        // A provider that refused for its rate limit, could not be reached or failed may work in a
        // moment; anything else needs a change. The provider's reason is logged, not shown.
        const retryable = fromProvider && (cause.status === 0 || cause.status === 429 || cause.status >= 500);
        const by = fromProvider ? cause.provider : 'The billing provider';
        const where = `${item.api.name} ${item.api.version}`;
        let message;
        if (cause instanceof CatalogRefusal) message = cause.message;
        else if (retryable) message = `The paid plan "${item.plan.display_name}" could not be set up for sale on ${where} right now. Try again.`;
        else message = `${by} could not create the paid plan "${item.plan.display_name}" for ${where}.`;
        super(message);
        this.name = 'BillingCatalogError';
        this.code = 'ERR_BILLING_CATALOG';
        this.cause = cause;
        this.retryable = retryable;
    }
}

/**
 * The parts of a plan's pricing the provider sells on an API, in order: a flat rate plan's one
 * price, or each usage price that applies to the API's type, with the meter it counts with
 * (narrowed by the price's filters). Throws CatalogRefusal when no usage price applies.
 */
function partsOf(plan, api, pricing, meters, connector) {
    if (pricing.pricing_type === 'FLAT_RATE') return [{ price: null, meter: null }];
    const parts = [];
    for (const price of pricing.prices) {
        const meter = meters.get(price.meter_handle);
        if (!meter) throw new Error(`The meter of the price "${price.name}" no longer exists.`);
        if (priceAppliesToApiType(price.filters, meter, api.type)) {
            const counted = countedFilters(meter, price.filters);
            parts.push({ price, meter: connector.catalog.priceMeter(counted.meter, counted.filters) });
        }
    }
    if (parts.length === 0) {
        throw new CatalogRefusal(`None of the prices of the plan ${plan.display_name} apply to this API's type `
            + `(${gatewayApiType(api.type) || api.type}).`);
    }
    return parts;
}

/**
 * What the catalog needs for some (API, plan) pairs: one item per pair whose plan is paid and that
 * has no API plan yet. `pairs` are { api: { uuid, name, version, type }, planUuid };
 * the API's uuid is null for an API not saved yet. Meters that cannot be counted, and a usage plan
 * none of whose prices apply to the API's type, fail here, before anything is created. Reads go
 * through `t` when the caller holds a transaction.
 */
async function itemsFor(orgId, connector, pairs, t) {
    const items = [];
    for (const { api, planUuid } of pairs) {
        const pricing = await planPricingDao.getByPlan(orgId, planUuid, t);
        if (!pricing) continue;
        if (api.uuid && await billingApiPlanDao.getApiPlan(orgId, api.uuid, planUuid, t)) continue;
        const plan = await subscriptionPlanDao.getByUuid(orgId, planUuid, t);
        if (!plan) continue;
        const handles = [...new Set(pricing.prices.map((p) => p.meter_handle))];
        const meters = new Map((await meterDao.listByHandles(orgId, handles, t)).map((m) => [m.handle, m]));
        const item = { api, plan, pricing };
        try {
            item.parts = partsOf(plan, api, pricing, meters, connector);
            for (const part of item.parts) {
                if (part.meter) connector.catalog.checkMeter(part.meter);
            }
        } catch (err) {
            throw new BillingCatalogError(item, err);
        }
        items.push(item);
    }
    return items;
}

/**
 * Deletes what `created` lists ({ kind, ref }, oldest first), newest first. A delete that fails is
 * logged, naming the object left behind, and the rest continue. `call` runs each provider call.
 */
async function undo(connector, created, call = (fn) => fn()) {
    const removers = {
        meter: (ref) => connector.catalog.removeUsageMeter(ref),
        price: (ref) => connector.catalog.removePrice(ref),
        plan: (ref) => connector.catalog.removePlan(ref),
    };
    for (const { kind, ref } of [...created].reverse()) {
        try {
            await call(() => removers[kind](ref));
        } catch (err) {
            logger.warn('Could not remove a billing catalog object after a failed build', { kind, ref, error: err.message });
        }
    }
}

/** The name of the provider's plan for a part: the API and plan, then the price's name for a usage price. */
function partName(api, plan, price) {
    const base = `${api.name} ${api.version} - ${plan.display_name}`;
    return price ? `${base} - ${price.name}` : base;
}

/**
 * Creates one item with the provider: for each part of the plan, a plan and its price, and for a
 * usage price the billing meter that reports the price's count to it, in that order. Parts are
 * created in parallel, each taking one of `limit`'s slots from its plan to its meter; `call` runs
 * each provider call. `onCreated({ kind, ref })` is awaited as soon as an object exists,
 * and a part stops when it throws. Once a part fails, parts not yet started are skipped and the ones
 * under way are waited for, then the first failure is thrown: what was created was all passed to
 * `onCreated`. Resolves to the API plan's items, in the parts' order.
 */
async function buildItem(connector, item, { limit, onCreated, call = (fn) => fn() }) {
    const { catalog } = connector;
    const { api, plan, pricing } = item;
    let stopped = false;
    const create = async (kind, fn) => {
        const ref = await call(fn);
        await onCreated({ kind, ref });
        return ref;
    };
    const buildPart = async ({ price, meter }) => {
        const name = partName(api, plan, price);
        const planRef = await create('plan', () => catalog.createPlan({ name }));
        const priceRef = await create('price', () => (price
            ? catalog.createUsagePrice({ planRef, name, pricing, price })
            : catalog.createFlatPrice({ planRef, name, pricing })));
        const meterRef = price
            ? await create('meter', () => catalog.createUsageMeter({ name, meter, planRef, priceRef }))
            : null;
        return { priceUuid: price ? price.uuid : pricing.flat_price_uuid, planRef, priceRef, meterRef };
    };
    const results = await Promise.allSettled(item.parts.map((part) => limit(async () => {
        if (stopped) return null;
        try {
            return await buildPart(part);
        } catch (err) {
            stopped = true;
            throw err;
        }
    })));
    const failed = results.find((r) => r.status === 'rejected');
    if (failed) throw failed.reason;
    return results.map((r) => r.value);
}

/** Saves a built API plan ({ orgId, apiUuid, planUuid, items }) in the caller's transaction. */
async function saveApiPlan(apiPlan, t) {
    await billingApiPlanDao.insertApiPlan(apiPlan, t);
}

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

/** Runs a provider call of an undo, trying again after a rate-limit refusal or a provider that is briefly unavailable. */
async function persisting(fn) {
    for (let attempt = 0; ; attempt += 1) {
        try {
            return await fn();
        } catch (err) {
            const temporary = err instanceof BillingProviderError && (err.status === 0 || err.status === 429 || err.status >= 500);
            if (!temporary || attempt >= UNDO_ATTEMPTS - 1) throw err;
            await sleep(err.status === 429 ? undoWaits.rateLimited() : undoWaits.unavailable(attempt));
        }
    }
}

/** Runs at most `max` tasks at a time, in the order they were given. */
function createLimiter(max) {
    let active = 0;
    const waiting = [];
    const next = () => {
        if (active >= max || waiting.length === 0) return;
        active += 1;
        const { task, resolve, reject } = waiting.shift();
        Promise.resolve().then(task).then(resolve, reject).finally(() => {
            active -= 1;
            next();
        });
    };
    return (task) => new Promise((resolve, reject) => {
        waiting.push({ task, resolve, reject });
        next();
    });
}

// At most [api_portal.billing.moesif] max_parallel_prices prices are created at a time, across every
// request of this instance, to stay well under the provider's rate limit (Moesif: 500 management calls
// per calendar minute). Made on first use, from the loaded configuration.
let partLimit;
function partLimiter() {
    if (!partLimit) partLimit = createLimiter(require('../../config/configLoader').config.billing.moesif.maxParallelPrices);
    return partLimit;
}

/**
 * Creates with the billing provider the paid plans some (API, plan) pairs enable, before the request
 * saves anything. `pairs` are { api: { uuid, name, version, type }, planUuid }, the API's uuid null
 * when the request creates it; `conn` is the organization's billing configuration and `connector`
 * its billing provider. Pairs whose plan is free, or that have an API plan already, need
 * nothing. When any part fails, everything created is removed and BillingCatalogError is thrown,
 * naming the plan; the provider's reason is logged. Resolves to { apiPlans, undo }: the API plans for
 * the request to save (saveApiPlans), and undo, which removes what was created when that save fails.
 */
async function build(orgId, conn, pairs, connector = conn && billingProvider.forConfiguration(conn)) {
    const nothing = { apiPlans: [], undo: async () => {} };
    if (!conn || pairs.length === 0) return nothing;
    const items = await itemsFor(orgId, connector, pairs);
    if (items.length === 0) return nothing;
    const created = [];
    let failed = false;
    // Once a part fails, parts of every plan of the request not started yet are skipped.
    const limit = (task) => partLimiter()(() => {
        if (failed) throw new BuildStopped();
        return task();
    });
    const results = await Promise.allSettled(items.map((item) => buildItem(connector, item, {
        limit,
        onCreated: async (object) => { created.push(object); },
    }).catch((err) => {
        failed = true;
        throw err;
    })));
    const failure = results.findIndex((r) => r.status === 'rejected' && !(r.reason instanceof BuildStopped));
    if (failure >= 0) {
        const item = items[failure];
        const cause = results[failure].reason;
        logger.warn('A paid plan could not be created with the billing provider; what was created is removed and the request fails', {
            orgId, plan: item.plan.handle, api: `${item.api.name} ${item.api.version}`,
            provider: cause.provider, status: cause.status, request: cause.request, cause: cause.message,
            rateLimited: cause.status === 429,
        });
        await undo(connector, created, persisting);
        throw new BillingCatalogError(item, cause);
    }
    const apiPlans = items.map((item, i) => ({ orgId, apiUuid: item.api.uuid, planUuid: item.plan.uuid,
        items: results[i].value }));
    return { apiPlans, undo: () => undo(connector, created, persisting) };
}

/** Saves, in the request's transaction, the API plans build() created; `apiUuid` fills in an API the request has just created. */
async function saveApiPlans(apiPlans, t, apiUuid) {
    for (const apiPlan of apiPlans) {
        await saveApiPlan({ ...apiPlan, apiUuid: apiPlan.apiUuid || apiUuid }, t);
    }
}

/** The provider could not take an API plan off sale. */
class BillingRetireError extends Error {
    constructor(cause) {
        const fromProvider = cause instanceof BillingProviderError;
        const retryable = fromProvider && (cause.status === 0 || cause.status === 429 || cause.status >= 500);
        const by = fromProvider ? cause.provider : 'The billing provider';
        super(`${by} could not take the paid plans off sale.`);
        this.name = 'BillingRetireError';
        this.cause = cause;
        this.retryable = retryable;
    }
}

/**
 * Takes API plans off sale with the provider (`apiPlans`: billingApiPlanDao rows): each item's billing
 * meter is removed and its price and plan are archived. Throws BillingRetireError when a part fails.
 */
async function retire(orgId, connector, apiPlans) {
    for (const apiPlan of apiPlans) {
        for (const item of apiPlan.items) {
            try {
                await persisting(() => connector.catalog.archivePart(
                    { planRef: item.plan_ref, priceRef: item.price_ref, meterRef: item.meter_ref }));
            } catch (err) {
                logger.warn('A paid plan could not be taken off sale with the billing provider', {
                    orgId, api: apiPlan.api_uuid, plan: apiPlan.plan_uuid, provider: err.provider,
                    status: err.status, request: err.request, cause: err.message,
                });
                throw new BillingRetireError(err);
            }
        }
    }
}

module.exports = {
    BillingCatalogError, BillingRetireError, itemsFor, buildItem, undo, saveApiPlan, build, saveApiPlans, retire, undoWaits,
};
