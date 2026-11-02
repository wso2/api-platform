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
 * Billing settings: the billing configuration an organization sells paid plans with, which is the
 * Moesif connector billing through Stripe (SETUP): the Moesif keys, and the Stripe key of the
 * Moesif connector's Stripe sub-connector. Every call to a provider goes through the billing
 * provider interface (billing/billingProvider.js).
 *
 * Setting up billing verifies both keys, connects the Moesif application to Stripe with the Stripe
 * key, then makes sure the payment provider sends events to two places: Moesif's endpoint (so Moesif
 * can match usage to customers and subscriptions) and the portal's own (checkout, subscription and
 * invoice changes). The Moesif application's connection to Stripe and Moesif's endpoint are billing's
 * own: setup replaces a connection already there, and removing billing removes both.
 *
 * An organization has at most one billing configuration, and its keys are never changed. To use
 * other keys or another provider, an administrator removes billing and sets it up again; nothing
 * created for the earlier configuration is reused. The payment provider's key decides the mode: a
 * test key puts the portal in test mode, shown on the billing settings page.
 */

const db = require('../../db/driver');
const billingConfigurationDao = require('../../dao/billingConfigurationDao');
const BillingConfigurationDTO = require('../../dto/billingConfigurationDto');
const userIdpReferenceDao = require('../../dao/userIdpReferenceDao');
const { BillingProviderError } = require('../../billing/http');
const billingProvider = require('../../billing/billingProvider');
const constants = require('../../utils/constants');
const util = require('../../utils/util');
const logger = require('../../config/logger');
const { logUserAction } = require('../../middlewares/auditLogger');
const billingCatalogService = require('./billingCatalogService');
const { BillingCatalogError, BillingRetireError } = billingCatalogService;
const billingApiPlanDao = require('../../dao/billingApiPlanDao');
const billingCheckoutDao = require('../../dao/billingCheckoutDao');
const billingSubscriptionDao = require('../../dao/billingSubscriptionDao');

// The payment providers a billing engine can bill through, by id.
const PAYMENT_PROVIDERS = Object.freeze({
    STRIPE: Object.freeze({
        id: 'STRIPE',
        name: 'Stripe',
        description: 'Collects payments from subscribers.',
    }),
});

// The billing engines an administrator can choose in Settings › Billing, each with the payment
// providers it can bill through. Adding one needs its connector (or the engine's payment
// sub-connector) first; the settings page lists every engine here.
const BILLING_ENGINES = Object.freeze([
    Object.freeze({
        id: 'MOESIF',
        name: 'Moesif',
        description: 'Meters API usage and manages plans and prices.',
        paymentProviders: Object.freeze([PAYMENT_PROVIDERS.STRIPE]),
    }),
]);

// Whether Settings › Billing offers a choice: with a single engine and payment provider it shows
// that pair instead of a choice of one.
const BILLING_HAS_CHOICE = BILLING_ENGINES.length > 1 || BILLING_ENGINES.some((e) => e.paymentProviders.length > 1);

// The setup used when a request names none: the first engine and its first payment provider.
const SETUP = Object.freeze({
    billing_provider: BILLING_ENGINES[0].id,
    payment_provider: BILLING_ENGINES[0].paymentProviders[0].id,
});

const NOT_CONNECTED = 'Billing is not set up for this organization.';
const ALREADY_SET_UP = 'Billing is already set up. Remove it to set it up again.';
const SUBSCRIPTIONS_ACTIVE = "Billing can't be removed while paid subscriptions are active.";

class BillingInputError extends Error {
    /** `field`: the request field the problem is about, shown under it; null for the request. */
    constructor(message, field) {
        super(message);
        this.field = field || null;
    }
}

/** One or both providers refused the keys, or could not be reached. */
class KeyCheckError extends Error {
    constructor(failures) {
        super(failures.map((f) => f.message).join(' '));
        this.failures = failures;
    }
}

/** The connector for settings that are not stored yet (or for a stored configuration). */
function connectorFor(settings) {
    return billingProvider.forConfiguration({ ...SETUP, ...settings });
}

/** The portal's public address (server.base_url): the webhook endpoint's unless the administrator chooses another. */
function defaultWebhookBaseUrl() {
    const { config } = require('../../config/configLoader');
    return String((config.server && config.server.baseUrl) || '').replace(/\/+$/, '');
}

