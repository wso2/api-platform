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
 * End-to-end tests for the meter handlers and plan pricing, against a real database.
 *
 * billingValidation.test.js covers the request rules in isolation. This file drives the
 * handlers, because what matters here is state: pricing is given only when a plan is created and
 * names meters that exist, a meter a plan charges for can neither change nor be deleted, deleting
 * a plan deletes its pricing, and every change records exactly one event for the Platform API, in
 * the same transaction.
 *
 * Same setup as keyManagerService.handlers.test.js: a temporary SQLite file built from the
 * shipped schema, driven through one child process so later calls see what earlier ones did.
 */

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const PROJECT_ROOT = path.join(__dirname, '..', '..', '..');
const METER_SERVICE = path.join(__dirname, 'meterService.js');
const PRICING_SERVICE = path.join(__dirname, 'planPricingService.js');
const PLAN_SERVICE = path.join(__dirname, '..', 'apiMetadataService.js');
const SCHEMA = path.join(PROJECT_ROOT, 'database', 'schema.sqlite.sql');
const SQLITE = path.join(PROJECT_ROOT, 'node_modules', 'better-sqlite3');

const ORG = 'org-uuid-1';
const PORTAL = 'test-portal';

let tmpDir;
test.after(() => {
    if (tmpDir) fs.rmSync(tmpDir, { recursive: true, force: true });
});

const CONFIG = (dbFile) => `
[api_portal.security]
encryption_key = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
session_secret = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

[api_portal.organization]
handle = "default"
portal_id = "${PORTAL}"

[api_portal.database]
type = "sqlite"
path = ${JSON.stringify(dbFile)}
`;

