/*
 * Copyright (c) 2024, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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

const crypto = require('crypto');
const db = require('../db/driver');
const { groupBy } = require('../db/rows');
const { CustomError, ValidationError } = require('../utils/errors/customErrors');
const { getPortalId } = require('../utils/orgContext');
const { bufferToUtf8 } = require('../utils/cryptoUtil');
const planConditions = require('../utils/subscriptionPlanConditions');
const planPricingDao = require('./planPricingDao');

const SUBSCRIPTION_PLANS_TABLE = 'subscription_plans';
const SUBSCRIPTION_PLAN_LIMITS_TABLE = 'subscription_plan_limits';
const SUBSCRIPTION_PLAN_CONDITIONS_TABLE = 'subscription_plan_conditions';
const API_SUBSCRIPTION_PLAN_MAPPINGS_TABLE = 'api_subscription_plan_mappings';

const VALID_LIMIT_TYPES = ['REQUEST_COUNT', 'EVENT_COUNT', 'BANDWIDTH', 'TOTAL_TOKEN_COUNT'];

const buildSubscriptionPlanRow = (orgId, plan) => {
  return {
    org_uuid: orgId,
    handle: plan.handle,
    display_name: plan.displayName,
    description: plan.description,
    // Platform API resolves subscription_plan.ref_id in webhook events as the
    // plan handle, so a new plan defaults to it; updates keep the stored value.
    ref_id: plan.refId ?? plan.handle ?? null,
  };
};

/**
 * Validates the `limits` older clients send; their REQUEST_COUNT and EVENT_COUNT limits are kept, as
 * the request-count and event-count conditions.
 */
const normalizeLimits = (limits) => {
  if (!Array.isArray(limits)) return [];
  return limits.map(l => {
    if (!Number.isSafeInteger(l.limitCount) || (l.limitCount !== -1 && l.limitCount <= 0)) {
      throw new ValidationError('limitCount must be -1 (unlimited) or a positive whole number for each limit');
    }
    const limitType = (l.limitType || 'REQUEST_COUNT').toUpperCase();
    if (!VALID_LIMIT_TYPES.includes(limitType)) {
      throw new ValidationError(`limitType must be one of ${VALID_LIMIT_TYPES.join(', ')}`);
    }
    if (l.timeAmount !== undefined && l.timeAmount !== null &&
        (!Number.isSafeInteger(l.timeAmount) || l.timeAmount <= 0)) {
      throw new ValidationError('timeAmount must be a positive whole number when provided');
    }
    // A limit with no window can't be enforced; refusing it beats saving the plan as unlimited.
    if (l.limitCount !== -1 && ['REQUEST_COUNT', 'EVENT_COUNT'].includes(limitType) && !l.timeUnit) {
      throw new ValidationError('timeUnit (MINUTE, HOUR, DAY or MONTH) is required for each request or event limit');
    }
    return {
      limit_type: limitType,
      time_unit: l.timeUnit ? l.timeUnit.toUpperCase() : null,
      time_amount: l.timeAmount || 1,
      limit_count: l.limitCount,
    };
  });
};

/*
 * Which plan tables exist. New installs have no subscription_plan_limits; databases from earlier
 * releases have it, and PostgreSQL and SQL Server get subscription_plan_conditions only when the
 * operator runs this release's schema script. Checked again on each use until the conditions table
 * is found.
 */
let planTables = null;

const hasTable = async (name, exec) => {
  const dialect = db.getDialect();
  const sql = dialect === 'postgres'
    ? 'SELECT 1 AS found FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = ?'
    : dialect === 'mssql'
      ? 'SELECT 1 AS found FROM sys.tables WHERE name = ?'
      : "SELECT 1 AS found FROM sqlite_master WHERE type = 'table' AND name = ?";
  return Boolean(await exec.queryOne(sql, [name]));
};