/** The URL Stripe sends the organization's billing events to, under its public address `baseUrl`. */
function webhookUrl(orgId, baseUrl) {
    return `${baseUrl}${constants.ROUTE.BASE_PATH}/billing/stripe/webhooks/${orgId}`;
}

const WEBHOOK_BASE_URL_INVALID = 'Enter an https address, such as https://portal.example.com.';

/**
 * A public address for the webhook endpoint: an https URL, with a path when the portal is behind a
 * proxy, and no query, fragment or credentials. Trailing slashes are dropped.
 */
function webhookBaseUrlOf(value) {
    const invalid = () => new BillingInputError(WEBHOOK_BASE_URL_INVALID, 'webhookBaseUrl');
    if (typeof value !== 'string' || value.trim() === '' || value.length > 1023) throw invalid();
    let url;
    try {
        url = new URL(value.trim());
    } catch {
        throw invalid();
    }
    if (url.protocol !== 'https:' || url.username || url.password || url.search || url.hash) throw invalid();
    return `${url.origin}${url.pathname}`.replace(/\/+$/, '');
}

function optionalString(body, name, max) {
    const value = body[name];
    if (value === undefined || value === null || value === '') return undefined;
    if (typeof value !== 'string' || value.trim() === '' || value.length > max) {
        throw new BillingInputError(`${name} must be a string of at most ${max} characters`);
    }
    return value.trim();
}

/**
 * The billing engine and payment provider a request names, defaulting to SETUP. They must be a
 * pair BILLING_ENGINES offers.
 */
function resolveSetup(body) {
    const provider = optionalString(body, 'provider', 20) ?? SETUP.billing_provider;
    const paymentProvider = optionalString(body, 'paymentProvider', 20) ?? SETUP.payment_provider;
    const engine = BILLING_ENGINES.find((e) => e.id === provider);
    if (!engine) {
        throw new BillingInputError(`provider must be one of: ${BILLING_ENGINES.map((e) => e.id).join(', ')}`);
    }
    if (!engine.paymentProviders.some((p) => p.id === paymentProvider)) {
        throw new BillingInputError(`paymentProvider must be one of: ${
            engine.paymentProviders.map((p) => p.id).join(', ')} for ${provider}`);
    }
    return { billing_provider: provider, payment_provider: paymentProvider };
}

/** Validates a set up request. */
function resolveInput(body) {
    if (!body || typeof body !== 'object' || Array.isArray(body)) {
        throw new BillingInputError('Request body is missing or invalid');
    }
    const setup = resolveSetup(body);
    const input = {
        ...setup,
        moesifCollectorApplicationId: optionalString(body, 'moesifCollectorApplicationId', 255),
        moesifManagementKey: optionalString(body, 'moesifManagementKey', 4096),
        stripeApiKey: optionalString(body, 'stripeApiKey', 255),
        stripePublishableKey: optionalString(body, 'stripePublishableKey', 255),
    };
    const missing = ['moesifCollectorApplicationId', 'moesifManagementKey', 'stripeApiKey', 'stripePublishableKey'].filter((k) => !input[k]);
    if (missing.length > 0) {
        throw new BillingInputError(`Missing required fields: ${missing.join(', ')}`);
    }
    const problem = billingProvider.checkSettings(setup, input);
    if (problem) throw new BillingInputError(problem.message, problem.field);
    input.stripeKeyType = billingProvider.paymentsOf(setup).mode(input.stripeApiKey);
    if (!input.stripeKeyType) {
        throw new BillingInputError('stripeApiKey must be a Stripe secret or restricted key (sk_ or rk_), '
            + 'not a publishable key');
    }
    // The browser starts Stripe's checkout with the publishable key: it must be one, in the secret key's mode.
    const publishableMode = billingProvider.paymentsOf(setup).publishableMode(input.stripePublishableKey);
    if (!publishableMode) {
        throw new BillingInputError('Enter a publishable key (pk_test_… or pk_live_…).', 'stripePublishableKey');
    }
    if (publishableMode !== input.stripeKeyType) {
        throw new BillingInputError(`This is a ${publishableMode.toLowerCase()} publishable key, but the secret key is a `
            + `${input.stripeKeyType.toLowerCase()} key. Use keys from the same mode.`, 'stripePublishableKey');
    }
    // Where Stripe sends the portal's events: the portal's own address unless another is given.
    const base = body.webhookBaseUrl;
    input.webhookBaseUrl = base === undefined || base === null || base === ''
        ? defaultWebhookBaseUrl() : webhookBaseUrlOf(base);
    return input;
}