const PROBE = `
const meters = require(${JSON.stringify(METER_SERVICE)});
const pricing = require(${JSON.stringify(PRICING_SERVICE)});
const plans = require(${JSON.stringify(PLAN_SERVICE)});
const Database = require(${JSON.stringify(SQLITE)});
const out = {};

function call(fn, req) {
    return new Promise((resolve) => {
        const headers = {};
        const res = {
            statusCode: 200,
            setHeader(k, v) { headers[k] = v; },
            status(c) { this.statusCode = c; return this; },
            json(b) { resolve({ status: this.statusCode, body: b, headers }); return this; },
            send(b) { resolve({ status: this.statusCode, body: b === undefined ? null : b, headers }); return this; },
        };
        Promise.resolve(fn(req, res)).catch((e) => resolve({ status: 'THREW', body: String(e && e.message) }));
    });
}

const as = (extra) => Object.assign(
    { orgId: ${JSON.stringify(ORG)}, auth: { userId: 'admin-1' }, baseUrl: '/api-portal', path: '/api/v0.9/meters', query: {}, params: {}, body: {} },
    extra || {}
);

const calls = {
    displayName: 'Successful calls',
    type: 'REQUEST_COUNT',
    filterFields: [{ field: 'HTTP_METHOD' }],
};

(async () => {
    const db = new Database(${JSON.stringify('DB_FILE')});
    const count = (q, ...p) => db.prepare(q).get(...p).n;

    // --- meters -----------------------------------------------------------
    // As the portal routes it (see as()): the router at the portal's root, the operation's path under it.
    out.created = await call(meters.createMeter, as({ body: calls }));
    out.derivedTaken = await call(meters.createMeter, as({ body: { ...calls, type: 'TOKENS',
        typeKey: 'metadata.aiTokenUsage.totalTokens' } }));
    out.duplicateId = await call(meters.createMeter, as({ body: { ...calls, id: 'successful-calls' } }));
    out.invalid = await call(meters.createMeter, as({ body: { ...calls, type: 'BYTES' } }));
    out.unknownFilterField = await call(meters.createMeter, as({ body: { ...calls, id: 'bad-fields',
        filterFields: [{ field: 'COLOR' }] } }));
    out.filterFieldTwice = await call(meters.createMeter, as({ body: { ...calls, id: 'bad-fields',
        filterFields: [{ field: 'API_TYPE' }, { field: 'api_type' }] } }));
    out.filterFieldWithOperator = await call(meters.createMeter, as({ body: { ...calls, id: 'bad-fields',
        filterFields: [{ field: 'API_TYPE', operator: 'EQUALS' }] } }));
    out.withConditions = await call(meters.createMeter, as({ body: { ...calls, id: 'with-conditions',
        conditions: [{ field: 'HTTP_STATUS_CODE', operator: 'IN' }] } }));
    out.get = await call(meters.getMeter, as({ params: { meterId: 'successful-calls' } }));
    out.getMissing = await call(meters.getMeter, as({ params: { meterId: 'no-such-meter' } }));
    // No plan prices the meter yet, so it can change.
    out.updated = await call(meters.updateMeter, as({
        params: { meterId: 'successful-calls' },
        body: { ...calls, description: 'Calls that succeeded' },
    }));
    out.renamed = await call(meters.updateMeter, as({
        params: { meterId: 'successful-calls' }, body: { ...calls, id: 'other-id' },
    }));
    out.updateMissing = await call(meters.updateMeter, as({ params: { meterId: 'no-such-meter' }, body: calls }));

    // --- plans created with their pricing ---------------------------------
    const goldPricing = {
        pricingType: 'USAGE_BASED', currency: 'USD', billingPeriod: 'MONTH',
        prices: [{ pricingModel: 'PER_UNIT', unitAmount: '0.01', meterId: 'successful-calls',
            filters: [{ field: 'http_method', operator: 'equals', values: ['get'] }] }],
    };
    out.freePlan = await call(plans.addSubscriptionPlans, as({ body: { id: 'free', displayName: 'Free' } }));
    out.pricingFree = await call(pricing.getPlanPricing, as({ params: { planId: 'free' } }));
    out.pricingNoPlan = await call(pricing.getPlanPricing, as({ params: { planId: 'no-such-plan' } }));
    out.goldCreated = await call(plans.addSubscriptionPlans, as({ body: { id: 'gold', displayName: 'Gold', pricing: goldPricing } }));
    out.pricingGet = await call(pricing.getPlanPricing, as({ params: { planId: 'gold' } }));
    out.renamedWhilePriced = await call(meters.updateMeter, as({
        params: { meterId: 'successful-calls' }, body: { ...calls, displayName: 'Billed calls' },
    }));
    out.deleteWhilePricedByGold = await call(meters.deleteMeter, as({ params: { meterId: 'successful-calls' } }));
    // A PUT of a plan that does not exist creates it, here pricing the same meter twice: POST
    // calls graduated, GET calls per call.
    out.silverCreated = await call(plans.putSubscriptionPlans, as({ body: { id: 'silver', displayName: 'Silver', pricing: {
        pricingType: 'USAGE_BASED', currency: 'USD', billingPeriod: 'MONTH',
        prices: [{ pricingModel: 'GRADUATED', meterId: 'successful-calls',
            filters: [{ field: 'HTTP_METHOD', operator: 'EQUALS', values: ['POST'] }],
            tiers: [{ upTo: 1000, unitAmount: '0' }, { upTo: null, unitAmount: '0.002' }] },
        { ...goldPricing.prices[0] }],
    } } }));
    out.silverGet = await call(pricing.getPlanPricing, as({ params: { planId: 'silver' } }));

    // Pricing the portal refuses creates no plan.
    const pricedPlan = (id, prices) => call(plans.addSubscriptionPlans, as({ body: { id, displayName: id,
        pricing: { ...goldPricing, prices } } }));
    out.pricingOverlap = await pricedPlan('overlap', [goldPricing.prices[0], { ...goldPricing.prices[0], filters: [] }]);
    out.pricingUnknownMeter = await pricedPlan('unknown-meter', [{ ...goldPricing.prices[0], meterId: 'missing-meter' }]);
    out.pricingFieldNotAllowed = await pricedPlan('type-filter', [{ ...goldPricing.prices[0],
        filters: [{ field: 'API_TYPE', operator: 'EQUALS', values: ['RestApi'] }] }]);
    out.pricingZeroFlat = await call(plans.addSubscriptionPlans, as({ body: { id: 'zero', displayName: 'Zero',
        pricing: { pricingType: 'FLAT_RATE', currency: 'USD', billingPeriod: 'MONTH', flatAmount: '0.00' } } }));
    out.refusedPlanRows = count("SELECT COUNT(*) AS n FROM subscription_plans WHERE handle IN ('overlap', 'unknown-meter', 'named', 'type-filter', 'zero')");

    // --- a meter plans price ----------------------------------------------
    out.describedWhilePriced = await call(meters.updateMeter, as({
        params: { meterId: 'successful-calls' }, body: { ...calls, description: 'Calls that went through' },
    }));
    out.filterFieldAddedWhilePriced = await call(meters.updateMeter, as({
        params: { meterId: 'successful-calls' },
        body: { ...calls, description: 'Calls that succeeded', filterFields: [{ field: 'HTTP_METHOD' }, { field: 'API_TYPE' }] },
    }));
    out.filterFieldRemovedWhilePriced = await call(meters.updateMeter, as({
        params: { meterId: 'successful-calls' }, body: { ...calls, description: 'Calls that succeeded', filterFields: [] },
    }));
    out.eventKeyChangedWhilePriced = await call(meters.updateMeter, as({
        params: { meterId: 'successful-calls' },
        body: { ...calls, description: 'Calls that succeeded', filterFields: [{ field: 'HTTP_METHOD', eventKey: 'verb' }] },
    }));
    out.meterWhilePriced = await call(meters.getMeter, as({ params: { meterId: 'successful-calls' } }));
    out.listWithUsage = await call(meters.listMeters, as());
    out.deleteInUse = await call(meters.deleteMeter, as({ params: { meterId: 'successful-calls' } }));

    // --- pricing never changes once a plan exists ---------------------------
    out.pricingOnExistingPlan = await call(plans.putSubscriptionPlans, as({ body: { id: 'gold', displayName: 'Gold',
        pricing: { pricingType: 'FLAT_RATE', currency: 'USD', billingPeriod: 'MONTH', flatAmount: '10' } } }));
    out.pricingOnExistingInBulk = await call(plans.putSubscriptionPlans, as({ body: [
        { id: 'platinum', displayName: 'Platinum', pricing: goldPricing },
        { id: 'silver', displayName: 'Silver', pricing: goldPricing },
    ] }));
    out.platinumRows = count('SELECT COUNT(*) AS n FROM subscription_plans WHERE handle = ?', 'platinum');
    out.updatedWithoutPricing = await call(plans.putSubscriptionPlans, as({ body: { id: 'gold', displayName: 'Gold plan',
        description: 'Pay per call' } }));
    out.updatedInBulkWithoutPricing = await call(plans.putSubscriptionPlans, as({ body: [
        { id: 'gold', displayName: 'Gold' }, { id: 'silver', displayName: 'Silver' },
    ] }));
    out.pricingAfterUpdates = await call(pricing.getPlanPricing, as({ params: { planId: 'gold' } }));
    out.silverAfterUpdates = await call(pricing.getPlanPricing, as({ params: { planId: 'silver' } }));

    // --- deleting a plan deletes its pricing --------------------------------
    const goldUuid = db.prepare('SELECT uuid FROM subscription_plans WHERE handle = ?').get('gold').uuid;
    out.goldDeleted = await call(plans.deleteSubscriptionPlan, as({ params: { planId: 'gold' } }));
    // The delete answers before its transaction commits; this next call waits for it.
    out.pricingAfterDelete = await call(pricing.getPlanPricing, as({ params: { planId: 'gold' } }));
    out.goldPricingRows = {
        pricing: count('SELECT COUNT(*) AS n FROM subscription_plan_pricing WHERE subscription_plan_uuid = ?', goldUuid),
        prices: count('SELECT COUNT(*) AS n FROM subscription_plan_pricing_usage WHERE subscription_plan_uuid = ?', goldUuid),
    };
    out.lockedBySilver = await call(meters.updateMeter, as({
        params: { meterId: 'successful-calls' }, body: { ...calls, displayName: 'Billed calls' },
    }));
    out.deleteInUseByOne = await call(meters.deleteMeter, as({ params: { meterId: 'successful-calls' } }));
    out.silverDeleted = await call(plans.deleteSubscriptionPlan, as({ params: { planId: 'silver' } }));
    out.changedAfterPlansDeleted = await call(meters.updateMeter, as({
        params: { meterId: 'successful-calls' },
        body: { ...calls, displayName: 'Billed calls', filterFields: [{ field: 'HTTP_METHOD' }, { field: 'API_TYPE' }] },
    }));
    out.deleted = await call(meters.deleteMeter, as({ params: { meterId: 'successful-calls' } }));
    out.deletedAgain = await call(meters.deleteMeter, as({ params: { meterId: 'successful-calls' } }));

    out.events = db.prepare('SELECT type, aggregate_type, payload FROM events ORDER BY rowid').all()
        .map((e) => ({ type: e.type, aggregateType: e.aggregate_type, payload: JSON.parse(e.payload) }));

    // --- event keys the administrator writes ----------------------------------
    const lastEvent = () => JSON.parse(db.prepare('SELECT payload FROM events ORDER BY rowid DESC LIMIT 1').get().payload);
    const inputTokens = { id: 'input-tokens', displayName: 'Input tokens', type: 'TOKENS', typeKey: ' usage.in ',
        filterFields: [{ field: 'API_TYPE', eventKey: 'api.kind' }, { field: 'HTTP_STATUS_CODE' }] };
    out.customKeys = await call(meters.createMeter, as({ body: inputTokens }));
    out.customKeysEvent = lastEvent();
    out.customKeysRow = db.prepare('SELECT type_key, CAST(filter_fields AS TEXT) AS filter_fields FROM meters WHERE handle = ?')
        .get('input-tokens');
    out.tokensWithoutKey = await call(meters.createMeter, as({ body: { ...inputTokens, id: 'no-key', typeKey: undefined } }));
    out.keyOnRequestMeter = await call(meters.createMeter, as({ body: { ...calls, id: 'request-key',
        typeKey: 'usage.in' } }));
    out.oldEventProperty = await call(meters.createMeter, as({ body: { ...inputTokens, id: 'old-property',
        eventProperty: 'INPUT_TOKENS' } }));
    out.spacedFilterKey = await call(meters.createMeter, as({ body: { ...inputTokens, id: 'spaced-key',
        filterFields: [{ field: 'API_TYPE', eventKey: 'api kind' }] } }));
    out.keysUpdated = await call(meters.updateMeter, as({ params: { meterId: 'input-tokens' }, body: { ...inputTokens,
        typeKey: 'usage.out', filterFields: [{ field: 'API_TYPE', eventKey: '' }, { field: 'RESOURCE', eventKey: 'path' }] } }));
    out.keysUpdatedEvent = lastEvent();

    // --- all four fields: a meter that lists them and a price that filters on each ---
    out.allFieldsMeter = await call(meters.createMeter, as({ body: { id: 'all-fields', displayName: 'All fields',
        type: 'REQUEST_COUNT', filterFields: ['HTTP_STATUS_CODE', 'HTTP_METHOD', 'RESOURCE', 'API_TYPE']
            .map((field) => ({ field })) } }));
    out.allFieldsPlan = await call(plans.addSubscriptionPlans, as({ body: { id: 'all-fields', displayName: 'All fields',
        pricing: { pricingType: 'USAGE_BASED', currency: 'USD', billingPeriod: 'MONTH', prices: [{
            pricingModel: 'PER_UNIT', unitAmount: '0.01', meterId: 'all-fields', filters: [
                { field: 'HTTP_STATUS_CODE', operator: 'IN', values: ['200', '201'] },
                { field: 'HTTP_METHOD', operator: 'NOT_EQUALS', values: ['DELETE'] },
                { field: 'RESOURCE', operator: 'STARTS_WITH', values: ['/orders'] },
                { field: 'API_TYPE', operator: 'IN', values: ['RestApi', 'LlmProvider'] },
            ] }] } } }));

    // --- Token fields and conditions -----------------------------------------
    const PROMPT = 'metadata.aiTokenUsage.promptTokens';
    const TOTAL = 'metadata.aiTokenUsage.totalTokens';
    out.tokenMeter = await call(meters.createMeter, as({ body: { id: 'prompt-tokens', displayName: 'Prompt tokens',
        type: 'TOKENS', typeKey: PROMPT,
        filterFields: [{ field: 'TOKEN', eventKey: TOTAL }, { field: 'API_TYPE' }] } }));
    out.tokenMeterEvent = lastEvent();
    out.tokenFieldOnRowKey = await call(meters.createMeter, as({ body: { id: 'row-key-twice', displayName: 'Row key twice',
        type: 'TOKENS', typeKey: PROMPT, filterFields: [{ field: 'TOKEN', eventKey: PROMPT }] } }));
    out.tokenFieldWithoutKey = await call(meters.createMeter, as({ body: { id: 'no-token-key', displayName: 'No token key',
        type: 'REQUEST_COUNT', filterFields: [{ field: 'TOKEN' }] } }));
    const condition = (operator, value, eventKey = PROMPT) => ({ field: 'TOKEN', eventKey, operator, values: [value] });
    const tokenPlan = (id, prices) => call(plans.addSubscriptionPlans, as({ body: { id, displayName: id, pricing: {
        pricingType: 'USAGE_BASED', currency: 'USD', billingPeriod: 'MONTH', prices } } }));
    const tokenPrice = (unitAmount, filters) => ({ pricingModel: 'PER_UNIT', unitAmount, unitQuantity: 1000000, meterId: 'prompt-tokens', filters });
    // Up to 333 prompt tokens at one price, above at another; both only when the call used any tokens at all.
    out.tokenTiers = await tokenPlan('token-tiers', [
        tokenPrice('1.00', [condition('LESS_THAN_OR_EQUAL', '0333'), condition('GREATER_THAN', '0', TOTAL)]),
        tokenPrice('2.00', [condition('GREATER_THAN', '333'), condition('GREATER_THAN', '0', TOTAL)]),
    ]);
    out.tokenOverlap = await tokenPlan('token-overlap', [
        tokenPrice('1.00', [condition('GREATER_THAN', '100')]), tokenPrice('2.00', [condition('GREATER_THAN', '333')])]);
    out.tokenUnknownKey = await tokenPlan('token-unknown', [
        tokenPrice('1.00', [condition('GREATER_THAN', '1', 'metadata.aiTokenUsage.completionTokens')])]);
    out.tokenImpossible = await tokenPlan('token-impossible', [
        tokenPrice('1.00', [condition('GREATER_THAN', '500'), condition('LESS_THAN', '100')])]);
    // Read once the plan's transaction has committed (the later calls waited for it).
    out.tokenTiersEvent = db.prepare("SELECT payload FROM events WHERE type = 'subscription_plan.created' ORDER BY rowid").all()
        .map((e) => JSON.parse(e.payload)).find((e) => e.handle === 'token-tiers').pricing;

    // --- plans sent together with their pricing (how the Platform API sends a new paid plan) ---
    await call(meters.createMeter, as({ body: { displayName: 'Bundle calls', type: 'REQUEST_COUNT' } }));
    const eventsBefore = count('SELECT COUNT(*) AS n FROM events');
    const bundle = { id: 'bundle', displayName: 'Bundle', refId: 'bundle',
        pricing: { pricingType: 'USAGE_BASED', currency: 'USD',
            prices: [{ pricingModel: 'PER_UNIT', unitAmount: '0.02', meterId: 'bundle-calls' }] } };
    out.planWithPricing = await call(plans.putSubscriptionPlans, as({ body: [bundle] }));
    out.planWithPricingGet = await call(pricing.getPlanPricing, as({ params: { planId: 'bundle' } }));
    out.planWithPricingEvents = db.prepare('SELECT type FROM events ORDER BY rowid').all().slice(eventsBefore).map((e) => e.type);
    out.planWithBadPricing = await call(plans.putSubscriptionPlans, as({ body: [{ id: 'broken', displayName: 'Broken',
        pricing: { pricingType: 'USAGE_BASED', currency: 'USD',
            prices: [{ pricingModel: 'PER_UNIT', unitAmount: '0.02', meterId: 'no-such-meter' }] } }] }));
    out.brokenPlanRows = count('SELECT COUNT(*) AS n FROM subscription_plans WHERE handle = ?', 'broken');
    out.flatPlan = await call(plans.putSubscriptionPlans, as({ body: [{ id: 'monthly', displayName: 'Monthly', refId: 'monthly',
        pricing: { pricingType: 'FLAT_RATE', currency: 'USD', billingPeriod: 'MONTH', flatAmount: '20.00' } }] }));
    out.flatPlanGet = await call(pricing.getPlanPricing, as({ params: { planId: 'monthly' } }));
    out.createdInBulk = await call(plans.addSubscriptionPlans, as({ body: [
        { id: 'yearly', displayName: 'Yearly', pricing: { pricingType: 'FLAT_RATE', currency: 'USD', billingPeriod: 'YEAR',
            flatAmount: '200.00' } },
        { id: 'trial', displayName: 'Trial' },
    ] }));
    out.yearlyGet = await call(pricing.getPlanPricing, as({ params: { planId: 'yearly' } }));

    // --- the Platform API's own changes are not sent back to it as events ---
    const platformApi = (extra) => as({ ...extra, auth: { mode: 'shared-key', userId: null } });
    const quietBefore = count('SELECT COUNT(*) AS n FROM events');
    out.quietMeter = await call(meters.createMeter, platformApi({ body: { id: 'quiet-calls', displayName: 'Quiet calls', type: 'REQUEST_COUNT' } }));
    out.quietPlan = await call(plans.putSubscriptionPlans, platformApi({ body: [{ id: 'quiet', displayName: 'Quiet', refId: 'quiet',
        pricing: { pricingType: 'USAGE_BASED', currency: 'USD', prices: [{ pricingModel: 'PER_UNIT', unitAmount: '0.02', meterId: 'quiet-calls' }] } }] }));
    out.quietPlanDeleted = await call(plans.deleteSubscriptionPlan, platformApi({ params: { planId: 'quiet' } }));
    out.quietMeterDeleted = await call(meters.deleteMeter, platformApi({ params: { meterId: 'quiet-calls' } }));
    out.quietEvents = db.prepare('SELECT type FROM events ORDER BY rowid').all().slice(quietBefore).map((e) => e.type);

    // --- a plan an API offers is not deleted ---
    await call(plans.putSubscriptionPlans, as({ body: [{ id: 'offered', displayName: 'Offered' }] }));
    const offered = db.prepare('SELECT uuid, portal_id FROM subscription_plans WHERE handle = ?').get('offered');
    db.prepare("INSERT INTO api_metadata (uuid, org_uuid, portal_id, name, status, version, type, handle, ref_id, created_by, updated_by) VALUES ('api-offer', ?, ?, 'Offer API', 'PUBLISHED', 'v1', 'REST', 'offer-api', 'offer-api', 'test', 'test')")
        .run(${JSON.stringify(ORG)}, offered.portal_id);
    db.prepare("INSERT INTO api_subscription_plan_mappings (uuid, api_uuid, plan_uuid, portal_id, created_by) VALUES ('map-offer', 'api-offer', ?, ?, 'test')")
        .run(offered.uuid, offered.portal_id);
    out.offeredDelete = await call(plans.deleteSubscriptionPlan, as({ params: { planId: 'offered' } }));
    out.offeredRows = count('SELECT COUNT(*) AS n FROM subscription_plans WHERE handle = ?', 'offered');

    // --- a new plan's handle is at most 40 characters, the most the Platform API keeps ---
    // A plan save answers before its transaction commits: let it finish before writing to the database here.
    const settled = () => new Promise((resolve) => setTimeout(resolve, 100));
    const long = 'h'.repeat(41);
    out.longHandle = await call(plans.addSubscriptionPlans, as({ body: { id: long, displayName: 'Long' } }));
    out.longHandlePut = await call(plans.putSubscriptionPlans, as({ body: { id: long, displayName: 'Long' } }));
    out.longHandleInBulk = await call(plans.putSubscriptionPlans, as({ body: [{ id: 'fine-handle', displayName: 'Fine' },
        { id: long, displayName: 'Long' }] }));
    out.longHandleRows = count("SELECT COUNT(*) AS n FROM subscription_plans WHERE handle IN (?, 'fine-handle')", long);
    out.handle40 = await call(plans.putSubscriptionPlans, as({ body: { id: 'h'.repeat(40), displayName: 'Forty' } }));
    await settled();
    db.prepare("INSERT INTO subscription_plans (uuid, handle, display_name, org_uuid, portal_id, created_by, updated_by) VALUES ('plan-old-long', ?, 'Old long', ?, ?, 'test', 'test')")
        .run('o'.repeat(50), ${JSON.stringify(ORG)}, offered.portal_id);
    out.oldLongRenamed = await call(plans.putSubscriptionPlans, as({ body: { id: 'o'.repeat(50), displayName: 'Old long renamed' } }));

    // --- a plan stays on an API while it has subscriptions to it ---
    await call(plans.putSubscriptionPlans, as({ body: [{ id: 'kept', displayName: 'Kept' }, { id: 'subscribed', displayName: 'Subscribed' }] }));
    await settled();
    const planUuid = (handle) => db.prepare('SELECT uuid FROM subscription_plans WHERE handle = ?').get(handle).uuid;
    db.prepare("INSERT INTO api_metadata (uuid, org_uuid, portal_id, name, status, version, type, handle, ref_id, created_by, updated_by) VALUES ('api-sub', ?, ?, 'Sub API', 'PUBLISHED', 'v1', 'RestApi', 'sub-api', 'sub-api', 'test', 'test')")
        .run(${JSON.stringify(ORG)}, offered.portal_id);
    for (const handle of ['kept', 'subscribed']) {
        db.prepare("INSERT INTO api_subscription_plan_mappings (uuid, api_uuid, plan_uuid, portal_id, created_by) VALUES (?, 'api-sub', ?, ?, 'test')")
            .run('map-' + handle, planUuid(handle), offered.portal_id);
    }
    db.prepare("INSERT INTO subscriptions (uuid, created_by, api_uuid, plan_uuid, org_uuid, portal_id, token, updated_by) VALUES ('sub-1', 'user-1', 'api-sub', ?, ?, ?, 'token-1', 'user-1')")
        .run(planUuid('subscribed'), ${JSON.stringify(ORG)}, offered.portal_id);
    const offerOnly = (handles) => call(plans.updateAPIMetadata, as({ params: { apiId: 'sub-api' }, files: {}, body: { metadata: JSON.stringify({
        name: 'Sub API', version: 'v1', type: 'RestApi', endPoints: { productionURL: 'https://sub.example.com', sandboxURL: 'https://sub.example.com' },
        subscriptionPlans: handles.map((id) => ({ id })) }) } }));
    const offeredOnSubApi = () => db.prepare("SELECT sp.handle FROM api_subscription_plan_mappings m JOIN subscription_plans sp ON sp.uuid = m.plan_uuid WHERE m.api_uuid = 'api-sub' ORDER BY sp.handle").all().map((r) => r.handle);
    out.subscribedTakenOff = await offerOnly(['kept']);
    out.offeredAfterRefusal = offeredOnSubApi();
    db.prepare("DELETE FROM subscriptions WHERE uuid = 'sub-1'").run();
    out.endedTakenOff = await offerOnly(['kept']);
    out.offeredAfterEnd = offeredOnSubApi();

    process.stdout.write('\\nPROBE_JSON:' + JSON.stringify(out) + '\\n');
    process.exit(0);
})();
`;

