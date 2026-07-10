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
const SubscriptionPlan = require('../models/subscriptionPlan');
const SubscriptionPlanLimit = require('../models/subscriptionPlanLimit');
const APISubscriptionPlan = require('../models/apiSubscriptionPlan');
const { APIMetadata } = require('../models/apiMetadata');
const { Sequelize } = require('sequelize');
const { v4: uuidv4 } = require('uuid');

const PLAN_INCLUDE = [{ model: SubscriptionPlanLimit, as: 'limits' }];
const LIMIT_ORDER = [[{ model: SubscriptionPlanLimit, as: 'limits' }, 'uuid', 'ASC']];
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
      throw new Sequelize.ValidationError('limitCount must be -1 (unlimited) or a positive number for each limit');
    }
    const limitType = (l.limitType || 'REQUEST_COUNT').toUpperCase();
    if (!VALID_LIMIT_TYPES.includes(limitType)) {
      throw new Sequelize.ValidationError(`limitType must be one of ${VALID_LIMIT_TYPES.join(', ')}`);
    }
    if (l.timeAmount !== undefined && l.timeAmount !== null &&
        (typeof l.timeAmount !== 'number' || !Number.isFinite(l.timeAmount) || l.timeAmount <= 0)) {
      throw new Sequelize.ValidationError('timeAmount must be a positive number when provided');
    }
    return {
      uuid: uuidv4(),
      limit_type: limitType,
      time_unit: l.timeUnit ? l.timeUnit.toUpperCase() : null,
      time_amount: l.timeAmount || 1,
      limit_count: l.limitCount,
    };
  });
};

const replaceLimits = async (planId, limits, t) => {
  await SubscriptionPlanLimit.destroy({ where: { plan_uuid: planId }, transaction: t });
  const rows = normalizeLimits(limits);
  if (rows.length === 0) return;
  await SubscriptionPlanLimit.bulkCreate(
    rows.map(r => ({ ...r, plan_uuid: planId })),
    { transaction: t }
  );
};

const create = async (orgId, plan, createdBy, t) => {
  try {
    const row = buildSubscriptionPlanRow(orgId, plan);
    row.created_by = createdBy;
    row.updated_by = createdBy;

    const created = await SubscriptionPlan.create(row, { transaction: t });
    await replaceLimits(created.uuid, plan.limits || [], t);
    return await SubscriptionPlan.findOne({ where: { uuid: created.uuid }, include: PLAN_INCLUDE, order: LIMIT_ORDER, transaction: t });
  } catch (error) {
    if (error instanceof Sequelize.UniqueConstraintError || error instanceof Sequelize.ValidationError) {
      throw error;
    }
    throw new Sequelize.DatabaseError(error);
  }
};

const createMany = async (orgId, plans, createdBy, t) => {
  try {
    const uuids = [];
    for (const plan of plans) {
      const row = { ...buildSubscriptionPlanRow(orgId, plan), created_by: createdBy, updated_by: createdBy };
      const p = await SubscriptionPlan.create(row, { transaction: t });
      await replaceLimits(p.uuid, plan.limits || [], t);
      uuids.push(p.uuid);
    }
    return await SubscriptionPlan.findAll({ where: { uuid: uuids }, include: PLAN_INCLUDE, order: LIMIT_ORDER, transaction: t });
  } catch (error) {
    if (error instanceof Sequelize.UniqueConstraintError || error instanceof Sequelize.ValidationError) {
      throw error;
    }
    throw new Sequelize.DatabaseError(error);
  }
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
  try {
    const row = buildSubscriptionPlanRow(orgId, plan);

    // Don't update primary keys
    delete row.org_uuid;
    if (!Object.prototype.hasOwnProperty.call(plan, 'refId')) {
      delete row.ref_id;
    }
    row.updated_by = updatedBy;
    row.updated_at = new Date();

    await SubscriptionPlan.update(row, {
      where: { uuid: planId, org_uuid: orgId },
      transaction: t
    });

    if (Object.prototype.hasOwnProperty.call(plan, 'limits')) {
      await replaceLimits(planId, plan.limits || [], t);
    }

    return await SubscriptionPlan.findOne({
      where: { uuid: planId, org_uuid: orgId },
      include: PLAN_INCLUDE,
      order: LIMIT_ORDER,
      transaction: t
    });
  } catch (error) {
    if (error instanceof Sequelize.UniqueConstraintError || error instanceof Sequelize.ValidationError) {
      throw error;
    }
    throw new Sequelize.DatabaseError(error);
  }
};

const deletePlan = async (orgId, planName, t) => {

    try {
        const subscriptionPlanResponse = await SubscriptionPlan.destroy({
            where: {
                handle: planName,
                org_uuid: orgId
            },
            transaction: t
        });
        return subscriptionPlanResponse;
    } catch (error) {
        if (error instanceof Sequelize.ValidationError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getByName = async (orgId, planName, t) => {

    try {
        const subscriptionPlanResponse = await SubscriptionPlan.findOne({
            where: {
                handle: planName,
                org_uuid: orgId
            },
            include: PLAN_INCLUDE,
            order: LIMIT_ORDER,
            transaction: t
        });
        return subscriptionPlanResponse;
    } catch (error) {
        if (error instanceof Sequelize.ValidationError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const listByApi = async (apiId, t) => {

    try {
        const subscriptionPlanResponse = await SubscriptionPlan.findAll({
            include: [
                {
                    model: APIMetadata,
                    where: { uuid: apiId },
                    through: { attributes: [] }
                },
                ...PLAN_INCLUDE,
            ],
            order: LIMIT_ORDER,
            transaction: t
        });
        return subscriptionPlanResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const list = async (orgId, t) => {
    try {

        const subscriptionPlansResponse = await SubscriptionPlan.findAll({
            where: {
                org_uuid: orgId
            },
            include: PLAN_INCLUDE,
            order: LIMIT_ORDER,
            transaction: t
        });
        return subscriptionPlansResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const createApiMapping = async (apiSubscriptionPlans, apiId, createdBy, t) => {
  try {
    const rows = apiSubscriptionPlans.map((plan) => ({
      plan_uuid: plan.planId,
      api_uuid: apiId,
      created_by: createdBy,
    }));

    return await APISubscriptionPlan.bulkCreate(rows, { transaction: t });
  } catch (error) {
    if (error instanceof Sequelize.ValidationError) throw error;
    throw new Sequelize.DatabaseError(error);
  }
};

const updateApiMapping = async (subscriptionPlans, apiId, updatedBy, t) => {

    let plansToCreate = [];
    try {
        for (const plan of subscriptionPlans) {
            plansToCreate.push({
                plan_uuid: plan.planId,
                api_uuid: apiId,
                created_by: updatedBy,
            })
        }
        await APISubscriptionPlan.destroy({
            where: {
                api_uuid: apiId
            },
            transaction: t
        });
        if (plansToCreate.length > 0) {
            return await APISubscriptionPlan.bulkCreate(plansToCreate, { transaction: t });
        }
        return plansToCreate;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

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