/** Checks the keys with both providers; resolves to the connector and the payment provider's event endpoints. */
async function verifyKeys(input) {
    const connector = connectorFor(input);
    try {
        return { connector, endpoints: await connector.checkKeys() };
    } catch (err) {
        if (err.failures) throw new KeyCheckError(err.failures);
        throw err;
    }
}


/** True when every failure is a provider's answer (not a fault in the portal itself). */
function isProviderFailure(error) {
    return (error.failures || [error]).every((f) => f instanceof BillingProviderError);
}

// The request field that holds each provider's key, so a refusal names the field it is about.
const KEY_FIELDS = Object.freeze({ Moesif: 'moesifManagementKey', Stripe: 'stripeApiKey' });

/*
 * Messages about a provider's refusal are the portal's own words: the provider's (Moesif's or
 * Stripe's) can name internals, so they are only logged, with the request they answered.
 */

/** One provider's refusal, said next to the key it is about. */
function keyFailureText(failure) {
    if (failure.status === 0 || failure.status >= 500) return `${failure.provider} could not be reached. Try again later.`;
    if (failure.status === 401) return `This ${failure.provider} key is invalid. Please enter a valid key.`;
    if (failure.status === 403) return `This ${failure.provider} key does not have a permission billing needs.`;
    return `${failure.provider} refused this key.`;
}

/** What the administrator can do about a provider's refusal of a step, or '' when nothing. */
function refusalAdvice(err) {
    if (err.status === 0 || err.status >= 500) return `${err.provider} could not be reached. Try again later.`;
    if (err.status === 401 || err.status === 403) {
        return 'Check that the keys are correct and have the permissions listed in Settings › Billing.';
    }
    return '';
}

const sentences = (...parts) => parts.filter(Boolean).join(' ');

function sendProviderError(res, error) {
    const failures = error.failures || [error];
    const message = failures.map(keyFailureText).join(' ');
    const errors = failures.map((f) => ({ field: KEY_FIELDS[f.provider] || 'provider', message: keyFailureText(f) }));
    if (failures.every((f) => f.status === 0 || f.status >= 500)) {
        return util.sendError(res, 502, message, { code: 'BILLING_PROVIDER_UNAVAILABLE', errors });
    }
    const hint = failures.some((f) => f.isAuthError)
        ? ' Check that each key is correct and has the permissions listed in Settings › Billing.' : '';
    return util.sendError(res, 400, message + hint, { code: 'BILLING_KEY_INVALID', errors });
}

/** The configuration as the REST API returns it. */
async function toDto(conn) {
    const { createdBy } = await userIdpReferenceDao.buildSingleAuditFields({ created_by: conn.created_by });
    return new BillingConfigurationDTO(conn, {
        webhookEndpointUrl: webhookUrl(conn.org_uuid, conn.stripeWebhookBaseUrl),
        webhookBaseUrl: conn.stripeWebhookBaseUrl,
        mode: billingProvider.paymentsOf(conn).publishableMode(conn.stripePublishableKey),
    }, { createdBy, createdAt: conn.created_at });
}

/**
 * The organization's billing configuration as the settings page shows it, or null. Reads the database only;
 * Stripe and Moesif are not called.
 */
async function describeConfiguration(orgId) {
    const conn = await billingConfigurationDao.get(orgId);
    if (!conn) return null;
    const dto = await toDto(conn);
    // Removing billing is refused while any is active; the page says so before trying.
    dto.activePaidSubscriptions = await countActiveSubscriptions(orgId);
    return dto;
}

/** The number of the organization's paid subscriptions that have not ended. */
async function countActiveSubscriptions(orgId) {
    const rows = await billingSubscriptionDao.listByOrg(orgId);
    return rows.filter((r) => r.status !== 'CANCELLED').length;
}

const getBillingConfiguration = async (req, res) => {
    try {
        const conn = await billingConfigurationDao.get(req.orgId);
        if (!conn) return util.sendError(res, 404, NOT_CONNECTED);
        return res.status(200).json(await toDto(conn));
    } catch (error) {
        logger.error('Error while retrieving the billing configuration', { error });
        return util.sendError(res, 500, 'Error while retrieving the billing configuration');
    }
};

