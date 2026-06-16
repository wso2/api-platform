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
const SubscriptionPolicy = require('../models/subscriptionPolicy');
const APISubscriptionPolicy = require('../models/apiSubscriptionPolicy');
const { APIMetadata } = require('../models/apiMetadata');
const { Sequelize } = require('sequelize');

const toUpper = (v) => (v ? String(v).toUpperCase() : null);

const computeRequestCount = (policy) => {
  const type = (policy.type || "").toLowerCase();

  if (type === "requestcount") {
    return policy.requestCount === -1 ? "Unlimited" : String(policy.requestCount);
  }
  if (type === "eventcount") {
    return policy.eventCount === -1 ? "Unlimited" : String(policy.eventCount);
  }
  return null;
};

const buildSubscriptionPolicyRow = (orgID, policy) => {
  const requestCount = computeRequestCount(policy);

  return {
    ORG_ID: orgID,

    // Store the APIM policy UUID if provided
    POLICY_ID: policy.policyId ?? policy.policyID ?? undefined,

    POLICY_NAME: policy.policyName,
    DISPLAY_NAME: policy.displayName,
    DESCRIPTION: policy.description,
    REQUEST_COUNT: requestCount,
    REF_ID: policy.refId ?? null,
  };
};

const create = async (orgID, policy, t) => {
  try {
    const row = buildSubscriptionPolicyRow(orgID, policy);

    return await SubscriptionPolicy.create(row, { transaction: t });
  } catch (error) {
    if (error instanceof Sequelize.UniqueConstraintError || error instanceof Sequelize.ValidationError) {
      throw error;
    }
    throw new Sequelize.DatabaseError(error);
  }
};

const createMany = async (orgID, policies, t) => {
  try {
    const rows = policies.map((policy) => buildSubscriptionPolicyRow(orgID, policy));

    return await SubscriptionPolicy.bulkCreate(rows, { transaction: t });
  } catch (error) {
    if (error instanceof Sequelize.UniqueConstraintError || error instanceof Sequelize.ValidationError) {
      throw error;
    }
    throw new Sequelize.DatabaseError(error);
  }
};

const put = async (orgID, policy, t) => {
  const current = await getByName(orgID, policy.policyName, t);
  if (current) {
    const updated = await update(orgID, current.POLICY_ID, policy, t);
    return { subscriptionPolicyResponse: updated, statusCode: 200 };
  }
  const created = await create(orgID, policy, t);
  return { subscriptionPolicyResponse: created, statusCode: 201 };
};

const update = async (orgID, policyID, policy, t) => {
  try {
    const row = buildSubscriptionPolicyRow(orgID, policy);

    // Don't update primary keys
    delete row.ORG_ID;
    delete row.POLICY_ID;
    if (!Object.prototype.hasOwnProperty.call(policy, 'refId')) {
      delete row.REF_ID;
    }

    const [_, updatedRows] = await SubscriptionPolicy.update(row, {
      where: { POLICY_ID: policyID, ORG_ID: orgID },
      returning: true,
      transaction: t
    });

    return updatedRows[0];
  } catch (error) {
    if (error instanceof Sequelize.UniqueConstraintError || error instanceof Sequelize.ValidationError) {
      throw error;
    }
    throw new Sequelize.DatabaseError(error);
  }
};

const deletePolicy = async (orgID, policyName, t) => {

    try {
        const subscriptionPolicyResponse = await SubscriptionPolicy.destroy({
            where: {
                POLICY_NAME: policyName,
                ORG_ID: orgID
            },
            transaction: t
        });
        return subscriptionPolicyResponse;
    } catch (error) {
        if (error instanceof Sequelize.ValidationError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const deleteById = async (orgID, policyID, t) => {

    try {
        const subscriptionPolicyResponse = await SubscriptionPolicy.destroy({
            where: {
                POLICY_ID: policyID,
                ORG_ID: orgID
            },
            transaction: t
        });
        return subscriptionPolicyResponse;
    } catch (error) {
        if (error instanceof Sequelize.ValidationError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getByName = async (orgID, policyName, t) => {

    try {
        const subscriptionPolicyResponse = await SubscriptionPolicy.findOne({
            where: {
                POLICY_NAME: policyName,
                ORG_ID: orgID
            },
            transaction: t
        });
        return subscriptionPolicyResponse;
    } catch (error) {
        if (error instanceof Sequelize.ValidationError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const get = async (policyID, orgID, t) => {
    try {
        const subscriptionPolicyResponse = await SubscriptionPolicy.findOne({
            where: {
                ORG_ID: orgID,
                POLICY_ID: policyID
            },
            transaction: t
        });
        return subscriptionPolicyResponse;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const listByApi = async (apiID, t) => {

    try {
        const subscriptionPolicyResponse = await SubscriptionPolicy.findAll({
            include: [
                {
                    model: APIMetadata,
                    where: { API_ID: apiID },
                    through: { attributes: [] }
                }
            ],
            transaction: t
        });
        return subscriptionPolicyResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const list = async (orgID, t) => {
    try {

        const subscriptionPoliciesResponse = await SubscriptionPolicy.findAll({
            where: {
                ORG_ID: orgID
            },
            transaction: t
        });
        return subscriptionPoliciesResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const createApiMapping = async (apiSubscriptionPolicies, apiID, t) => {
  try {
    const rows = apiSubscriptionPolicies.map((policy) => ({
      POLICY_ID: policy.policyId ?? policy.policyID,
      API_ID: apiID,
    }));

    return await APISubscriptionPolicy.bulkCreate(rows, { transaction: t });
  } catch (error) {
    if (error instanceof Sequelize.ValidationError) throw error;
    throw new Sequelize.DatabaseError(error);
  }
};

const updateApiMapping = async (subscriptionPolicies, apiID, t) => {

    let policiesToCreate = [];
    try {
        for (const policy of subscriptionPolicies) {
            policiesToCreate.push({
                POLICY_ID: policy.policyId ?? policy.policyID,
                API_ID: apiID,
            })
        }
        if (policiesToCreate.length > 0) {
            await APISubscriptionPolicy.destroy({
                where: {
                    API_ID: apiID
                },
                transaction: t
            });
            return await APISubscriptionPolicy.bulkCreate(policiesToCreate, { transaction: t });
        } else {
            return policiesToCreate;
        }
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
    delete: deletePolicy,
    deleteById,
    getByName,
    get,
    listByApi,
    list,
    createApiMapping,
    updateApiMapping,
};