let _results;
function results() {
    if (_results) return _results;
    tmpDir = tmpDir || fs.mkdtempSync(path.join(os.tmpdir(), 'ap-billing-'));
    const dbFile = path.join(tmpDir, 'portal.db');

    const seed = path.join(tmpDir, 'seed.js');
    fs.writeFileSync(seed, `
        const fs = require('fs');
        const Database = require(${JSON.stringify(SQLITE)});
        const db = new Database(${JSON.stringify(dbFile)});
        db.exec(fs.readFileSync(${JSON.stringify(SCHEMA)}, 'utf8'));
        db.prepare(\`INSERT INTO organizations
            (uuid, portal_id, display_name, handle, idp_ref_id, configuration, created_by, updated_by)
            VALUES (?,?,?,?,?,?,?,?)\`)
            .run(${JSON.stringify(ORG)}, ${JSON.stringify(PORTAL)}, 'Default', 'default', 'idp-1', '{}', 'test', 'test');
    `);
    const seeded = spawnSync(process.execPath, [seed], { cwd: PROJECT_ROOT, encoding: 'utf8' });
    assert.equal(seeded.status, 0, `seeding failed: ${seeded.stderr}`);

    const cfgFile = path.join(tmpDir, 'config.toml');
    fs.writeFileSync(cfgFile, CONFIG(dbFile));
    const runner = path.join(tmpDir, 'probe.js');
    fs.writeFileSync(runner, PROBE.replaceAll(JSON.stringify('DB_FILE'), JSON.stringify(dbFile)));

    const run = spawnSync(process.execPath, [runner, '--config', cfgFile], {
        cwd: PROJECT_ROOT,
        encoding: 'utf8',
        env: { PATH: process.env.PATH, HOME: process.env.HOME },
    });
    assert.equal(run.status, 0, `probe failed: ${run.stderr}`);
    const line = run.stdout.split('\n').find((l) => l.startsWith('PROBE_JSON:'));
    assert.ok(line, `no PROBE_JSON in stdout: ${run.stdout}\n${run.stderr}`);
    _results = JSON.parse(line.slice('PROBE_JSON:'.length));
    return _results;
}