/**
 * A step of setting up billing failed with a provider. `field` names the request field whose key
 * the provider would not accept for this step, when that is the reason.
 */
class SetupStepError extends Error {
    constructor(message, cause, field) {
        super(message);
        this.cause = cause;
        this.field = field || null;
        this.retryable = cause instanceof BillingProviderError && (cause.status === 0 || cause.status >= 500);
    }
}

/**
 * Runs one setup step; a provider refusal becomes a SetupStepError with `message`. With
 * `keyRefused` ({ field, message }), a 401 or 403 is reported against that key field instead.
 * `advice` says what to change when the provider refuses what the step sent (another 4xx).
 */
async function setupStep(message, step, keyRefused, advice) {
    try {
        return await step();
    } catch (err) {
        if (!(err instanceof BillingProviderError)) throw err;
        if (keyRefused && (err.status === 401 || err.status === 403)) {
            throw new SetupStepError(keyRefused.message, err, keyRefused.field);
        }
        const refused = err.status >= 400 && err.status < 500 && err.status !== 401 && err.status !== 403;
        throw new SetupStepError(sentences(`${message}.`, refusalAdvice(err) || (refused && advice)), err);
    }
}

/** Undoes completed setup steps, newest first; failures are only logged. */
async function undoSteps(steps, orgId) {
    for (const step of [...steps].reverse()) {
        try {
            await step.undo();
        } catch (err) {
            logger.warn('Could not undo a billing step', { orgId, step: step.name, error: err.message });
        }
    }
}

/**
 * Sets up billing. Every step must succeed, or the steps already done are undone and nothing is
 * saved:
 *   1. both keys are checked;
 *   2. the Moesif application is connected to Stripe with the Stripe key, replacing any connection;
 *   3. Moesif's endpoint is registered in Stripe (unless it is there already);
 *   4. the portal's endpoint is registered in Stripe, replacing any already at its address;
 *   5. each paid plan already enabled for an API is created in Moesif (billingCatalogService); a
 *      paid plan Moesif does not create fails setup;
 *   6. the configuration is saved, with those plans, which are then on sale.
 */
