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
    ORG_ID: orgID,

    // Store the APIM plan UUID if provided
    PLAN_ID: plan.planId ?? plan.planID ?? undefined,

    PLAN_NAME: plan.planName,
    DISPLAY_NAME: plan.displayName,
    DESCRIPTION: plan.description,
    REQUEST_COUNT: requestCount,
    REF_ID: plan.refId ?? null,
  };
};

const create = async (orgID, plan, t) => {
  try {
    const row = buildSubscriptionPlanRow(orgID, plan);

    return await SubscriptionPlan.create(row, { transaction: t });
  } catch (error) {
    if (error instanceof Sequelize.UniqueConstraintError || error instanceof Sequelize.ValidationError) {
      throw error;
    }
    throw new Sequelize.DatabaseError(error);
  }
};

const createMany = async (orgID, plans, t) => {
  try {
    const rows = plans.map((plan) => buildSubscriptionPlanRow(orgID, plan));

    return await SubscriptionPlan.bulkCreate(rows, { transaction: t });
  } catch (error) {
    if (error instanceof Sequelize.UniqueConstraintError || error instanceof Sequelize.ValidationError) {
      throw error;
    }
    throw new Sequelize.DatabaseError(error);
  }
};

const put = async (orgID, plan, t) => {
  const current = await getByName(orgID, plan.planName, t);
  if (current) {
    const updated = await update(orgID, current.PLAN_ID, plan, t);
    return { subscriptionPlanResponse: updated, statusCode: 200 };
  }
  const created = await create(orgID, plan, t);
  return { subscriptionPlanResponse: created, statusCode: 201 };
};

const update = async (orgID, planID, plan, t) => {
  try {
    const row = buildSubscriptionPlanRow(orgID, plan);

    // Don't update primary keys
    delete row.ORG_ID;
    delete row.PLAN_ID;
    if (!Object.prototype.hasOwnProperty.call(plan, 'refId')) {
      delete row.REF_ID;
    }

    await SubscriptionPlan.update(row, {
      where: { PLAN_ID: planID, ORG_ID: orgID },
      transaction: t
    });

    // `returning: true` only yields row instances on Postgres; re-fetch
    // explicitly so the result is reliable on SQLite too.
    return await SubscriptionPlan.findOne({
      where: { PLAN_ID: planID, ORG_ID: orgID },
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
                PLAN_NAME: planName,
                ORG_ID: orgID
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
                PLAN_ID: planID,
                ORG_ID: orgID
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
                PLAN_NAME: planName,
                ORG_ID: orgID
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
                ORG_ID: orgID,
                PLAN_ID: planID
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
                    where: { API_ID: apiID },
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
                ORG_ID: orgID
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

const createApiMapping = async (apiSubscriptionPlans, apiID, t) => {
  try {
    const rows = apiSubscriptionPlans.map((plan) => ({
      PLAN_ID: plan.planId ?? plan.planID,
      API_ID: apiID,
    }));

    return await APISubscriptionPlan.bulkCreate(rows, { transaction: t });
  } catch (error) {
    if (error instanceof Sequelize.ValidationError) throw error;
    throw new Sequelize.DatabaseError(error);
  }
};

const updateApiMapping = async (subscriptionPlans, apiID, t) => {

    let plansToCreate = [];
    try {
        for (const plan of subscriptionPlans) {
            plansToCreate.push({
                PLAN_ID: plan.planId ?? plan.planID,
                API_ID: apiID,
            })
        }
        await APISubscriptionPlan.destroy({
            where: {
                API_ID: apiID
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