const planSchema = async (exec) => {
  if (planTables) return planTables;
  const tables = {
    conditions: await hasTable(SUBSCRIPTION_PLAN_CONDITIONS_TABLE, exec),
    legacyLimits: await hasTable(SUBSCRIPTION_PLAN_LIMITS_TABLE, exec),
  };
  if (tables.conditions) planTables = tables;
  return tables;
};

const inPlaceholders = (n) => Array.from({ length: n }, () => '?').join(', ');

const toCondition = (row) => ({
  id: row.condition_id,
  type: row.condition_type,
  params: JSON.parse(bufferToUtf8(row.params) || '{}'),
});

const WINDOW_MINUTES = { MINUTE: 1, HOUR: 60, DAY: 24 * 60, MONTH: 31 * 24 * 60 };

/**
 * Reads the REQUEST_COUNT and EVENT_COUNT rows of plans saved before conditions as each plan's
 * request-count and event-count conditions.
 */
const loadLegacyLimits = async (plans, exec) => {
  const ids = [...new Set(plans.map((p) => p.uuid))];
  if (ids.length === 0) return;
  const rows = await exec.query(
    `SELECT plan_uuid, limit_type, time_unit, time_amount, limit_count FROM ${SUBSCRIPTION_PLAN_LIMITS_TABLE}
     WHERE limit_type IN ('REQUEST_COUNT', 'EVENT_COUNT') AND plan_uuid IN (${inPlaceholders(ids.length)}) AND portal_id = ?`,
    [...ids, getPortalId()]
  );
  const grouped = groupBy(rows, 'plan_uuid');
  for (const plan of plans) {
    // Old rows carry no order of their own: shortest window first.
    const length = (l) => (WINDOW_MINUTES[String(l.time_unit).toUpperCase()] || 0) * (Number(l.time_amount) || 1);
    const limits = [...(grouped.get(plan.uuid) || [])].sort((a, b) => length(a) - length(b));
    plan.conditions = planConditions.conditionsFromLimits([], limits);
  }
};

/**
 * Attaches `.conditions` to each plan, and `.limits`: the REQUEST_COUNT and EVENT_COUNT limits of its
 * request-count and event-count conditions, for the clients and events that predate conditions. A plan saved before conditions is
 * read from subscription_plan_limits.
 */
const attachConditions = async (plans, t) => {
  if (plans.length === 0) return plans;
  const exec = t || db;
  const schema = await planSchema(exec);
  let grouped = new Map();
  if (schema.conditions) {
    const ids = [...new Set(plans.map((p) => p.uuid))];
    grouped = groupBy(await exec.query(
      `SELECT * FROM ${SUBSCRIPTION_PLAN_CONDITIONS_TABLE} WHERE plan_uuid IN (${inPlaceholders(ids.length)}) AND portal_id = ? ORDER BY sort_order ASC`,
      [...ids, getPortalId()]
    ), 'plan_uuid');
  }
  for (const plan of plans) {
    plan.conditions = (grouped.get(plan.uuid) || []).map(toCondition);
  }
  if (schema.legacyLimits) {
    await loadLegacyLimits(plans.filter((p) => p.conditions.length === 0), exec);
  }
  for (const plan of plans) {
    plan.limits = planConditions.legacyLimitsFromConditions(plan.conditions).map((l) => ({
      limit_type: l.limitType, time_unit: l.timeUnit, time_amount: l.timeAmount, limit_count: l.limitCount,
    }));
  }
  return plans;
};

/**
 * Stores the request-count and event-count conditions in subscription_plan_limits, as REQUEST_COUNT
 * and EVENT_COUNT rows, for a database not upgraded yet.
 */
