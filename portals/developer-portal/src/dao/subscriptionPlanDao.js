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

const buildSubscriptionPlanRow = (orgID, plan) => {
  const requestCount = computeRequestCount(plan);

  return {
    ORG_UUID: orgID,

    // Store the APIM plan UUID if provided
    UUID: plan.planId ?? plan.planID ?? undefined,

    HANDLE: plan.planName,
    NAME: plan.displayName,
    DESCRIPTION: plan.description,
    REQUEST_COUNT: requestCount,
    REF_ID: plan.refId ?? null,
  };
};

const create = async (orgID, plan, createdBy, t) => {
  try {
    const row = buildSubscriptionPlanRow(orgID, plan);
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

const createMany = async (orgID, plans, createdBy, t) => {
  try {
    const rows = plans.map((plan) => ({
      ...buildSubscriptionPlanRow(orgID, plan),
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

const put = async (orgID, plan, updatedBy, t) => {
  const current = await getByName(orgID, plan.planName, t);
  if (current) {
    const updated = await update(orgID, current.UUID, plan, updatedBy, t);
    return { subscriptionPlanResponse: updated, statusCode: 200 };
  }
  const created = await create(orgID, plan, updatedBy, t);
  return { subscriptionPlanResponse: created, statusCode: 201 };
};

const update = async (orgID, planID, plan, updatedBy, t) => {
  try {
    const row = buildSubscriptionPlanRow(orgID, plan);

    // Don't update primary keys
    delete row.ORG_UUID;
    delete row.UUID;
    if (!Object.prototype.hasOwnProperty.call(plan, 'refId')) {
      delete row.REF_ID;
    }
    row.UPDATED_BY = updatedBy;
    row.UPDATED_AT = new Date();

    await SubscriptionPlan.update(row, {
      where: { UUID: planID, ORG_UUID: orgID },
      transaction: t
    });

    // `returning: true` only yields row instances on Postgres; re-fetch
    // explicitly so the result is reliable on SQLite too.
    return await SubscriptionPlan.findOne({
      where: { UUID: planID, ORG_UUID: orgID },
      transaction: t
    });
  } catch (error) {
    if (error instanceof Sequelize.UniqueConstraintError || error instanceof Sequelize.ValidationError) {
      throw error;
    }
    throw new Sequelize.DatabaseError(error);
  }
};

const deletePlan = async (orgID, planName, t) => {

    try {
        const subscriptionPlanResponse = await SubscriptionPlan.destroy({
            where: {
                HANDLE: planName,
                ORG_UUID: orgID
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

const deleteById = async (orgID, planID, t) => {

    try {
        const subscriptionPlanResponse = await SubscriptionPlan.destroy({
            where: {
                UUID: planID,
                ORG_UUID: orgID
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

const getByName = async (orgID, planName, t) => {

    try {
        const subscriptionPlanResponse = await SubscriptionPlan.findOne({
            where: {
                HANDLE: planName,
                ORG_UUID: orgID
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

const get = async (planID, orgID, t) => {
    try {
        const subscriptionPlanResponse = await SubscriptionPlan.findOne({
            where: {
                ORG_UUID: orgID,
                UUID: planID
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

const listByApi = async (apiID, t) => {

    try {
        const subscriptionPlanResponse = await SubscriptionPlan.findAll({
            include: [
                {
                    model: APIMetadata,
                    where: { UUID: apiID },
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

const list = async (orgID, t) => {
    try {

        const subscriptionPlansResponse = await SubscriptionPlan.findAll({
            where: {
                ORG_UUID: orgID
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

const createApiMapping = async (apiSubscriptionPlans, apiID, createdBy, t) => {
  try {
    const rows = apiSubscriptionPlans.map((plan) => ({
      PLAN_UUID: plan.planId ?? plan.planID,
      API_UUID: apiID,
      CREATED_BY: createdBy,
    }));

    return await APISubscriptionPlan.bulkCreate(rows, { transaction: t });
  } catch (error) {
    if (error instanceof Sequelize.ValidationError) throw error;
    throw new Sequelize.DatabaseError(error);
  }
};

const updateApiMapping = async (subscriptionPlans, apiID, updatedBy, t) => {

    let plansToCreate = [];
    try {
        for (const plan of subscriptionPlans) {
            plansToCreate.push({
                PLAN_UUID: plan.planId ?? plan.planID,
                API_UUID: apiID,
                CREATED_BY: updatedBy,
            })
        }
        await APISubscriptionPlan.destroy({
            where: {
                API_UUID: apiID
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