const createBillingConfiguration = async (req, res) => {
    const orgId = req.orgId;
    const done = [];
    try {
        if (await billingConfigurationDao.get(orgId)) {
            return util.sendError(res, 409, ALREADY_SET_UP);
        }
        const input = resolveInput(req.body);
        const url = webhookUrl(orgId, input.webhookBaseUrl);
        const { connector, endpoints } = await verifyKeys(input);

        const moesifStripe = await setupStep('Moesif could not connect to Stripe',
            () => connector.connectPayments(),
            { field: 'moesifManagementKey', message: 'This Moesif key cannot connect the Moesif application to Stripe. It needs the App configs scopes listed in Settings › Billing.' });
        done.push({ name: 'moesif stripe connection', undo: moesifStripe.undo });

        const moesifEndpoint = await setupStep('Stripe did not accept Moesif\'s webhook endpoint',
            () => connector.ensureMoesifEvents(endpoints));
        if (moesifEndpoint.undo) {
            done.push({ name: 'moesif endpoint', undo: moesifEndpoint.undo });
        }

        const endpoint = await setupStep(`Stripe did not accept the portal's webhook endpoint ${url}`,
            () => connector.payments.createEventEndpoint({ url, endpoints }),
            null, 'Use a public https address that Stripe can reach.');
        done.push({ name: 'portal endpoint', undo: () => connector.payments.removeEventEndpoint(endpoint.ref) });

        const pairs = (await billingApiPlanDao.listPaidPairs(orgId)).map((p) => ({
            api: { uuid: p.api_uuid, name: p.api_name, version: p.api_version },
            planUuid: p.plan_uuid,
        }));
        const catalog = await billingCatalogService.build(orgId, input, pairs, connector);
        done.push({ name: 'paid plans', undo: catalog.undo });
        const conn = {
            billing_provider: input.billing_provider,
            payment_provider: input.payment_provider,
            settings: {
                stripe: {
                    publishableKey: input.stripePublishableKey,
                    webhookEndpointId: endpoint.ref,
                    webhookBaseUrl: input.webhookBaseUrl,
                },
            },
            secrets: {
                moesif: { collectorApplicationId: input.moesifCollectorApplicationId, managementKey: input.moesifManagementKey },
                stripe: { apiKey: input.stripeApiKey, webhookSecret: endpoint.secret },
            },
        };
        const saved = await db.withTransaction(async (t) => {
            const row = await billingConfigurationDao.create(orgId, conn, util.resolveActor(req), t);
            await billingCatalogService.saveApiPlans(catalog.apiPlans, t);
            return row;
        });
        logUserAction('BILLING_CONFIGURATION_CREATED', req,
            { orgId, resourceUuid: orgId, resourceType: 'billing_configuration' });
        // One configuration per organization, read at the address it was created at.
        res.setHeader('Location', `${req.baseUrl}${req.path.replace(/\/$/, '')}`);
        return res.status(201).json(await toDto(saved));
    } catch (error) {
        await undoSteps(done, orgId);
        if (db.isDuplicateKeyError(error)) {
            // Another setup at the same moment saved its configuration first.
            return util.sendError(res, 409, ALREADY_SET_UP);
        }
        if (error instanceof BillingInputError) {
            logger.warn('Billing not set up: invalid request', { orgId, field: error.field, reason: error.message });
            return util.sendError(res, 400, error.message, error.field
                ? { code: 'BILLING_INPUT_INVALID', errors: [{ field: error.field, message: error.message }] } : {});
        }
        if (error instanceof SetupStepError && error.field) {
            logger.warn('Billing not set up: a provider refused a key for a step', {
                orgId, field: error.field, status: error.cause && error.cause.status,
                request: error.cause && error.cause.request, cause: error.cause && error.cause.message,
            });
            return util.sendError(res, 400, error.message,
                { code: 'BILLING_KEY_INVALID', errors: [{ field: error.field, message: error.message }] });
        }
        if (error instanceof SetupStepError || error instanceof BillingCatalogError) {
            logger.warn('Billing not set up: a step failed', {
                orgId, reason: error.message, status: error.cause && error.cause.status,
                request: error.cause && error.cause.request, cause: error.cause && error.cause.message,
            });
            return util.sendError(res, error.retryable ? 502 : 400, error.message,
                { code: error instanceof BillingCatalogError ? 'BILLING_CATALOG_FAILED' : 'BILLING_SETUP_FAILED' });
        }
        if ((error instanceof BillingProviderError || error instanceof KeyCheckError) && isProviderFailure(error)) {
            // The providers' answers only; the keys themselves are never logged.
            // Key lengths, not keys: enough to tell a truncated or autofilled value from the real one.
            const lengthOf = (name) => (req.body && typeof req.body[name] === 'string' ? req.body[name].trim().length : null);
            logger.warn('Billing not set up: a provider refused a key', {
                orgId,
                failures: (error.failures || [error]).map((f) => ({ provider: f.provider, status: f.status, message: f.message })),
                keyLengths: { moesifManagementKey: lengthOf('moesifManagementKey'), stripeApiKey: lengthOf('stripeApiKey') },
            });
            return sendProviderError(res, error);
        }
        logger.error('Error while setting up billing', { error: error.message });
        return util.sendError(res, 500, 'Error while setting up billing');
    }
};

/** A step of removing billing that a provider refused; `message` says which. */
class RemovalStepError extends Error {
    constructor(message, cause) {
        super(message);
        this.cause = cause;
        this.retryable = Boolean(cause) && (cause.status === 0 || cause.status === 429 || cause.status >= 500);
    }
}

/**
 * Runs one removal step and records its undo. A provider that no longer accepts the stored key
 * (401) cannot be asked to remove anything, and billing must still be removable: the step is
 * skipped and logged. Any other refusal fails the removal.
 */
async function removalStep(name, failure, fn, done, orgId, forbidden) {
    try {
        const undo = await fn();
        if (undo) done.push({ name, undo });
    } catch (err) {
        if (!(err instanceof BillingProviderError)) throw err;
        if (err.status === 401) {
            logger.warn('Billing removal: a provider no longer accepts the stored key; step skipped',
                { orgId, step: name, provider: err.provider });
            return;
        }
        const advice = (err.status === 403 && forbidden) || refusalAdvice(err);
        throw new RemovalStepError(sentences(`${failure}. Billing was not removed.`, advice), err);
    }
}

/**
 * Ends the organization's open checkouts, so none can be paid once billing is gone. Resolves to
 * true when one was paid before it could be ended: its subscription is on the way, and removing
 * billing must wait for it like any active one.
 */
