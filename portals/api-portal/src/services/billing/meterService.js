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
 * Meters: what a paid plan charges for. A meter counts the requests the gateway allows, or adds
 * up a number in each event (for AI APIs, a token count) at its event key. It has no conditions: it
 * lists the fields its plan prices may filter on, each with the event key it reads, and each price
 * narrows it with its own filters.
 *
 * Once at least one plan prices a meter it cannot change at all (name and description included)
 * nor be deleted; before that every field can change.
 *
 * This portal is the source of truth for its meters. Every change is also sent to the
 * Platform API as a meter.* event, recorded in the same transaction.
 */

const crypto = require('crypto');
const db = require('../../db/driver');
const meterDao = require('../../dao/meterDao');
const MeterDTO = require('../../dto/meterDto');
const userIdpReferenceDao = require('../../dao/userIdpReferenceDao');
const { publish } = require('../webhooks/eventPublisher');
const sharedKeyAuth = require('../../middlewares/sharedKeyAuth');
const util = require('../../utils/util');
const logger = require('../../config/logger');
const { logUserAction } = require('../../middlewares/auditLogger');
const { MeterValidationError, validateMeter } = require('./meterValidation');

const EVENT = Object.freeze({
    CREATED: 'meter.created',
    UPDATED: 'meter.updated',
    DELETED: 'meter.deleted',
});

const ERROR = Object.freeze({
    NOT_FOUND: 'Meter not found',
    IN_USE: 'METER_IN_USE',
});

function buildMeterPayload(meter) {
    return {
        handle: meter.handle,
        display_name: meter.display_name,
        description: meter.description || null,
        type: meter.type,
        type_key: meter.type_key || null,
        filter_fields: (meter.filter_fields || []).map((f) => ({ field: f.field, event_key: f.eventKey })),
    };
}

/** How many plans price a meter, for its refusals: counts, not names, as a meter may be priced by many plans. */
function pricedBy(plans) {
    return plans.length === 1 ? 'The meter is priced by 1 plan' : `The meter is priced by ${plans.length} plans`;
}

/** The refusal for a change to a meter that plans price. */
function inUseMessage(plans) {
    return `${pricedBy(plans)}, so it can't change.`;
}

async function publishMeter(eventType, orgId, meter, t) {
    const payload = eventType === EVENT.DELETED ? { handle: meter.handle } : buildMeterPayload(meter);
    await publish(eventType, payload, { transaction: t, orgId, aggregateType: 'meter', aggregateId: meter.uuid });
}

async function toDto(orgId, meter) {
    const usage = await meterDao.listPlanHandlesByMeter(orgId, [meter.uuid]);
    const audit = await userIdpReferenceDao.buildSingleAuditFields(meter);
    return new MeterDTO(meter, usage.get(meter.uuid), audit);
}

/** A derived handle as it is, or with a random suffix when another meter has it. */
async function freeDerivedHandle(orgId, handle) {
    if (!(await meterDao.getByHandle(orgId, handle))) return handle;
    const base = handle.slice(0, 33).replace(/-+$/, '');
    return `${base}-${crypto.randomBytes(3).toString('hex')}`;
}

const listMeters = async (req, res) => {
    try {
        const meters = await meterDao.list(req.orgId);
        const usage = await meterDao.listPlanHandlesByMeter(req.orgId, meters.map((m) => m.uuid));
        const audits = await userIdpReferenceDao.buildListAuditFields(meters);
        const dtos = meters.map((m, i) => new MeterDTO(m, usage.get(m.uuid), audits[i]));
        return res.status(200).json(util.toPaginatedList(dtos, req));
    } catch (error) {
        logger.error('Error while listing meters', { error });
        return util.sendError(res, 500, 'Error while listing meters');
    }
};

const getMeter = async (req, res) => {
    try {
        const meter = await meterDao.getByHandle(req.orgId, req.params.meterId);
        if (!meter) return util.sendError(res, 404, ERROR.NOT_FOUND);
        return res.status(200).json(await toDto(req.orgId, meter));
    } catch (error) {
        logger.error('Error while retrieving meter', { error });
        return util.sendError(res, 500, 'Error while retrieving meter');
    }
};

