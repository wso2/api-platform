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
const APISubscriptionPlan = require('../models/apiSubscriptionPlan');
const { APIMetadata } = require('../models/apiMetadata');
const { Sequelize } = require('sequelize');

const toUpper = (v) => (v ? String(v).toUpperCase() : null);

const computeRequestCount = (plan) => {
  const type = (plan.type || "").toLowerCase();

  if (type === "requestcount") {
    if (plan.requestCount === undefined || plan.requestCount === null) return null;
    return plan.requestCount === -1 ? "Unlimited" : String(plan.requestCount);
  }
  if (type === "eventcount") {
    if (plan.eventCount === undefined || plan.eventCount === null) return null;
    return plan.eventCount === -1 ? "Unlimited" : String(plan.eventCount);
  }
  return null;
};

const buildSubscriptionPlanRow = (orgId, plan) => {
  const requestCount = computeRequestCount(plan);

  return {
    ORG_UUID: orgId,

    // Store the APIM plan UUID if provided
    UUID: plan.planId ?? undefined,

    HANDLE: plan.planName,
    NAME: plan.displayName,
    DESCRIPTION: plan.description,
    REQUEST_COUNT: requestCount,
    REF_ID: plan.refId ?? null,
  };
};

const create = async (orgId, plan, createdBy, t) => {
  try {
    const row = buildSubscriptionPlanRow(orgId, plan);
    row.CREATED_BY = createdBy;
    row.UPDATED_BY = createdBy;

    return await SubscriptionPlan.create(row, { transaction: t });
  } catch (error) {
    if (error instanceof Sequelize.UniqueConstraintError || error instanceof Sequelize.ValidationError) {
      throw error;
    }
    throw new Sequelize.DatabaseError(error);
  }
};

const createMany = async (orgId, plans, createdBy, t) => {
  try {
    const rows = plans.map((plan) => ({
      ...buildSubscriptionPlanRow(orgId, plan),
      CREATED_BY: createdBy,
      UPDATED_BY: createdBy,
    }));

    return await SubscriptionPlan.bulkCreate(rows, { transaction: t });
  } catch (error) {
    if (error instanceof Sequelize.UniqueConstraintError || error instanceof Sequelize.ValidationError) {
      throw error;
    }
    throw new Sequelize.DatabaseError(error);
  }
};

const put = async (orgId, plan, updatedBy, t) => {
  const current = await getByName(orgId, plan.planName, t);
  if (current) {
    const updated = await update(orgId, current.UUID, plan, updatedBy, t);
    return { subscriptionPlanResponse: updated, statusCode: 200 };
  }
  const created = await create(orgId, plan, updatedBy, t);
  return { subscriptionPlanResponse: created, statusCode: 201 };
};

const update = async (orgId, planId, plan, updatedBy, t) => {
  try {
    const row = buildSubscriptionPlanRow(orgId, plan);

    // Don't update primary keys
    delete row.ORG_UUID;
    delete row.UUID;
    if (!Object.prototype.hasOwnProperty.call(plan, 'refId')) {
      delete row.REF_ID;
    }
    row.UPDATED_BY = updatedBy;
    row.UPDATED_AT = new Date();

    await SubscriptionPlan.update(row, {
      where: { UUID: planId, ORG_UUID: orgId },
      transaction: t
    });

    // `returning: true` only yields row instances on Postgres; re-fetch
    // explicitly so the result is reliable on SQLite too.
    return await SubscriptionPlan.findOne({
      where: { UUID: planId, ORG_UUID: orgId },
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
                HANDLE: planName,
                ORG_UUID: orgId
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

const deleteById = async (orgId, planId, t) => {

    try {
        const subscriptionPlanResponse = await SubscriptionPlan.destroy({
            where: {
                UUID: planId,
                ORG_UUID: orgId
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
                HANDLE: planName,
                ORG_UUID: orgId
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
};

const get = async (planId, orgId, t) => {
    try {
        const subscriptionPlanResponse = await SubscriptionPlan.findOne({
            where: {
                ORG_UUID: orgId,
                UUID: planId
            },
            transaction: t
        });
        return subscriptionPlanResponse;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const listByApi = async (apiId, t) => {

    try {
        const subscriptionPlanResponse = await SubscriptionPlan.findAll({
            include: [
                {
                    model: APIMetadata,
                    where: { UUID: apiId },
                    through: { attributes: [] }
                }
            ],
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
                ORG_UUID: orgId
            },
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
      PLAN_UUID: plan.planId,
      API_UUID: apiId,
      CREATED_BY: createdBy,
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
                PLAN_UUID: plan.planId,
                API_UUID: apiId,
                CREATED_BY: updatedBy,
            })
        }
        await APISubscriptionPlan.destroy({
            where: {
                API_UUID: apiId
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
    deleteById,
    getByName,
    get,
    listByApi,
    list,
    createApiMapping,
    updateApiMapping,
};