const saveLegacyLimits = async (planId, conditions, schema, exec) => {
  // subscription_plan_limits keeps neither other condition types nor onExceed: allow.
  const { REQUEST_COUNT, EVENT_COUNT } = planConditions.TYPE;
  if (!schema.legacyLimits || conditions.some((c) => ![REQUEST_COUNT, EVENT_COUNT].includes(c.type)
      || c.params?.onExceed === 'allow')) {
    throw new CustomError(503, 'Service Unavailable',
      'The database has no subscription_plan_conditions table; run this release\'s database schema script to store this plan\'s conditions.');
  }
  await exec.execute(`DELETE FROM ${SUBSCRIPTION_PLAN_LIMITS_TABLE} WHERE plan_uuid = ? AND portal_id = ?`, [planId, getPortalId()]);
  for (const l of planConditions.legacyLimitsFromConditions(conditions)) {
    await exec.execute(
      `INSERT INTO ${SUBSCRIPTION_PLAN_LIMITS_TABLE} (uuid, portal_id, plan_uuid, limit_type, time_unit, time_amount, limit_count)
       VALUES (?, ?, ?, ?, ?, ?, ?)`,
      [crypto.randomUUID(), getPortalId(), planId, l.limitType, l.timeUnit, l.timeAmount, l.limitCount]
    );
  }
};

/**
 * Saves the plan's conditions. `conditions` replaces them all. `limits` sent alone, by clients that
 * predate conditions, sets the request-count and event-count conditions from its REQUEST_COUNT and
 * EVENT_COUNT limits and keeps the others. With neither, an update keeps them. Saving moves a plan off subscription_plan_limits.
 */
const savePlanDetails = async (planId, plan, isNew, t) => {
  const exec = t || db;
  const hasLimits = Object.prototype.hasOwnProperty.call(plan, 'limits');
  let conditions;
  if (plan.conditions !== undefined && plan.conditions !== null) {
    conditions = planConditions.normalizeConditions(plan.conditions);
  } else if (hasLimits) {
    const current = isNew ? [] : (await attachConditions([{ uuid: planId }], t))[0].conditions;
    // Checked again: adding the request-count or event-count condition can take a plan past its 20 conditions.
    conditions = planConditions.normalizeConditions(
      planConditions.conditionsFromLimits(current, normalizeLimits(plan.limits)));
  } else if (isNew) {
    conditions = [];
  } else {
    return;
  }

  const schema = await planSchema(exec);
  if (!schema.conditions) {
    await saveLegacyLimits(planId, conditions, schema, exec);
    return;
  }
  const portalId = getPortalId();
  await exec.execute(`DELETE FROM ${SUBSCRIPTION_PLAN_CONDITIONS_TABLE} WHERE plan_uuid = ? AND portal_id = ?`, [planId, portalId]);
  for (const [i, c] of conditions.entries()) {
    await exec.execute(
      `INSERT INTO ${SUBSCRIPTION_PLAN_CONDITIONS_TABLE} (uuid, portal_id, plan_uuid, condition_id, condition_type, params, sort_order)
       VALUES (?, ?, ?, ?, ?, ?, ?)`,
      [crypto.randomUUID(), portalId, planId, c.id, c.type, db.binaryParam(Buffer.from(JSON.stringify(c.params), 'utf8')), i]
    );
  }
  if (schema.legacyLimits) {
    await exec.execute(`DELETE FROM ${SUBSCRIPTION_PLAN_LIMITS_TABLE} WHERE plan_uuid = ? AND portal_id = ?`, [planId, getPortalId()]);
  }
};

/** Attaches `.conditions`, `.limits` and `.pricing` (null for a free plan) to each plan row. */
const attachDetails = async (plans, t) => {
  await attachConditions(plans, t);
  await planPricingDao.attachPricing(plans, t);
  return plans;
};

/** Fetches a single plan (scoped to its organization and portal) with its conditions and pricing attached. */
const findPlanByUuid = async (orgId, planId, t) => {
  const exec = t || db;
  const plan = await exec.queryOne(
    `SELECT * FROM ${SUBSCRIPTION_PLANS_TABLE} WHERE uuid = ? AND org_uuid = ? AND portal_id = ?`,
    [planId, orgId, getPortalId()]
  );
  if (!plan) return null;
  await attachDetails([plan], t);
  return plan;
};