const createMeter = async (req, res) => {
    const orgId = req.orgId;
    let input;
    try {
        input = validateMeter(req.body, { create: true });
    } catch (error) {
        if (error instanceof MeterValidationError) return util.sendError(res, 400, error.message);
        throw error;
    }
    try {
        if (!input.handleGiven) {
            input.handle = await freeDerivedHandle(orgId, input.handle);
        }
        const userId = util.resolveActor(req);
        const created = await db.withTransaction(async (t) => {
            const meter = await meterDao.create(orgId, input, userId, t);
            if (!sharedKeyAuth.madeByPlatformApi(req)) await publishMeter(EVENT.CREATED, orgId, meter, t);
            return meter;
        });
        logUserAction('METER_CREATED', req, { orgId, resourceUuid: created.uuid, resourceType: 'meter' });
        // The router is mounted at the portal's root (baseUrl), so the collection's address is baseUrl + path.
        res.setHeader('Location', `${req.baseUrl}${req.path.replace(/\/$/, '')}/${encodeURIComponent(created.handle)}`);
        return res.status(201).json(await toDto(orgId, created));
    } catch (error) {
        if (db.isDuplicateKeyError(error)) {
            return util.sendError(res, 409, `A meter with id "${input.handle}" already exists in this organization.`);
        }
        logger.error('Error while creating meter', { error });
        return util.sendError(res, 500, 'Error while creating meter');
    }
};

const updateMeter = async (req, res) => {
    const orgId = req.orgId;
    const { meterId } = req.params;
    let input;
    try {
        input = validateMeter(req.body, { create: false });
    } catch (error) {
        if (error instanceof MeterValidationError) return util.sendError(res, 400, error.message);
        throw error;
    }
    if (req.body.id !== undefined && req.body.id !== meterId) {
        return util.sendError(res, 400, 'The id of a meter cannot be changed');
    }
    try {
        const userId = util.resolveActor(req);
        const outcome = await db.withTransaction(async (t) => {
            const current = await meterDao.getByHandle(orgId, meterId, t);
            if (!current) return { status: 404 };
            // Once a plan prices the meter nothing about it changes: its prices count what it counts
            // and are named after it, and their billing-provider objects cannot be edited.
            const plans = (await meterDao.listPlanHandlesByMeter(orgId, [current.uuid], t)).get(current.uuid) || [];
            if (plans.length > 0) return { status: 409, plans };
            const meter = await meterDao.update(orgId, meterId, input, userId, t);
            if (!sharedKeyAuth.madeByPlatformApi(req)) await publishMeter(EVENT.UPDATED, orgId, meter, t);
            return { status: 200, meter };
        });
        if (outcome.status === 404) return util.sendError(res, 404, ERROR.NOT_FOUND);
        if (outcome.status === 409) return util.sendError(res, 409, inUseMessage(outcome.plans), { code: ERROR.IN_USE });
        const updated = outcome.meter;
        logUserAction('METER_UPDATED', req, { orgId, resourceUuid: updated.uuid, resourceType: 'meter' });
        return res.status(200).json(await toDto(orgId, updated));
    } catch (error) {
        logger.error('Error while updating meter', { error });
        return util.sendError(res, 500, 'Error while updating meter');
    }
};

const deleteMeter = async (req, res) => {
    const orgId = req.orgId;
    const { meterId } = req.params;
    try {
        const outcome = await db.withTransaction(async (t) => {
            const meter = await meterDao.getByHandle(orgId, meterId, t);
            if (!meter) return { status: 404 };
            const usage = await meterDao.listPlanHandlesByMeter(orgId, [meter.uuid], t);
            const plans = usage.get(meter.uuid);
            if (plans && plans.length > 0) return { status: 409, plans };
            await meterDao.delete(orgId, meterId, t);
            if (!sharedKeyAuth.madeByPlatformApi(req)) await publishMeter(EVENT.DELETED, orgId, meter, t);
            return { status: 204, meter };
        });
        if (outcome.status === 404) return util.sendError(res, 404, ERROR.NOT_FOUND);
        if (outcome.status === 409) {
            return util.sendError(res, 409, `${pricedBy(outcome.plans)}, so it can't be deleted.`, { code: ERROR.IN_USE });
        }
        logUserAction('METER_DELETED', req, { orgId, resourceUuid: outcome.meter.uuid, resourceType: 'meter' });
        return res.status(204).send();
    } catch (error) {
        logger.error('Error while deleting meter', { error });
        return util.sendError(res, 500, 'Error while deleting meter');
    }
};

module.exports = {
    EVENT,
    buildMeterPayload,
    listMeters,
    getMeter,
    createMeter,
    updateMeter,
    deleteMeter,
};