// The message of a refusal: handleError puts a validation message in errors[0].
const refusal = (r) => (r.body.errors && r.body.errors[0] ? r.body.errors[0].message : r.body.message);

// ---------------------------------------------------------------------------
// meters
// ---------------------------------------------------------------------------

test('createMeter stores a meter under an id derived from its name and answers 201 with its location', () => {
    const r = results().created;
    assert.equal(r.status, 201);
    assert.equal(r.body.id, 'successful-calls');
    assert.equal(r.headers.Location, '/api-portal/api/v0.9/meters/successful-calls');
    assert.equal(r.body.type, 'REQUEST_COUNT');
    assert.equal(r.body.typeKey, null);
    assert.ok(!('eventProperty' in r.body));
    assert.equal(r.body.conditions, undefined);
    assert.equal(r.body.planCount, 0);
});

test('createMeter gives a derived id that is taken a suffix, but refuses a given id that is taken', () => {
    assert.equal(results().derivedTaken.status, 201);
    assert.equal(results().derivedTaken.body.typeKey, 'metadata.aiTokenUsage.totalTokens');
    assert.match(results().derivedTaken.body.id, /^successful-calls-[0-9a-f]{6}$/);
    assert.equal(results().duplicateId.status, 409);
});

test('createMeter refuses an invalid meter with 400', () => {
    assert.equal(results().invalid.status, 400);
    assert.equal(results().unknownFilterField.status, 400);
    assert.match(results().unknownFilterField.body.message, /filterFields: unknown field COLOR/);
    assert.equal(results().filterFieldTwice.status, 400);
    assert.match(results().filterFieldTwice.body.message, /filterFields: API_TYPE is listed twice/);
    assert.equal(results().filterFieldWithOperator.status, 400);
    assert.equal(results().filterFieldWithOperator.body.message, 'Each entry of filterFields has only a field and an eventKey');
});