async function endOpenCheckouts(orgId, connector) {
    for (const checkout of await billingCheckoutDao.listOpenByOrg(orgId)) {
        if (!checkout.checkout_ref) {
            // Never reached the payment provider: nothing there to end.
            await billingCheckoutDao.deleteOpenCheckout(checkout.uuid);
            continue;
        }
        let ended;
        try {
            ended = await connector.payments.expireCheckout(checkout.checkout_ref);
        } catch (err) {
            if (err instanceof BillingProviderError && err.status === 401) {
                logger.warn('Billing removal: an open checkout could not be ended; the stored key no longer works',
                    { orgId, checkoutUuid: checkout.uuid });
                continue;
            }
            throw new RemovalStepError(
                sentences('Stripe did not end an open checkout. Billing was not removed.', refusalAdvice(err)), err);
        }
        if (ended.state !== 'EXPIRED') return true;
        await billingCheckoutDao.deleteOpenCheckout(checkout.uuid);
    }
    return false;
}

/** Takes the organization's API plans off sale with the billing provider. */
async function retireApiPlans(orgId, connector) {
    try {
        await billingCatalogService.retire(orgId, connector, await billingApiPlanDao.listByOrg(orgId));
    } catch (err) {
        if (!(err instanceof BillingRetireError)) throw err;
        if (err.cause && err.cause.status === 401) {
            logger.warn('Billing removal: the billing provider no longer accepts the stored key; paid plans left on sale there',
                { orgId, provider: err.cause.provider });
            return;
        }
        throw new RemovalStepError(sentences('The paid plans could not be taken off sale. Billing was not removed.',
            refusalAdvice(err.cause) || (err.retryable ? 'Try again.' : '')), err.cause);
    }
}

/** Billing can't be removed while a paid subscription is active. */
class SubscriptionsActiveError extends Error {
    constructor() {
        super(SUBSCRIPTIONS_ACTIVE);
        this.code = 'BILLING_SUBSCRIPTIONS_ACTIVE';
    }
}

/**
 * Removes the organization's billing and everything that holds a key the administrator entered:
 *   1. refused while a paid subscription is active;
 *   2. open checkouts are ended;
 *   3. every paid plan sold on an API is taken off sale with Moesif;
 *   4. Moesif's connection to Stripe, with the Stripe key it holds, is removed;
 *   5. Moesif's endpoint in Stripe, whose address carries the Collector Application Id, is removed;
 *   6. the portal's endpoint is removed from Stripe;
 *   7. the configuration and its keys are deleted, with its customers, their checkouts, and its API
 *      plans.
 * When a step fails, steps 4 to 6 already done are undone and billing is not removed. Resolves to
 * false when the organization has no billing; throws SubscriptionsActiveError or RemovalStepError.
 */
async function removeBilling(orgId) {
    const done = [];
    try {
        const existing = await billingConfigurationDao.getWithSecrets(orgId);
        if (!existing) return false;
        if (await countActiveSubscriptions(orgId) > 0) throw new SubscriptionsActiveError();
        const connector = billingProvider.forConfiguration(existing);
        if (await endOpenCheckouts(orgId, connector)) throw new SubscriptionsActiveError();
        await retireApiPlans(orgId, connector);

        await removalStep('moesif stripe connection', 'Moesif did not remove its connection to Stripe',
            async () => (await connector.disconnectPayments()).undo, done, orgId,
            'The Moesif key does not have the Alert rules: create scope. Disconnect Stripe from the Moesif '
            + 'application, then remove billing again.');
        await removalStep('moesif endpoint', 'Stripe did not remove Moesif\'s webhook endpoint',
            async () => (await connector.removeMoesifEvents()).undo, done, orgId);
        const portalEndpointRef = existing.stripeWebhookEndpointId;
        if (portalEndpointRef) {
            await removalStep('portal endpoint', 'Stripe did not remove the portal\'s webhook endpoint', async () => {
                await connector.payments.removeEventEndpoint(portalEndpointRef);
                return async () => {
                    const again = await connector.payments.createEventEndpoint(
                        { url: webhookUrl(orgId, existing.stripeWebhookBaseUrl), endpoints: [] });
                    await billingConfigurationDao.setEventEndpoint(orgId, { endpointId: again.ref, secret: again.secret });
                };
            }, done, orgId);
        }
        await billingConfigurationDao.delete(orgId);
        return true;
    } catch (error) {
        await undoSteps(done, orgId);
        if (error instanceof RemovalStepError) {
            logger.warn('Billing not removed: a provider refused a step', {
                orgId, reason: error.message, status: error.cause && error.cause.status,
                request: error.cause && error.cause.request, cause: error.cause && error.cause.message,
            });
        }
        throw error;
    }
}

