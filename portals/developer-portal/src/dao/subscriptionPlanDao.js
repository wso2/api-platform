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
const { ValidationError } = require('../utils/errors/customErrors');

const SUBSCRIPTION_PLANS_TABLE = 'dp_subscription_plans';
const SUBSCRIPTION_PLAN_LIMITS_TABLE = 'dp_subscription_plan_limits';
const API_SUBSCRIPTION_PLAN_MAPPINGS_TABLE = 'dp_api_subscription_plan_mappings';

const VALID_LIMIT_TYPES = ['REQUEST_COUNT', 'EVENT_COUNT', 'BANDWIDTH', 'TOTAL_TOKEN_COUNT'];

const buildSubscriptionPlanRow = (orgId, plan) => {
  return {
    org_uuid: orgId,
    handle: plan.handle,
    display_name: plan.displayName,
    description: plan.description,
    ref_id: plan.refId ?? null,
  };
};

const normalizeLimits = (limits) => {
  if (!Array.isArray(limits)) return [];
  return limits.map(l => {
    if (typeof l.limitCount !== 'number' || !Number.isFinite(l.limitCount) ||
        (l.limitCount !== -1 && l.limitCount <= 0)) {
      throw new ValidationError('limitCount must be -1 (unlimited) or a positive number for each limit');
    }
    const limitType = (l.limitType || 'REQUEST_COUNT').toUpperCase();
    if (!VALID_LIMIT_TYPES.includes(limitType)) {
      throw new ValidationError(`limitType must be one of ${VALID_LIMIT_TYPES.join(', ')}`);
    }
    if (l.timeAmount !== undefined && l.timeAmount !== null &&
        (typeof l.timeAmount !== 'number' || !Number.isFinite(l.timeAmount) || l.timeAmount <= 0)) {
      throw new ValidationError('timeAmount must be a positive number when provided');
    }
    return {
      uuid: crypto.randomUUID(),
      limit_type: limitType,
      time_unit: l.timeUnit ? l.timeUnit.toUpperCase() : null,
      time_amount: l.timeAmount || 1,
      limit_count: l.limitCount,
    };
  });
};

const replaceLimits = async (planId, limits, t) => {
  const exec = t || db;
  await exec.execute(`DELETE FROM ${SUBSCRIPTION_PLAN_LIMITS_TABLE} WHERE plan_uuid = ?`, [planId]);
  const rows = normalizeLimits(limits);
  for (const r of rows) {
    await exec.execute(
      `INSERT INTO ${SUBSCRIPTION_PLAN_LIMITS_TABLE} (uuid, plan_uuid, limit_type, time_unit, time_amount, limit_count)
       VALUES (?, ?, ?, ?, ?, ?)`,
      [r.uuid, planId, r.limit_type, r.time_unit, r.time_amount, r.limit_count]
    );
  }
};

/**
 * App-side "eager load" of a plan's limits, mirroring the previous
 * `include: PLAN_INCLUDE, order: LIMIT_ORDER` shape: one query for all limit
 * rows belonging to the given plans, grouped and attached under `.limits`,
 * ordered by `uuid ASC` within each group.
 */
const attachLimits = async (plans, t) => {
  const exec = t || db;
  if (plans.length === 0) return plans;
  const planIds = plans.map(p => p.uuid);
  const placeholders = planIds.map(() => '?').join(', ');
  const limitRows = await exec.query(
    `SELECT * FROM ${SUBSCRIPTION_PLAN_LIMITS_TABLE} WHERE plan_uuid IN (${placeholders}) ORDER BY uuid ASC`,
    planIds
  );
  const grouped = groupBy(limitRows, 'plan_uuid');
  for (const plan of plans) {
    plan.limits = grouped.get(plan.uuid) || [];
  }
  return plans;
};

/** Fetches a single plan (scoped to its organization) with `.limits` attached. */
const findPlanByUuid = async (orgId, planId, t) => {
  const exec = t || db;
  const plan = await exec.queryOne(
    `SELECT * FROM ${SUBSCRIPTION_PLANS_TABLE} WHERE uuid = ? AND org_uuid = ?`,
    [planId, orgId]
  );
  if (!plan) return null;
  await attachLimits([plan], t);
  return plan;
};