test('getMeter returns a meter by id, with the event keys of what it counts and filters on, and 404 for an unknown one', () => {
    assert.equal(results().get.status, 200);
    assert.equal(results().get.body.type, 'REQUEST_COUNT');
    assert.equal(results().get.body.typeKey, null);
    // A filter field sent without a key reads its default.
    assert.deepEqual(results().get.body.filterFields, [{ field: 'HTTP_METHOD', eventKey: 'request.verb' }]);
    assert.equal(results().getMissing.status, 404);
});

test('a meter stores the event keys it is given, defaults for the rest, and sends them to the Platform API', () => {
    const r = results().customKeys;
    assert.equal(r.status, 201);
    assert.equal(r.body.typeKey, 'usage.in');
    assert.deepEqual(r.body.filterFields, [
        { field: 'API_TYPE', eventKey: 'api.kind' },
        { field: 'HTTP_STATUS_CODE', eventKey: 'response.status' },
    ]);
    assert.deepEqual(results().customKeysRow, {
        type_key: 'usage.in',
        filter_fields: '[{"field":"API_TYPE","eventKey":"api.kind"},{"field":"HTTP_STATUS_CODE","eventKey":"response.status"}]',
    });
    assert.deepEqual(results().customKeysEvent, {
        handle: 'input-tokens',
        display_name: 'Input tokens',
        description: null,
        type: 'TOKENS',
        type_key: 'usage.in',
        filter_fields: [{ field: 'API_TYPE', event_key: 'api.kind' }, { field: 'HTTP_STATUS_CODE', event_key: 'response.status' }],
    });
});

test('the event keys of a meter no plan prices can change; an emptied filter key goes back to its default', () => {
    const r = results().keysUpdated;
    assert.equal(r.status, 200);
    assert.equal(r.body.typeKey, 'usage.out');
    assert.deepEqual(r.body.filterFields, [
        { field: 'API_TYPE', eventKey: 'metadata.apiType' },
        { field: 'RESOURCE', eventKey: 'path' },
    ]);
    assert.equal(results().keysUpdatedEvent.type_key, 'usage.out');
    assert.deepEqual(results().keysUpdatedEvent.filter_fields, [
        { field: 'API_TYPE', event_key: 'metadata.apiType' },
        { field: 'RESOURCE', event_key: 'path' },
    ]);
});

test('a meter may list all four fields, and a price may filter on each of them', () => {
    const m = results().allFieldsMeter;
    assert.equal(m.status, 201, JSON.stringify(m.body));
    assert.deepEqual(m.body.filterFields, [
        { field: 'HTTP_STATUS_CODE', eventKey: 'response.status' },
        { field: 'HTTP_METHOD', eventKey: 'request.verb' },
        { field: 'RESOURCE', eventKey: 'metadata.apiResourceTemplate' },
        { field: 'API_TYPE', eventKey: 'metadata.apiType' },
    ]);
    const p = results().allFieldsPlan;
    assert.equal(p.status, 201, JSON.stringify(p.body));
    assert.deepEqual(p.body.pricing.prices[0].filters.map((f) => f.field),
        ['HTTP_STATUS_CODE', 'HTTP_METHOD', 'RESOURCE', 'API_TYPE']);
});

