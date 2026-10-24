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
 * The subscriber's Usage page: the billable usage of one of their usage based subscriptions over a
 * period, one series per usage price of the plan, per hour, day or week, and the charges so far in
 * the current billing period. The current billing period is shown whole: the hours or days still
 * to come are in the series with no value yet.
 *
 * Both usage and charges come through the billing provider interface. Usage is counted by the
 * connector the same way its billing meter counts it (with Moesif, the meter's own filter), so the
 * page shows what is billed. The charges come from the payment provider's estimate of the next
 * invoice. Both lag the gateway by a few minutes, which the page says.
 */

const subDao = require('../../dao/subscriptionDao');
const planPricingDao = require('../../dao/planPricingDao');
const meterDao = require('../../dao/meterDao');
const billingCheckoutDao = require('../../dao/billingCheckoutDao');
const billingSubscriptionDao = require('../../dao/billingSubscriptionDao');
const billingApiPlanDao = require('../../dao/billingApiPlanDao');
const subscriptionBilling = require('./subscriptionBillingService');
const billingProvider = require('../../billing/billingProvider');
const { countedFilters } = require('./planPricingValidation');
const { meterUnit } = require('../../utils/meterLabels');
const util = require('../../utils/util');
const logger = require('../../config/logger');

const PERIODS = Object.freeze(['CURRENT', 'PREVIOUS', 'LAST_30_DAYS', 'LAST_7_DAYS', 'LAST_24_HOURS']);
// The bins a series can be counted in; weeks start on Monday (UTC).
const INTERVALS = Object.freeze(['HOUR', 'DAY', 'WEEK']);
const HOUR_MS = 60 * 60 * 1000;
const DAY_MS = 24 * HOUR_MS;
const INTERVAL_MS = Object.freeze({ HOUR: HOUR_MS, DAY: DAY_MS, WEEK: 7 * DAY_MS });
// A bin is offered when the range has at most this many of it, and more than one.
const MAX_BUCKETS = 800;

/**
 * The time range of a period: usage is read `from` - `to` (now at the latest), and the series runs
 * `from` - `end`, which for the current billing period is its end, still to come. CURRENT and
 * PREVIOUS are billing periods.
 */
function periodRange(period, billing, now = new Date()) {
    if (period === 'LAST_24_HOURS') {
        // 24 whole hours: the current one and the 23 before it.
        return { from: new Date(bucketStart(now, 'HOUR') - 23 * HOUR_MS), to: now, end: now };
    }
    if (period === 'LAST_7_DAYS') {
        // 7 whole days: today and the 6 before it.
        return { from: new Date(bucketStart(now, 'DAY') - 6 * DAY_MS), to: now, end: now };
    }
    const start = billing.current_period_start ? new Date(billing.current_period_start) : null;
    const end = billing.current_period_end ? new Date(billing.current_period_end) : null;
    if (period !== 'LAST_30_DAYS' && start && end && end > start) {
        if (period === 'PREVIOUS') {
            return { from: previousPeriodStart(start, end), to: start, end: start };
        }
        return { from: start, to: end < now ? end : now, end };
    }
    return { from: new Date(now.getTime() - 30 * DAY_MS), to: now, end: now };
}

/**
 * The start of the billing period before the one `start` - `end`: one billing interval earlier (a
 * month, or a year for a period longer than 300 days), at the same time of day. Months differ in
 * length, so the previous period may not be as long as the current one.
 *
 * The day of the month is the billing day: the later of the current period's start and end days.
 * The payment provider moves a billing day of 29-31 back to the last day of a shorter month and
 * returns to it after (31 January, 28 February, 31 March). In a month without the billing day, the
 * month's last day is used.
 */
function previousPeriodStart(start, end) {
    const months = end.getTime() - start.getTime() > 300 * DAY_MS ? 12 : 1;
    const billingDay = Math.max(start.getUTCDate(), end.getUTCDate());
    const first = new Date(Date.UTC(start.getUTCFullYear(), start.getUTCMonth() - months, 1, start.getUTCHours(),
        start.getUTCMinutes(), start.getUTCSeconds(), start.getUTCMilliseconds()));
    const lastDay = new Date(Date.UTC(first.getUTCFullYear(), first.getUTCMonth() + 1, 0)).getUTCDate();
    first.setUTCDate(Math.min(billingDay, lastDay));
    return first;
}

/** The bins the range can be shown in: more than one of each, and at most MAX_BUCKETS. */
function seriesIntervals(range) {
    const span = range.end.getTime() - range.from.getTime();
    return INTERVALS.filter((i) => span > INTERVAL_MS[i] && span / INTERVAL_MS[i] <= MAX_BUCKETS);
}

/** The bin a range is shown in unless one is chosen: hours up to two days, days up to 120, then weeks. */
function seriesInterval(range) {
    const span = range.end.getTime() - range.from.getTime();
    if (span <= 2 * DAY_MS) return 'HOUR';
    return span <= 120 * DAY_MS ? 'DAY' : 'WEEK';
}

/** The start (UTC) of the hour, day or week (from Monday) `time` falls in. */
function bucketStart(time, interval) {
    const t = new Date(time);
    if (interval === 'HOUR') return Date.UTC(t.getUTCFullYear(), t.getUTCMonth(), t.getUTCDate(), t.getUTCHours());
    const day = Date.UTC(t.getUTCFullYear(), t.getUTCMonth(), t.getUTCDate());
    return interval === 'WEEK' ? day - ((t.getUTCDay() + 6) % 7) * DAY_MS : day;
}

function nextBucket(t, interval) {
    if (interval === 'HOUR') return t + HOUR_MS;
    const d = new Date(t);
    return Date.UTC(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate() + (interval === 'WEEK' ? 7 : 1));
}

/**
 * Every hour, day or week of the range, oldest first, as [{ start, value }]: the provider's count
 * where it has one, 0 where it has none, and null for the ones still to come.
 */
function fillSeries(counted, range, interval, now = new Date()) {
    const values = new Map();
    for (const b of counted) {
        const key = bucketStart(b.start, interval);
        values.set(key, (values.get(key) || 0) + b.value);
    }
    const series = [];
    for (let t = bucketStart(range.from, interval); t < range.end.getTime(); t = nextBucket(t, interval)) {
        series.push({ start: new Date(t).toISOString(), value: t > now.getTime() ? null : (values.get(t) || 0) });
    }
    return series;
}

function subscriptionSummary(sub) {
    const api = sub.api_metadata || {};
    const plan = sub.subscription_plan || {};
    return {
        id: sub.uuid,
        apiName: api.name || '',
        apiVersion: api.version || '',
        planName: plan.display_name || '',
    };
}

/**
 * The subscription's billable usage of one price per hour, day or week, as the connector counts it
 * with the price's meter and values. The subscriber sees the price's name and what is counted
 * ("request").
 */
async function priceUsage(usage, price, meter, ids, range, interval) {
    const result = { name: price.name, unit: meterUnit(meter), total: null, buckets: null };
    let counted;
    try {
        counted = await usage.series(meter, ids, { from: range.from, to: range.to, interval });
    } catch (err) {
        if (err instanceof billingProvider.CatalogInputError) return result;
        throw err;
    }
    result.buckets = fillSeries(counted, range, interval);
    result.total = result.buckets.reduce((sum, b) => sum + (b.value || 0), 0);
    return result;
}

/** GET /usage?subscriptionId=&period= */
const getUsage = async (req, res) => {
    const orgId = req.orgId;
    try {
        const period = req.query.period || 'CURRENT';
        if (!PERIODS.includes(period)) {
            return util.sendError(res, 400, `period must be one of ${PERIODS.join(', ')}`);
        }
        const requestedInterval = req.query.interval;
        if (requestedInterval !== undefined && !INTERVALS.includes(requestedInterval)) {
            return util.sendError(res, 400, `interval must be one of ${INTERVALS.join(', ')}`);
        }
        const subs = await subDao.list(orgId, { createdBy: util.resolveActor(req) });
        const billingRows = await billingSubscriptionDao.listBySubscriptions(subs.map((s) => s.uuid));
        // Only a usage-based plan has billable usage: flat rate subscriptions are not listed.
        const pricingOf = new Map();
        for (const s of subs.filter((x) => billingRows.has(x.uuid) && x.subscription_plan)) {
            pricingOf.set(s.uuid, await planPricingDao.getByPlan(orgId, s.subscription_plan.uuid));
        }
        const paid = subs.filter((s) => (pricingOf.get(s.uuid) || {}).pricing_type === 'USAGE_BASED');
        const body = { subscriptions: paid.map(subscriptionSummary), subscriptionId: null, period, from: null, to: null,
            status: null, usage: null, estimate: null, usageAvailable: true };
        const requested = req.query.subscriptionId;
        // Another user's subscription is not found, also for a caller with no usage-based plan.
        if (requested && !subs.some((s) => s.uuid === requested)) {
            return util.sendError(res, 404, 'Subscription not found');
        }
        if (paid.length === 0) return res.status(200).json(body);

        const sub = requested ? paid.find((s) => s.uuid === requested) : paid[0];
        if (!sub) return util.sendError(res, 404, 'Subscription not found');
        const billing = billingRows.get(sub.uuid);
        const range = periodRange(period, billing);
        if (requestedInterval && !seriesIntervals(range).includes(requestedInterval)) {
            return util.sendError(res, 400, `This period can be shown per ${seriesIntervals(range).join(', ')}.`);
        }
        const interval = requestedInterval || seriesInterval(range);
        Object.assign(body, {
            subscriptionId: sub.uuid,
            from: range.from.toISOString(),
            to: range.end.toISOString(),
            status: subscriptionBilling.accessStatus(billing.billing_status),
        });

        const pricing = pricingOf.get(sub.uuid);
        // Each usage price the subscription was sold with counts with its meter: the prices with an
        // item in its API plan, so a price that does not apply to this API's type is not shown.
        const sold = new Set((await billingApiPlanDao.listItems(billing.api_plan_uuid)).map((i) => i.price_uuid).filter(Boolean));
        const prices = pricing ? pricing.prices.filter((p) => sold.has(p.uuid)) : [];
        const handles = [...new Set(prices.map((p) => p.meter_handle))];
        const meters = new Map((await meterDao.listByHandles(orgId, handles)).map((m) => [m.handle, m]));
        const counted = prices.filter((p) => meters.has(p.meter_handle));
        body.usage = counted.length > 0 ? {
            interval,
            intervals: seriesIntervals(range),
            prices: counted.map((p) => ({ name: p.name, unit: meterUnit(meters.get(p.meter_handle)), total: null,
                buckets: null })),
        } : null;

        const conn = await subscriptionBilling.configurationFor(billing);
        const customer = conn ? await billingCheckoutDao.getCustomer(billing.customer_uuid) : null;
        if (!conn || !customer) {
            body.usageAvailable = false;
            return res.status(200).json(body);
        }

        const connector = billingProvider.forConfiguration(conn);
        const ids = { customerRef: customer.customer_ref, subscriptionRef: billing.billing_subscription_id };
        const [usage, estimate] = await Promise.allSettled([
            body.usage
                ? Promise.all(counted.map((p) => {
                    const what = countedFilters(meters.get(p.meter_handle), p.filters);
                    return priceUsage(connector.usage, p, connector.catalog.priceMeter(what.meter, what.filters), ids, range,
                        interval);
                }))
                : Promise.resolve(null),
            period === 'CURRENT' && billing.billing_status !== 'CANCELLED'
                ? connector.payments.estimate(billing.billing_subscription_id)
                : Promise.resolve(null),
        ]);
        if (usage.status === 'fulfilled') {
            if (usage.value) body.usage.prices = usage.value;
        } else {
            body.usageAvailable = false;
            logger.warn('Could not read usage from the billing provider', { subscriptionId: sub.uuid, error: usage.reason && usage.reason.message });
        }
        if (estimate.status === 'fulfilled' && estimate.value) {
            body.estimate = subscriptionBilling.estimateDto(estimate.value,
                await subscriptionBilling.priceNames(orgId, billing, pricing));
        } else if (estimate.status === 'rejected') {
            logger.warn('Could not read the estimate from the payment provider', {
                subscriptionId: sub.uuid, error: estimate.reason && estimate.reason.message,
            });
        }
        return res.status(200).json(body);
    } catch (error) {
        logger.error('Error reading usage', { error: error.message, orgId });
        return util.handleError(res, error);
    }
};

module.exports = { PERIODS, INTERVALS, periodRange, seriesInterval, seriesIntervals, fillSeries, getUsage };