const create = async (orgId, plan, createdBy, t) => {
  const exec = t || db;
  const uuid = crypto.randomUUID();
  const now = new Date();
  const row = buildSubscriptionPlanRow(orgId, plan);

  await exec.execute(
    `INSERT INTO ${SUBSCRIPTION_PLANS_TABLE}
       (uuid, org_uuid, portal_id, handle, display_name, description, ref_id, created_by, updated_by, created_at, updated_at)
     VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
    [uuid, row.org_uuid, getPortalId(), row.handle, row.display_name, row.description, row.ref_id, createdBy, createdBy, now, now]
  );
  await savePlanDetails(uuid, plan, true, t);
  return findPlanByUuid(orgId, uuid, t);
};

const createMany = async (orgId, plans, createdBy, t) => {
  const exec = t || db;
  const portalId = getPortalId();
  const uuids = [];
  for (const plan of plans) {
    const uuid = crypto.randomUUID();
    const now = new Date();
    const row = buildSubscriptionPlanRow(orgId, plan);
    await exec.execute(
      `INSERT INTO ${SUBSCRIPTION_PLANS_TABLE}
         (uuid, org_uuid, portal_id, handle, display_name, description, ref_id, created_by, updated_by, created_at, updated_at)
       VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
      [uuid, row.org_uuid, portalId, row.handle, row.display_name, row.description, row.ref_id, createdBy, createdBy, now, now]
    );
    await savePlanDetails(uuid, plan, true, t);
    uuids.push(uuid);
  }
  if (uuids.length === 0) return [];
  const placeholders = uuids.map(() => '?').join(', ');
  const rows = await exec.query(
    `SELECT * FROM ${SUBSCRIPTION_PLANS_TABLE} WHERE uuid IN (${placeholders}) AND org_uuid = ? AND portal_id = ?`,
    [...uuids, orgId, portalId]
  );
  await attachDetails(rows, t);
  return rows;
};

const put = async (orgId, plan, updatedBy, t) => {
  const current = await getByName(orgId, plan.handle, t);
  if (current) {
    const updated = await update(orgId, current.uuid, plan, updatedBy, t);
    return { subscriptionPlanResponse: updated, statusCode: 200 };
  }
  const created = await create(orgId, plan, updatedBy, t);
  return { subscriptionPlanResponse: created, statusCode: 201 };
};

const update = async (orgId, planId, plan, updatedBy, t) => {
  const exec = t || db;
  const row = buildSubscriptionPlanRow(orgId, plan);
  const updatedAt = new Date();

  // Don't update primary keys — org_uuid never changes; ref_id only changes
  // when the caller explicitly supplied it.
  const setCols = ['handle = ?', 'display_name = ?', 'description = ?'];
  const params = [row.handle, row.display_name, row.description];
  // undefined counts as absent: the YAML mapper sets refId from spec.refId even
  // when the document has none.
  if (plan.refId !== undefined) {
    setCols.push('ref_id = ?');
    params.push(plan.refId ?? null);
  }
  setCols.push('updated_by = ?', 'updated_at = ?');
  params.push(updatedBy, updatedAt);

  const { rowCount } = await exec.execute(
    `UPDATE ${SUBSCRIPTION_PLANS_TABLE} SET ${setCols.join(', ')} WHERE uuid = ? AND org_uuid = ? AND portal_id = ?`,
    [...params, planId, orgId, getPortalId()]
  );

  if (rowCount === 0) return null;

  await savePlanDetails(planId, plan, false, t);

  return findPlanByUuid(orgId, planId, t);
};

/**
 * Deletes the plan unless subscriptions use it. Resolves to `{ deleted, subscriptionCount }`:
 * `deleted` is 1 when the plan was deleted, and 0 when it was not found or has subscriptions;
 * `subscriptionCount` is how many subscriptions it has. Run it inside a transaction: on PostgreSQL
 * and SQL Server the plan row is locked before the count, so no subscription can be added before
 * the delete. SQLite runs one write transaction at a time, so it needs no lock.
 */