const create = async (orgId, plan, createdBy, t) => {
  const exec = t || db;
  const uuid = crypto.randomUUID();
  const now = new Date();
  const row = buildSubscriptionPlanRow(orgId, plan);

  await exec.execute(
    `INSERT INTO ${SUBSCRIPTION_PLANS_TABLE}
       (uuid, org_uuid, handle, display_name, description, ref_id, created_by, updated_by, created_at, updated_at)
     VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
    [uuid, row.org_uuid, row.handle, row.display_name, row.description, row.ref_id, createdBy, createdBy, now, now]
  );
  await replaceLimits(uuid, plan.limits || [], t);
  return findPlanByUuid(orgId, uuid, t);
};

const createMany = async (orgId, plans, createdBy, t) => {
  const exec = t || db;
  const uuids = [];
  for (const plan of plans) {
    const uuid = crypto.randomUUID();
    const now = new Date();
    const row = buildSubscriptionPlanRow(orgId, plan);
    await exec.execute(
      `INSERT INTO ${SUBSCRIPTION_PLANS_TABLE}
         (uuid, org_uuid, handle, display_name, description, ref_id, created_by, updated_by, created_at, updated_at)
       VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
      [uuid, row.org_uuid, row.handle, row.display_name, row.description, row.ref_id, createdBy, createdBy, now, now]
    );
    await replaceLimits(uuid, plan.limits || [], t);
    uuids.push(uuid);
  }
  if (uuids.length === 0) return [];
  const placeholders = uuids.map(() => '?').join(', ');
  const rows = await exec.query(
    `SELECT * FROM ${SUBSCRIPTION_PLANS_TABLE} WHERE uuid IN (${placeholders}) AND org_uuid = ?`,
    [...uuids, orgId]
  );
  await attachLimits(rows, t);
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
  if (Object.prototype.hasOwnProperty.call(plan, 'refId')) {
    setCols.push('ref_id = ?');
    params.push(row.ref_id);
  }
  setCols.push('updated_by = ?', 'updated_at = ?');
  params.push(updatedBy, updatedAt);

  await exec.execute(
    `UPDATE ${SUBSCRIPTION_PLANS_TABLE} SET ${setCols.join(', ')} WHERE uuid = ? AND org_uuid = ?`,
    [...params, planId, orgId]
  );

  if (Object.prototype.hasOwnProperty.call(plan, 'limits')) {
    await replaceLimits(planId, plan.limits || [], t);
  }

  return findPlanByUuid(orgId, planId, t);
};

const deletePlan = async (orgId, planName, t) => {
  const exec = t || db;
  const { rowCount } = await exec.execute(
    `DELETE FROM ${SUBSCRIPTION_PLANS_TABLE} WHERE handle = ? AND org_uuid = ?`,
    [planName, orgId]
  );
  return rowCount;
};

const getByName = async (orgId, planName, t) => {
  const exec = t || db;
  const plan = await exec.queryOne(
    `SELECT * FROM ${SUBSCRIPTION_PLANS_TABLE} WHERE handle = ? AND org_uuid = ?`,
    [planName, orgId]
  );
  if (!plan) return null;
  await attachLimits([plan], t);
  return plan;
};

/**
 * Plans mapped to a given API, via the dp_api_subscription_plan_mappings join
 * table. Mirrors the previous belongsToMany `include: [{model: APIMetadata,
 * where: {uuid: apiId}}, ...PLAN_INCLUDE]` shape. Intentionally not scoped by
 * org_uuid — the original Sequelize query wasn't either.
 */
const listByApi = async (apiId, t) => {
  const exec = t || db;
  const plans = await exec.query(
    `SELECT sp.* FROM ${SUBSCRIPTION_PLANS_TABLE} sp
     JOIN ${API_SUBSCRIPTION_PLAN_MAPPINGS_TABLE} m ON m.plan_uuid = sp.uuid
     WHERE m.api_uuid = ?`,
    [apiId]
  );
  await attachLimits(plans, t);
  return plans;
};

const list = async (orgId, t) => {
  const exec = t || db;
  const plans = await exec.query(`SELECT * FROM ${SUBSCRIPTION_PLANS_TABLE} WHERE org_uuid = ?`, [orgId]);
  await attachLimits(plans, t);
  return plans;
};

const createApiMapping = async (apiSubscriptionPlans, apiId, createdBy, t) => {
  const exec = t || db;
  const now = new Date();
  const created = [];
  for (const plan of apiSubscriptionPlans) {
    const uuid = crypto.randomUUID();
    await exec.execute(
      `INSERT INTO ${API_SUBSCRIPTION_PLAN_MAPPINGS_TABLE} (uuid, plan_uuid, api_uuid, created_by, created_at)
       VALUES (?, ?, ?, ?, ?)`,
      [uuid, plan.planId, apiId, createdBy, now]
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
    listByApi,
    list,
    createApiMapping,
    updateApiMapping,
};