test('a meter may list Token fields by key, sent to the Platform API with their keys', () => {
    const m = results().tokenMeter;
    assert.equal(m.status, 201, JSON.stringify(m.body));
    assert.deepEqual(m.body.filterFields, [
        { field: 'TOKEN', eventKey: 'metadata.aiTokenUsage.totalTokens' },
        { field: 'API_TYPE', eventKey: 'metadata.apiType' },
    ]);
    assert.deepEqual(results().tokenMeterEvent.filter_fields, [
        { field: 'TOKEN', event_key: 'metadata.aiTokenUsage.totalTokens' },
        { field: 'API_TYPE', event_key: 'metadata.apiType' },
    ]);
    assert.equal(results().tokenFieldOnRowKey.status, 400);
    assert.equal(results().tokenFieldOnRowKey.body.message, 'The key metadata.aiTokenUsage.promptTokens is already the meter\'s Token key');
    assert.equal(results().tokenFieldWithoutKey.status, 400);
    assert.equal(results().tokenFieldWithoutKey.body.message, 'A Token field needs an eventKey');
});

test('prices compare Token keys: stored as plain numbers, named, and sent with each condition\'s key', () => {
    const r = results().tokenTiers;
    assert.equal(r.status, 201, JSON.stringify(r.body));
    // The prices are listed by name; each is found by its first condition here.
    const upTo333 = r.body.pricing.prices.find((p) => p.filters[0].operator === 'LESS_THAN_OR_EQUAL');
    assert.deepEqual(upTo333.filters, [
        { field: 'TOKEN', eventKey: 'metadata.aiTokenUsage.promptTokens', operator: 'LESS_THAN_OR_EQUAL', values: ['333'] },
        { field: 'TOKEN', eventKey: 'metadata.aiTokenUsage.totalTokens', operator: 'GREATER_THAN', values: ['0'] },
    ]);
    const byFirstOperator = (filters) => [...filters].sort((a, b) => a[0].operator.localeCompare(b[0].operator));
    assert.deepEqual(byFirstOperator(results().tokenTiersEvent.prices.map((p) => p.filters)), [
        [{ field: 'TOKEN', event_key: 'metadata.aiTokenUsage.promptTokens', operator: 'GREATER_THAN', values: ['333'] },
            { field: 'TOKEN', event_key: 'metadata.aiTokenUsage.totalTokens', operator: 'GREATER_THAN', values: ['0'] }],
        [{ field: 'TOKEN', event_key: 'metadata.aiTokenUsage.promptTokens', operator: 'LESS_THAN_OR_EQUAL', values: ['333'] },
            { field: 'TOKEN', event_key: 'metadata.aiTokenUsage.totalTokens', operator: 'GREATER_THAN', values: ['0'] }],
    ]);
    const refused = {
        tokenOverlap: 'Prices 1 and 2 both count the same requests of the meter prompt-tokens; give them different filters.',
        tokenUnknownKey: 'price 1: the meter has no Token field with the key metadata.aiTokenUsage.completionTokens',
        tokenImpossible: 'price 1: the Token conditions on metadata.aiTokenUsage.promptTokens can\'t all be true',
    };
    for (const [key, message] of Object.entries(refused)) {
        assert.equal(results()[key].status, 400, key);
        assert.equal(refusal(results()[key]), message, key);
    }
});

test('a Tokens meter needs its event key, a request meter takes none, and eventProperty is not a meter field', () => {
    const refused = {
        tokensWithoutKey: 'A TOKENS meter needs a typeKey',
        keyOnRequestMeter: 'typeKey applies only to a TOKENS meter',
        oldEventProperty: 'eventProperty is not a meter field',
        spacedFilterKey: 'The event key of API type must be 1 to 255 characters with no spaces',
    };
    for (const [key, message] of Object.entries(refused)) {
        assert.equal(results()[key].status, 400, key);
        assert.equal(results()[key].body.message, message, key);
    }
});

test('updateMeter changes a meter no plan prices, but never the id', () => {
    const r = results().updated;
    assert.equal(r.status, 200);
    assert.equal(r.body.description, 'Calls that succeeded');
    assert.equal(results().renamed.status, 400);
    assert.equal(results().updateMissing.status, 404);
});

// ---------------------------------------------------------------------------
// plans created with their pricing
// ---------------------------------------------------------------------------

test('a plan created without pricing is free', () => {
    assert.equal(results().freePlan.status, 201);
    assert.equal(results().freePlan.body.paid, false);
    assert.equal(results().pricingFree.status, 404);
    assert.equal(results().pricingFree.body.message, 'The plan is free: it has no pricing');
    assert.equal(results().pricingNoPlan.status, 404);
    assert.equal(results().pricingNoPlan.body.message, 'Subscription plan not found');
});

const price = (body) => ({
    pricingType: body.pricingType, currency: body.currency, billingPeriod: body.billingPeriod, flatAmount: body.flatAmount,
    prices: body.prices,
});
const GOLD = {
    pricingType: 'USAGE_BASED', currency: 'USD', billingPeriod: 'MONTH', flatAmount: null, prices: [{
        meterId: 'successful-calls', filters: [{ field: 'HTTP_METHOD', operator: 'EQUALS', values: ['GET'] }], pricingModel: 'PER_UNIT',
        unitAmount: '0.01', unitQuantity: 1, tiers: null,
    }],
};
// Its prices in the order a plan lists them, by name: GET before POST.
const SILVER = {
    pricingType: 'USAGE_BASED', currency: 'USD', billingPeriod: 'MONTH', flatAmount: null, prices: [GOLD.prices[0], {
        meterId: 'successful-calls', filters: [{ field: 'HTTP_METHOD', operator: 'EQUALS', values: ['POST'] }],
        pricingModel: 'GRADUATED', unitAmount: null, unitQuantity: null, tiers: [
            { upTo: 1000, unitAmount: '0', flatAmount: null },
            { upTo: null, unitAmount: '0.002', flatAmount: null },
        ],
    }],
};

test('a plan created with pricing (POST) is paid, and its pricing is stored as given, normalized', () => {
    const created = results().goldCreated;
    assert.equal(created.status, 201, JSON.stringify(created.body));
    assert.equal(created.body.paid, true);
    assert.equal(created.body.refId, 'gold');
    assert.deepEqual(price(created.body.pricing), GOLD);
    assert.equal(results().pricingGet.status, 200);
    assert.deepEqual(price(results().pricingGet.body), GOLD);
});

test('a PUT of a new plan creates it with its pricing; a plan may price a meter twice with different filters', () => {
    const created = results().silverCreated;
    assert.equal(created.status, 201, JSON.stringify(created.body));
    assert.equal(created.body.paid, true);
    assert.deepEqual(price(created.body.pricing), SILVER);
    assert.deepEqual(price(results().silverGet.body), SILVER);
});

test('a flat rate plan has one amount per period and no prices', () => {
    assert.equal(results().flatPlan.status, 201, JSON.stringify(results().flatPlan.body));
    const body = results().flatPlanGet.body;
    assert.deepEqual([body.pricingType, body.flatAmount, body.prices], ['FLAT_RATE', '20.00', []]);
});