/** DELETE /billing-configuration: removes the organization's billing (removeBilling). */
const deleteBillingConfiguration = async (req, res) => {
    const orgId = req.orgId;
    try {
        if (!(await removeBilling(orgId))) return util.sendError(res, 404, NOT_CONNECTED);
        logUserAction('BILLING_CONFIGURATION_DELETED', req,
            { orgId, resourceUuid: orgId, resourceType: 'billing_configuration' });
        return res.status(204).send();
    } catch (error) {
        if (error instanceof SubscriptionsActiveError) {
            return util.sendError(res, 409, error.message, { code: error.code });
        }
        if (error instanceof RemovalStepError) {
            return util.sendError(res, error.retryable ? 502 : 400, error.message, { code: 'BILLING_REMOVE_FAILED' });
        }
        logger.error('Error while removing the billing configuration', { error: error.message });
        return util.sendError(res, 500, 'Error while removing the billing configuration');
    }
};

/**
 * Moves the portal's webhook endpoint to another public address (`webhookBaseUrl`): for a portal
 * Stripe reaches through another host or a proxy. Stripe keeps the endpoint and its signing secret,
 * so events continue to arrive and be verified; when Stripe no longer has the endpoint, a new one is
 * registered. The keys and everything else stay as they are.
 */
const updateBillingWebhookEndpoint = async (req, res) => {
    const orgId = req.orgId;
    try {
        const conn = await billingConfigurationDao.getWithSecrets(orgId);
        if (!conn) return util.sendError(res, 404, NOT_CONNECTED);
        const baseUrl = webhookBaseUrlOf((req.body || {}).webhookBaseUrl);
        const url = webhookUrl(orgId, baseUrl);
        const connector = billingProvider.forConfiguration(conn);
        const endpoint = await setupStep(`Stripe did not accept the webhook endpoint ${url}`,
            () => connector.payments.moveEventEndpoint({
                url, current: { ref: conn.stripeWebhookEndpointId, secret: conn.stripeWebhookSecret },
            }),
            null, 'Use a public https address that Stripe can reach.');
        const saved = await billingConfigurationDao.setEventEndpoint(orgId,
            { baseUrl, endpointId: endpoint.ref, secret: endpoint.secret });
        logUserAction('BILLING_WEBHOOK_ENDPOINT_UPDATED', req,
            { orgId, resourceUuid: orgId, resourceType: 'billing_configuration' });
        return res.status(200).json(await toDto(saved));
    } catch (error) {
        if (error instanceof BillingInputError) {
            logger.warn('Billing webhook endpoint not changed: invalid request', { orgId, field: error.field, reason: error.message });
            return util.sendError(res, 400, error.message,
                { code: 'BILLING_INPUT_INVALID', errors: [{ field: error.field, message: error.message }] });
        }
        if (error instanceof SetupStepError) {
            logger.warn('Billing webhook endpoint not changed: Stripe refused it', {
                orgId, reason: error.message, status: error.cause && error.cause.status,
                request: error.cause && error.cause.request, cause: error.cause && error.cause.message,
            });
            if (error.retryable) {
                return util.sendError(res, 502, error.message, { code: 'BILLING_PROVIDER_UNAVAILABLE' });
            }
            return util.sendError(res, 400, error.message,
                { code: 'BILLING_INPUT_INVALID', errors: [{ field: 'webhookBaseUrl', message: error.message }] });
        }
        logger.error('Error while changing the billing webhook endpoint', { error: error.message });
        return util.sendError(res, 500, 'Error while changing the billing webhook endpoint');
    }
};

module.exports = {
    BILLING_ENGINES,
    BILLING_HAS_CHOICE,
    PAYMENT_PROVIDERS,
    SETUP,
    RemovalStepError,
    SubscriptionsActiveError,
    removeBilling,
    webhookUrl,
    describeConfiguration,
    getBillingConfiguration,
    createBillingConfiguration,
    deleteBillingConfiguration,
    updateBillingWebhookEndpoint,
};
