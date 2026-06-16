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
const logger = require('../config/logger');

const createAPIFlow = async (orgID, viewId, apiFlowData, t) => {
    try {
        const apiFlow = await APIFlow.create({
            ORG_ID: orgID,
            VIEW_ID: viewId,
            NAME: apiFlowData.name,
            HANDLE: apiFlowData.handle,
            DESCRIPTION: apiFlowData.description,
            AGENT_PROMPT: apiFlowData.agentPrompt,
            STATUS: apiFlowData.status || 'PUBLISHED',
            VISIBILITY: apiFlowData.visibility || 'PUBLIC',
            AGENT_VISIBILITY: apiFlowData.agentVisibility || 'VISIBLE',
            FILE_CONTENT: apiFlowData.apiFlowDefinition != null ? Buffer.from(apiFlowData.apiFlowDefinition) : null,
            CONTENT_TYPE: apiFlowData.contentType || 'ARAZZO',
            CREATED_AT: new Date(),
            UPDATED_AT: new Date()
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

const updateAPIFlow = async (orgID, viewId, apiFlowId, apiFlowData, t) => {
    const updateFields = { UPDATED_AT: new Date() };
    if (apiFlowData.name !== undefined) updateFields.NAME = apiFlowData.name;
    if (apiFlowData.handle !== undefined) updateFields.HANDLE = apiFlowData.handle;
    if (apiFlowData.description !== undefined) updateFields.DESCRIPTION = apiFlowData.description;
    if (apiFlowData.agentPrompt !== undefined) updateFields.AGENT_PROMPT = apiFlowData.agentPrompt;
    if (apiFlowData.status !== undefined) updateFields.STATUS = apiFlowData.status;
    if (apiFlowData.visibility !== undefined) updateFields.VISIBILITY = apiFlowData.visibility;
    if (apiFlowData.agentVisibility !== undefined) updateFields.AGENT_VISIBILITY = apiFlowData.agentVisibility;
    if (apiFlowData.apiFlowDefinition !== undefined) updateFields.FILE_CONTENT = apiFlowData.apiFlowDefinition != null ? Buffer.from(apiFlowData.apiFlowDefinition) : null;
    if (apiFlowData.contentType !== undefined) updateFields.CONTENT_TYPE = apiFlowData.contentType;

    const [count, rows] = await APIFlow.update(updateFields, {
        where: { API_FLOW_ID: apiFlowId, ORG_ID: orgID, VIEW_ID: viewId },
        returning: true,
        transaction: t
    });
    return [count, rows];
};

const deleteAPIFlow = async (orgID, viewId, apiFlowId, t) => {
    return await APIFlow.destroy({
        where: { API_FLOW_ID: apiFlowId, ORG_ID: orgID, VIEW_ID: viewId },
        transaction: t
    });
};

const getAPIFlow = async (orgID, viewId, apiFlowId) => {
    return await APIFlow.findOne({
        where: { API_FLOW_ID: apiFlowId, ORG_ID: orgID, VIEW_ID: viewId }
    });
};

const getAPIFlowByHandle = async (orgID, viewId, handle) => {
    return await APIFlow.findOne({
        where: { HANDLE: handle, ORG_ID: orgID, VIEW_ID: viewId }
    });
};

const getAllAPIFlows = async (orgID, viewId) => {
    return await APIFlow.findAll({
        where: { ORG_ID: orgID, VIEW_ID: viewId },
        order: [['CREATED_AT', 'DESC']]
    });
};

const getPublishedAPIFlows = async (orgID, viewId, { visibility, agentVisibility } = {}) => {
    const where = { ORG_ID: orgID, VIEW_ID: viewId, STATUS: 'PUBLISHED' };
    if (visibility) where.VISIBILITY = visibility;
    if (agentVisibility) where.AGENT_VISIBILITY = agentVisibility;
    return await APIFlow.findAll({
        where,
        order: [['CREATED_AT', 'DESC']]
    });
};

const getPublishedAPIFlowByHandle = async (orgID, viewId, handle, { visibility, agentVisibility } = {}) => {
    const where = { HANDLE: handle, ORG_ID: orgID, VIEW_ID: viewId, STATUS: 'PUBLISHED' };
    if (visibility) where.VISIBILITY = visibility;
    if (agentVisibility) where.AGENT_VISIBILITY = agentVisibility;
    return await APIFlow.findOne({ where });
};

module.exports = {
    createAPIFlow,
    updateAPIFlow,
    deleteAPIFlow,
    getAPIFlow,
    getAPIFlowByHandle,
    getAllAPIFlows,
    getPublishedAPIFlows,
    getPublishedAPIFlowByHandle
};