test('a meter has no conditions', () => {
    assert.equal(results().withConditions.status, 400);
    assert.match(results().withConditions.body.message, /A meter has no conditions; each plan price gives its own filters/);
});

test('a plan may not price a meter so that one request is counted by two prices', () => {
    assert.equal(results().pricingOverlap.status, 400);
    assert.match(refusal(results().pricingOverlap),
        /Prices 1 and 2 both count the same requests of the meter successful-calls; give them different filters\./);
});

test('a price filters only on the fields its meter allows', () => {
    assert.equal(results().pricingFieldNotAllowed.status, 400);
    assert.match(refusal(results().pricingFieldNotAllowed),
        /^price 1: the meter successful-calls does not allow filtering on API_TYPE$/);
});

test('pricing naming an unknown meter is refused with 400', () => {
    assert.equal(results().pricingUnknownMeter.status, 400);
    assert.match(refusal(results().pricingUnknownMeter), /missing-meter/);
});

test('a paid plan with no amount is refused: a free plan is one without pricing', () => {
    assert.equal(results().pricingZeroFlat.status, 400);
    assert.equal(refusal(results().pricingZeroFlat), 'flatAmount must be more than 0. Create a free plan instead.');
});

test('a plan whose pricing is refused is not created', () => {
    assert.equal(results().refusedPlanRows, 0);
});

// ---------------------------------------------------------------------------
// a meter plans price
// ---------------------------------------------------------------------------

test('a meter one plan prices cannot change at all, its name included', () => {
    const r = results().renamedWhilePriced;
    assert.equal(r.status, 409);
    assert.equal(r.body.code, 'METER_IN_USE');
    assert.equal(r.body.message, 'The meter is priced by 1 plan, so it can\'t change.');
    assert.equal(results().deleteWhilePricedByGold.status, 409);
    assert.equal(results().deleteWhilePricedByGold.body.code, 'METER_IN_USE');
    assert.equal(results().deleteWhilePricedByGold.body.message, 'The meter is priced by 1 plan, so it can\'t be deleted.');
});

test('a meter several plans price cannot change either: not its description, its filter fields nor their keys', () => {
    for (const key of ['describedWhilePriced', 'filterFieldAddedWhilePriced', 'filterFieldRemovedWhilePriced',
        'eventKeyChangedWhilePriced']) {
        const r = results()[key];
        assert.equal(r.status, 409, key);
        assert.equal(r.body.code, 'METER_IN_USE', key);
        assert.equal(r.body.message, 'The meter is priced by 2 plans, so it can\'t change.', key);
    }
    const meter = results().meterWhilePriced.body;
    assert.equal(meter.displayName, 'Successful calls');
    assert.equal(meter.description, 'Calls that succeeded');
    assert.deepEqual(meter.filterFields, [{ field: 'HTTP_METHOD', eventKey: 'request.verb' }]);
});

test('a meter counts the plans that charge for it and cannot be deleted while they do', () => {
    const listed = results().listWithUsage.body.list.find((m) => m.id === 'successful-calls');
    assert.equal(listed.planCount, 2);
    assert.equal(listed.usedBy, undefined, 'a count, not the plans');
    assert.equal(results().deleteInUse.status, 409);
    assert.equal(results().deleteInUse.body.code, 'METER_IN_USE');
    assert.equal(results().deleteInUse.body.message, 'The meter is priced by 2 plans, so it can\'t be deleted.');
});

// ---------------------------------------------------------------------------
// pricing never changes once a plan exists
// ---------------------------------------------------------------------------

test('pricing sent for a plan that already exists is refused with 409 PLAN_PRICING_LOCKED', () => {
    const r = results().pricingOnExistingPlan;
    assert.equal(r.status, 409);
    assert.equal(r.body.code, 'PLAN_PRICING_LOCKED');
    assert.equal(r.body.message, 'The plan gold already exists; its pricing can\'t change once a plan is created.');
});

test('a bulk PUT with pricing for an existing plan is refused as a whole: the new plan in it is not created', () => {
    const r = results().pricingOnExistingInBulk;
    assert.equal(r.status, 409);
    assert.equal(r.body.code, 'PLAN_PRICING_LOCKED');
    assert.equal(r.body.message, 'The plan silver already exists; its pricing can\'t change once a plan is created.');
    assert.equal(results().platinumRows, 0);
});

test('an existing plan updated without pricing keeps its pricing', () => {
    const single = results().updatedWithoutPricing;
    assert.equal(single.status, 200, JSON.stringify(single.body));
    assert.equal(single.body.displayName, 'Gold plan');
    assert.equal(single.body.paid, true);
    assert.deepEqual(price(single.body.pricing), GOLD);
    const bulk = results().updatedInBulkWithoutPricing;
    assert.equal(bulk.status, 200, JSON.stringify(bulk.body));
    assert.deepEqual(bulk.body.map((p) => [p.id, p.displayName, p.paid]), [['gold', 'Gold', true], ['silver', 'Silver', true]]);
    assert.deepEqual(price(results().pricingAfterUpdates.body), GOLD);
    assert.deepEqual(price(results().silverAfterUpdates.body), SILVER);
});

// ---------------------------------------------------------------------------
// deleting a plan
// ---------------------------------------------------------------------------

test('deleting a plan deletes its pricing, and the meter is then priced by the plans left', () => {
    assert.equal(results().goldDeleted.status, 204);
    assert.deepEqual(results().goldPricingRows, { pricing: 0, prices: 0 });
    assert.equal(results().pricingAfterDelete.status, 404);
    const r = results().lockedBySilver;
    assert.equal(r.status, 409);
    assert.equal(r.body.message, 'The meter is priced by 1 plan, so it can\'t change.');
    assert.equal(results().deleteInUseByOne.status, 409);
    assert.equal(results().deleteInUseByOne.body.code, 'METER_IN_USE');
    assert.equal(results().deleteInUseByOne.body.message, 'The meter is priced by 1 plan, so it can\'t be deleted.');
});

test('once no plan prices a meter it can change again, and be deleted', () => {
    assert.equal(results().silverDeleted.status, 204);
    const r = results().changedAfterPlansDeleted;
    assert.equal(r.status, 200, JSON.stringify(r.body));
    assert.equal(r.body.displayName, 'Billed calls');
    assert.deepEqual(r.body.filterFields.map((f) => f.field), ['HTTP_METHOD', 'API_TYPE']);
    assert.equal(r.body.planCount, 0);
    assert.equal(results().deleted.status, 204);
    assert.equal(results().deletedAgain.status, 404);
});

// ---------------------------------------------------------------------------
// events for the Platform API
// ---------------------------------------------------------------------------