const deletePlan = async (orgId, planName, t) => {
  const exec = t || db;
  const dialect = db.getDialect();
  const lockHint = dialect === 'mssql' ? ' WITH (UPDLOCK, ROWLOCK)' : '';
  const forUpdate = dialect === 'postgres' ? ' FOR UPDATE' : '';
  const plan = await exec.queryOne(
    `SELECT uuid FROM ${SUBSCRIPTION_PLANS_TABLE}${lockHint} WHERE handle = ? AND org_uuid = ? AND portal_id = ?${forUpdate}`,
    [planName, orgId, getPortalId()]
  );
  if (!plan) return { deleted: 0, subscriptionCount: 0 };

  const { n } = await exec.queryOne(
    'SELECT COUNT(*) AS n FROM subscriptions WHERE plan_uuid = ? AND org_uuid = ? AND portal_id = ?',
    [plan.uuid, orgId, getPortalId()]
  );
  const subscriptionCount = Number(n);
  if (subscriptionCount > 0) return { deleted: 0, subscriptionCount };

  const { rowCount } = await exec.execute(
    `DELETE FROM ${SUBSCRIPTION_PLANS_TABLE} WHERE uuid = ? AND org_uuid = ? AND portal_id = ?`,
    [plan.uuid, orgId, getPortalId()]
  );
  return { deleted: rowCount, subscriptionCount: 0 };
};

/**
 * Records the Platform API handle a plan is known by. Only used for a plan created in this
 * portal, whose ref_id is empty until the plan is sent to the Platform API.
 */
const setRefId = async (orgId, planId, refId, t) => {
  const exec = t || db;
  await exec.execute(
    `UPDATE ${SUBSCRIPTION_PLANS_TABLE} SET ref_id = ? WHERE uuid = ? AND org_uuid = ? AND portal_id = ?`,
    [refId, planId, orgId, getPortalId()]
  );
};

const getByName = async (orgId, planName, t) => {
  const exec = t || db;
  const plan = await exec.queryOne(
    `SELECT * FROM ${SUBSCRIPTION_PLANS_TABLE} WHERE handle = ? AND org_uuid = ? AND portal_id = ?`,
    [planName, orgId, getPortalId()]
  );
  if (!plan) return null;
  await attachDetails([plan], t);
  return plan;
};

/**
 * Plans mapped to a given API, via the api_subscription_plan_mappings join
 * table. Mirrors the previous belongsToMany `include: [{model: APIMetadata,
 * where: {uuid: apiId}}, ...PLAN_INCLUDE]` shape. Intentionally not scoped by
 * org_uuid — the original Sequelize query wasn't either.
 */
/** How many APIs offer the plan. */
const countApis = async (orgId, planUuid, t) => {
  const exec = t || db;
  const { n } = await exec.queryOne(
    `SELECT COUNT(*) AS n FROM ${API_SUBSCRIPTION_PLAN_MAPPINGS_TABLE} m
     JOIN ${SUBSCRIPTION_PLANS_TABLE} sp ON sp.uuid = m.plan_uuid AND sp.portal_id = m.portal_id
     WHERE m.plan_uuid = ? AND sp.org_uuid = ? AND m.portal_id = ?`,
    [planUuid, orgId, getPortalId()]
  );
  return Number(n);
};

/** How many subscriptions the plan has. */
const countSubscriptions = async (orgId, planUuid) => {
  const { n } = await db.queryOne(
    'SELECT COUNT(*) AS n FROM subscriptions WHERE plan_uuid = ? AND org_uuid = ? AND portal_id = ?',
    [planUuid, orgId, getPortalId()]
  );
  return Number(n);
};

/** How many subscriptions each of the organization's plans has, as a Map of plan uuid to count; plans with none are left out. */
const countSubscriptionsByPlan = async (orgId) => {
  const rows = await db.query(
    'SELECT plan_uuid, COUNT(*) AS n FROM subscriptions WHERE plan_uuid IS NOT NULL AND org_uuid = ? AND portal_id = ? GROUP BY plan_uuid',
    [orgId, getPortalId()]
  );
  return new Map(rows.map((row) => [row.plan_uuid, Number(row.n)]));
};

