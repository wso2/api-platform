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
const { APIFlow } = require('../models/apiFlow');
const { Sequelize } = require('sequelize');
const constants = require('../utils/constants');
const logger = require('../config/logger');

const create = async (orgId, viewId, apiFlowData, createdBy, t) => {
    try {
        const apiFlow = await APIFlow.create({
            org_uuid: orgId,
            view_uuid: viewId,
            name: apiFlowData.name,
            handle: apiFlowData.handle,
            description: apiFlowData.description,
            agent_prompt: apiFlowData.agentPrompt,
            status: apiFlowData.status || constants.API_FLOW_STATUS.PUBLISHED,
            agent_visibility: apiFlowData.agentVisibility || constants.AGENT_VISIBILITY.VISIBLE,
            file_content: apiFlowData.apiFlowDefinition != null ? Buffer.from(apiFlowData.apiFlowDefinition) : null,
            content_type: apiFlowData.contentType || constants.API_FLOW_CONTENT_TYPE.ARAZZO,
            created_by: createdBy,
            updated_by: createdBy,
            created_at: new Date(),
            updated_at: new Date()
        }, { transaction: t });
        return apiFlow;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        logger.error('Error creating APIFlow', { error: error.message });
        throw new Sequelize.DatabaseError(error);
    }
};

const update = async (orgId, viewId, apiFlowId, apiFlowData, updatedBy, t) => {
    const updateFields = { updated_at: new Date(), updated_by: updatedBy };
    if (apiFlowData.name !== undefined) updateFields.name = apiFlowData.name;
    if (apiFlowData.handle !== undefined) updateFields.handle = apiFlowData.handle;
    if (apiFlowData.description !== undefined) updateFields.description = apiFlowData.description;
    if (apiFlowData.agentPrompt !== undefined) updateFields.agent_prompt = apiFlowData.agentPrompt;
    if (apiFlowData.status !== undefined) updateFields.status = apiFlowData.status;
    if (apiFlowData.agentVisibility !== undefined) updateFields.agent_visibility = apiFlowData.agentVisibility;
    if (apiFlowData.apiFlowDefinition !== undefined) updateFields.file_content = apiFlowData.apiFlowDefinition != null ? Buffer.from(apiFlowData.apiFlowDefinition) : null;
    if (apiFlowData.contentType !== undefined) updateFields.content_type = apiFlowData.contentType;

    const [count, rows] = await APIFlow.update(updateFields, {
        where: { uuid: apiFlowId, org_uuid: orgId, view_uuid: viewId },
        returning: true,
        transaction: t
    });
    return [count, rows];
};

const deleteFlow = async (orgId, viewId, apiFlowId, t) => {
    return await APIFlow.destroy({
        where: { uuid: apiFlowId, org_uuid: orgId, view_uuid: viewId },
        transaction: t
    });
};

const get = async (orgId, viewId, apiFlowId) => {
    return await APIFlow.findOne({
        where: { uuid: apiFlowId, org_uuid: orgId, view_uuid: viewId }
    });
};

const getByHandle = async (orgId, viewId, handle) => {
    return await APIFlow.findOne({
        where: { handle: handle, org_uuid: orgId, view_uuid: viewId }
    });
};

const list = async (orgId, viewId) => {
    return await APIFlow.findAll({
        where: { org_uuid: orgId, view_uuid: viewId },
        order: [['created_at', 'DESC']]
    });
};

const listPublished = async (orgId, viewId, { agentVisibility } = {}) => {
    const where = { org_uuid: orgId, view_uuid: viewId, status: 'PUBLISHED' };
    if (agentVisibility) where.agent_visibility = agentVisibility;
    return await APIFlow.findAll({
        where,
        order: [['created_at', 'DESC']]
    });
};

const getPublishedByHandle = async (orgId, viewId, handle, { agentVisibility } = {}) => {
    const where = { handle: handle, org_uuid: orgId, view_uuid: viewId, status: 'PUBLISHED' };
    if (agentVisibility) where.agent_visibility = agentVisibility;
    return await APIFlow.findOne({ where });
};

module.exports = {
    create,
    update,
    delete: deleteFlow,
    get,
    getByHandle,
    list,
    listPublished,
    getPublishedByHandle
};