test('every successful change leaves exactly one event, in order; refused ones leave none', () => {
    assert.deepEqual(results().events.map((e) => e.type), [
        'meter.created',
        'meter.created',
        'meter.updated',
        'subscription_plan.created',
        // Plans created with pricing: one event each, which carries the pricing.
        'subscription_plan.created',
        'subscription_plan.created',
        'subscription_plan.updated',
        'subscription_plan.updated',
        'subscription_plan.updated',
        // A plan's pricing is deleted with the plan.
        'subscription_plan.deleted',
        'subscription_plan.deleted',
        'meter.updated',
        'meter.deleted',
    ]);
    assert.ok(!results().events.some((e) => e.type.startsWith('plan_pricing.')));
});

// The Platform API already has what it changed: it would only get its own change back.
test('the Platform API\'s own meter, plan and pricing changes are not sent back to it as events', () => {
    assert.equal(results().quietMeter.status, 201, JSON.stringify(results().quietMeter.body));
    assert.ok(results().quietPlan.status < 300, JSON.stringify(results().quietPlan.body));
    assert.equal(results().quietPlanDeleted.status, 204, JSON.stringify(results().quietPlanDeleted.body));
    assert.equal(results().quietMeterDeleted.status, 204, JSON.stringify(results().quietMeterDeleted.body));
    assert.deepEqual(results().quietEvents, []);
});

// Subscribers see a plan that an API offers on that API's page. This also covers a Platform API
// publish that timed out after its listing was saved: the plans the API offers are kept.
test('a plan an API offers is not deleted', () => {
    assert.equal(results().offeredDelete.status, 409);
    assert.equal(results().offeredDelete.body.code, 'PLAN_IN_USE');
    assert.equal(results().offeredDelete.body.message, 'The plan Offered is offered by 1 API. Remove it from that API before deleting it.');
    assert.equal(results().offeredRows, 1);
});

test('a new plan\'s id is at most 40 characters, the most the Platform API keeps; a plan that has a longer one keeps it', () => {
    const r = results();
    for (const refused of [r.longHandle, r.longHandlePut, r.longHandleInBulk]) {
        assert.equal(refused.status, 400, JSON.stringify(refused));
        assert.equal(refused.body.errors[0].message, 'id must be at most 40 characters');
    }
    assert.equal(r.longHandleRows, 0, 'nothing of a refused request is created, in bulk neither');
    assert.equal(r.handle40.status, 201, JSON.stringify(r.handle40));
    assert.equal(r.oldLongRenamed.status, 200, JSON.stringify(r.oldLongRenamed));
});

test('a plan is not removed from an API while it has subscriptions to it; once they have ended it is', () => {
    const r = results();
    assert.equal(r.subscribedTakenOff.status, 409, JSON.stringify(r.subscribedTakenOff));
    assert.equal(r.subscribedTakenOff.body.message, 'ERR_PLAN_SUB_EXIST', JSON.stringify(r.subscribedTakenOff.body));
    assert.equal(r.subscribedTakenOff.body.errors[0].message,
        'The plan Subscribed has 1 subscription to this API. It can be removed from the API once that subscription has ended.');
    assert.deepEqual(r.offeredAfterRefusal, ['kept', 'subscribed'], 'nothing changed');
    assert.equal(r.endedTakenOff.status, 200, JSON.stringify(r.endedTakenOff));
    assert.deepEqual(r.offeredAfterEnd, ['kept']);
});

test('meter events carry the full meter, and the delete only its handle', () => {
    const events = results().events;
    assert.deepEqual(events[2].payload, {
        handle: 'successful-calls',
        display_name: 'Successful calls',
        description: 'Calls that succeeded',
        type: 'REQUEST_COUNT',
        type_key: null,
        filter_fields: [{ field: 'HTTP_METHOD', event_key: 'request.verb' }],
    });
    assert.equal(events[2].aggregateType, 'meter');
    assert.deepEqual(events[12].payload, { handle: 'successful-calls' });
});

test('a plan\'s created event carries its pricing, naming each price\'s meter by handle; null for a free plan', () => {
    const events = results().events;
    assert.equal(events[3].payload.pricing, null);
    assert.equal(events[4].payload.handle, 'gold');
    assert.equal(events[4].aggregateType, 'subscription_plan');
    assert.deepEqual(events[4].payload.pricing, {
        pricing_type: 'USAGE_BASED',
        currency: 'USD',
        billing_period: 'MONTH',
        flat_amount: null,
        prices: [{
            meter_handle: 'successful-calls',
            filters: [{ field: 'HTTP_METHOD', operator: 'EQUALS', values: ['GET'] }],
            pricing_model: 'PER_UNIT',
            unit_amount: '0.01',
            unit_quantity: 1,
            tiers: null,
        }],
    });
    assert.equal(events[5].payload.handle, 'silver');
    assert.equal(events[5].payload.pricing.flat_amount, null);
    assert.equal(events[5].payload.pricing.prices[1].pricing_model, 'GRADUATED');
    assert.deepEqual(events[5].payload.pricing.prices[1].tiers, [
        { up_to: 1000, unit_amount: '0', flat_amount: null },
        { up_to: null, unit_amount: '0.002', flat_amount: null },
    ]);
    // An update carries the pricing the plan was created with, which never changes.
    const createdPricing = new Map(events.filter((e) => e.type === 'subscription_plan.created')
        .map((e) => [e.payload.handle, e.payload.pricing]));
    const updates = events.filter((x) => x.type === 'subscription_plan.updated');
    for (const e of updates) {
        assert.deepEqual(e.payload.pricing, createdPricing.get(e.payload.handle), e.payload.handle);
    }
    assert.ok(updates.some((e) => e.payload.pricing), 'a paid plan was updated');
    assert.deepEqual(events[9].payload, { handle: 'gold' });
});

// ---------------------------------------------------------------------------
// plans sent together with their pricing
// ---------------------------------------------------------------------------

test('a plan sent with its pricing in a bulk PUT gets both in one request, and one event that carries both', () => {
    assert.equal(results().planWithPricing.status, 201);
    assert.equal(results().planWithPricing.body[0].paid, true);
    assert.equal(results().planWithPricingGet.status, 200);
    assert.equal(results().planWithPricingGet.body.prices[0].pricingModel, 'PER_UNIT');
    assert.equal(results().planWithPricingGet.body.prices[0].meterId, 'bundle-calls');
    assert.deepEqual(results().planWithPricingEvents, ['subscription_plan.created']);
});

test('pricing the portal refuses in a bulk PUT creates neither the plan nor its pricing', () => {
    assert.equal(results().planWithBadPricing.status, 400);
    assert.match(JSON.stringify(results().planWithBadPricing.body), /no-such-meter/);
    assert.equal(results().brokenPlanRows, 0);
});

test('a bulk POST creates each plan with the pricing it is given, and leaves the others free', () => {
    const r = results().createdInBulk;
    assert.equal(r.status, 201, JSON.stringify(r.body));
    assert.deepEqual(r.body.map((p) => [p.id, p.paid]), [['yearly', true], ['trial', false]]);
    const body = results().yearlyGet.body;
    assert.deepEqual([body.pricingType, body.billingPeriod, body.flatAmount], ['FLAT_RATE', 'YEAR', '200.00']);
});