/**
 * How many subscriptions each of the plans has to the API, as a Map of plan uuid to count (a plan
 * with none is absent). Given a transaction `t`, on PostgreSQL and SQL Server the plan rows are
 * locked before the count, as in deletePlan, so no subscription to these plans can be added until
 * the transaction ends.
 */
const countApiSubscriptions = async (orgId, apiId, planUuids, t) => {
  const counts = new Map();
  if (planUuids.length === 0) return counts;
  const exec = t || db;
  const marks = planUuids.map(() => '?').join(', ');
  if (t) {
    const dialect = db.getDialect();
    const lockHint = dialect === 'mssql' ? ' WITH (UPDLOCK, ROWLOCK)' : '';
    const forUpdate = dialect === 'postgres' ? ' FOR UPDATE' : '';
    await exec.query(
      `SELECT uuid FROM ${SUBSCRIPTION_PLANS_TABLE}${lockHint} WHERE uuid IN (${marks}) AND org_uuid = ? AND portal_id = ?${forUpdate}`,
      [...planUuids, orgId, getPortalId()]
    );
  }
  const rows = await exec.query(
    `SELECT plan_uuid, COUNT(*) AS n FROM subscriptions
     WHERE api_uuid = ? AND plan_uuid IN (${marks}) AND org_uuid = ? AND portal_id = ? GROUP BY plan_uuid`,
    [apiId, ...planUuids, orgId, getPortalId()]
  );
  for (const row of rows) counts.set(row.plan_uuid, Number(row.n));
  return counts;
};

const listByApi = async (apiId, t) => {
  const exec = t || db;
  const plans = await exec.query(
    `SELECT sp.* FROM ${SUBSCRIPTION_PLANS_TABLE} sp
     JOIN ${API_SUBSCRIPTION_PLAN_MAPPINGS_TABLE} m ON m.plan_uuid = sp.uuid AND m.portal_id = sp.portal_id
     WHERE m.api_uuid = ? AND m.portal_id = ?`,
    [apiId, getPortalId()]
  );
  await attachDetails(plans, t);
  return plans;
};

const list = async (orgId, t) => {
  const exec = t || db;
  const plans = await exec.query(
    `SELECT * FROM ${SUBSCRIPTION_PLANS_TABLE} WHERE org_uuid = ? AND portal_id = ? ORDER BY handle`,
    [orgId, getPortalId()]
  );
  await attachDetails(plans, t);
  return plans;
};

const createApiMapping = async (apiSubscriptionPlans, apiId, createdBy, t) => {
  const exec = t || db;
  const now = new Date();
  const portalId = getPortalId();
  const created = [];
  for (const plan of apiSubscriptionPlans) {
    const uuid = crypto.randomUUID();
    await exec.execute(
      `INSERT INTO ${API_SUBSCRIPTION_PLAN_MAPPINGS_TABLE} (uuid, portal_id, plan_uuid, api_uuid, created_by, created_at)
       VALUES (?, ?, ?, ?, ?, ?)`,
      [uuid, portalId, plan.planId, apiId, createdBy, now]
    );
    created.push({ uuid, plan_uuid: plan.planId, api_uuid: apiId, created_by: createdBy, created_at: now });
  }
  return created;
};

const updateApiMapping = async (subscriptionPlans, apiId, updatedBy, t) => {
  const exec = t || db;
  await exec.execute(`DELETE FROM ${API_SUBSCRIPTION_PLAN_MAPPINGS_TABLE} WHERE api_uuid = ?`, [apiId]);
  if (subscriptionPlans.length > 0) {
    return createApiMapping(subscriptionPlans, apiId, updatedBy, t);
  }
  return [];
};

module.exports = {
    create,
    createMany,
    put,
    update,
    delete: deletePlan,
    getByName,
    getByUuid: findPlanByUuid,
    setRefId,
    listByApi,
    countSubscriptions,
    countApis,
    countApiSubscriptions,
    countSubscriptionsByPlan,
    list,
    attachConditions,
    createApiMapping,
    updateApiMapping,
};
