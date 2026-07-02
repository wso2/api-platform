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
const { APIWorkflow } = require('../models/apiWorkflow');
const { Sequelize } = require('sequelize');
const constants = require('../utils/constants');
const logger = require('../config/logger');

const create = async (orgId, viewId, apiWorkflowData, createdBy, t) => {
    try {
        const apiWorkflow = await APIWorkflow.create({
            org_uuid: orgId,
            view_uuid: viewId,
            name: apiWorkflowData.name,
            handle: apiWorkflowData.handle,
            description: apiWorkflowData.description,
            agent_prompt: apiWorkflowData.agentPrompt,
            status: apiWorkflowData.status || constants.API_WORKFLOW_STATUS.PUBLISHED,
            agent_visibility: apiWorkflowData.agentVisibility || constants.AGENT_VISIBILITY.VISIBLE,
            file_content: apiWorkflowData.apiWorkflowDefinition != null ? Buffer.from(apiWorkflowData.apiWorkflowDefinition) : null,
            content_type: apiWorkflowData.contentType || constants.API_WORKFLOW_CONTENT_TYPE.ARAZZO,
            created_by: createdBy,
            updated_by: createdBy,
            created_at: new Date(),
            updated_at: new Date()
        }, { transaction: t });
        return apiWorkflow;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        logger.error('Error creating API Workflow', { error: error.message });
        throw new Sequelize.DatabaseError(error);
    }
};

const update = async (orgId, viewId, apiWorkflowId, apiWorkflowData, updatedBy, t) => {
    const updateFields = { updated_at: new Date(), updated_by: updatedBy };
    if (apiWorkflowData.name !== undefined) updateFields.name = apiWorkflowData.name;
    if (apiWorkflowData.handle !== undefined) updateFields.handle = apiWorkflowData.handle;
    if (apiWorkflowData.description !== undefined) updateFields.description = apiWorkflowData.description;
    if (apiWorkflowData.agentPrompt !== undefined) updateFields.agent_prompt = apiWorkflowData.agentPrompt;
    if (apiWorkflowData.status !== undefined) updateFields.status = apiWorkflowData.status;
    if (apiWorkflowData.agentVisibility !== undefined) updateFields.agent_visibility = apiWorkflowData.agentVisibility;
    if (apiWorkflowData.apiWorkflowDefinition !== undefined) updateFields.file_content = apiWorkflowData.apiWorkflowDefinition != null ? Buffer.from(apiWorkflowData.apiWorkflowDefinition) : null;
    if (apiWorkflowData.contentType !== undefined) updateFields.content_type = apiWorkflowData.contentType;

    const [count, rows] = await APIWorkflow.update(updateFields, {
        where: { uuid: apiWorkflowId, org_uuid: orgId, view_uuid: viewId },
        returning: true,
        transaction: t
    });
    return [count, rows];
};

const deleteFlow = async (orgId, viewId, apiWorkflowId, t) => {
    return await APIWorkflow.destroy({
        where: { uuid: apiWorkflowId, org_uuid: orgId, view_uuid: viewId },
        transaction: t
    });
};

const getByHandle = async (orgId, viewId, handle) => {
    return await APIWorkflow.findOne({
        where: { handle: handle, org_uuid: orgId, view_uuid: viewId }
    });
};

const list = async (orgId, viewId) => {
    return await APIWorkflow.findAll({
        where: { org_uuid: orgId, view_uuid: viewId },
        order: [['created_at', 'DESC']]
    });
};

const listPublished = async (orgId, viewId, { agentVisibility } = {}) => {
    const where = { org_uuid: orgId, view_uuid: viewId, status: 'PUBLISHED' };
    if (agentVisibility) where.agent_visibility = agentVisibility;
    return await APIWorkflow.findAll({
        where,
        order: [['created_at', 'DESC']]
    });
};

const getPublishedByHandle = async (orgId, viewId, handle, { agentVisibility } = {}) => {
    const where = { handle: handle, org_uuid: orgId, view_uuid: viewId, status: 'PUBLISHED' };
    if (agentVisibility) where.agent_visibility = agentVisibility;
    return await APIWorkflow.findOne({ where });
};

module.exports = {
    create,
    update,
    delete: deleteFlow,
    getByHandle,
    list,
    listPublished,
    getPublishedByHandle
};
