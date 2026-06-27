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
const apiFlowDao = require('../dao/apiFlowDao');
const orgDao = require('../dao/organizationDao');
const apiDao = require('../dao/apiDao');
const viewDao = require('../dao/viewDao');
const apiFlowService = require('../services/apiFlowService');
const logger = require('../config/logger');
const { loadLayoutFromAPI, renderGivenTemplate, renderTemplateFromAPI, isAiDisabledForPortal } = require('../utils/util');
const constants = require('../utils/constants');
const { config } = require('../config/configLoader');
const fs = require('fs');
const path = require('path');
const Handlebars = require('handlebars');
const yaml = require('js-yaml');

const resolveViewId = async (orgID, viewName) => {
    return await viewDao.getId(orgID, viewName);
};


const extractSourceDescriptions = (flow) => {
    if ((flow.CONTENT_TYPE || constants.API_FLOW_CONTENT_TYPE.ARAZZO) !== constants.API_FLOW_CONTENT_TYPE.ARAZZO || !flow.FILE_CONTENT) return [];
    try {
        const raw = Buffer.isBuffer(flow.FILE_CONTENT) ? flow.FILE_CONTENT.toString('utf8') : String(flow.FILE_CONTENT);
        const spec = yaml.load(raw);
        return Array.isArray(spec?.sourceDescriptions)
            ? spec.sourceDescriptions.map(sd => ({ name: sd.name, url: sd.url || null })).filter(sd => sd.name)
            : [];
    } catch {
        return [];
    }
};

const escapeRegex = (str) => str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

const buildSpecUrlPattern = (orgName, viewName) =>
    new RegExp(
        `^(?:https?://[^/]+)?/${escapeRegex(orgName)}/views/${escapeRegex(viewName)}/(api|mcp)/([^/]+)/docs/specification\\.(json|graphql|xml)$`
    );

const resolveSourceUrls = async (sources, orgName, viewName, orgID) => {
    const pattern = buildSpecUrlPattern(orgName, viewName);
    return Promise.all(sources.map(async (source) => {
        if (!source.url) return source;
        const match = source.url.match(pattern);
        if (!match) return source;
        const [, apiType, apiHandle] = match;
        const apiId = await apiDao.getId(orgID, apiHandle);
        if (!apiId) return source;
        return { ...source, url: `/${orgName}/views/${viewName}/${apiType}/${apiHandle}.md`, isDevportalApi: true };
    }));
};


const loadAPIFlows = async (req, res, next) => {
    const { orgName, viewName } = req.params;

    try {
        const orgDetails = await orgDao.get(orgName);
        if (!orgDetails) {
            const err = Object.assign(new Error('Organization not found'), { status: 404 });
            return next(err);
        }

        const orgID = orgDetails.ID;
        const viewId = await resolveViewId(orgID, viewName);

        const apiFlows = await apiFlowDao.listPublished(orgID, viewId);

        const profile = req.user ? {
            username: req.user.sub,
            authenticated: true,
            firstName: req.user.firstName,
            lastName: req.user.lastName,
            email: req.user.email,
            imageURL: req.user.imageURL,
            isAdmin: req.user.isAdmin,
        } : null;
        const devportalMode = orgDetails.CONFIGURATION?.devportalMode || 'DEFAULT';

        const resolvedFlows = apiFlows.map(flow => {
            const sources = extractSourceDescriptions(flow);
            return {
                apiFlowId: flow.ID,
                handle: flow.HANDLE,
                name: flow.NAME,
                description: flow.DESCRIPTION,
                agentPrompt: flow.AGENT_PROMPT,
                status: flow.STATUS,
                agentVisibility: flow.AGENT_VISIBILITY || constants.AGENT_VISIBILITY.VISIBLE,
                sources,
                sourcesPreview: sources.slice(0, 4),
                sourcesMoreCount: Math.max(0, sources.length - 4)
            };
        });

        const templateContent = {
            apiFlows: resolvedFlows,
            orgName,
            viewName,
            baseUrl: `/${orgName}/views/${viewName}`,
            profile,
            devportalMode
        };

        const dbLayout = await loadLayoutFromAPI(orgID, viewName);
        let html;
        if (dbLayout) {
            const templatePath = path.join(process.cwd(), 'src/defaultContent/pages/api-workflows/page.hbs');
            const templateResponse = fs.readFileSync(templatePath, 'utf8');
            const styleContent = await orgDao.getContent({ orgId: orgID, fileType: 'style', viewName: viewName, fileName: 'main.css' });
            const themedLayout = styleContent
                ? dbLayout.replace(/\/styles\//g, `${constants.DEVPORTAL_API.orgPath(orgID)}/views/${viewName}/layout?fileType=style&fileName=`)
                : dbLayout;
            html = await renderGivenTemplate(templateResponse, themedLayout, templateContent);
        } else {
            html = await renderTemplateFromAPI(templateContent, orgID, orgName, 'pages/api-workflows', viewName);
        }
        res.send(html);
    } catch (error) {
        logger.error('Error loading API flows', {
            error: error.message,
            stack: error.stack,
            orgName,
            viewName
        });
        error.status = 500;
        return next(error);
    }
};

const loadAPIFlowDetail = async (req, res, next) => {
    const { orgName, viewName, handle } = req.params;

    try {
        const orgDetails = await orgDao.get(orgName);
        if (!orgDetails) {
            const err = Object.assign(new Error('Organization not found'), { status: 404 });
            return next(err);
        }

        const orgID = orgDetails.ID;
        const viewId = await resolveViewId(orgID, viewName);

        const apiFlow = await apiFlowDao.getPublishedByHandle(orgID, viewId, handle);

        if (!apiFlow) {
            const err = Object.assign(new Error('API Workflow not found or not published'), { status: 404 });
            return next(err);
        }

        const profile = req.user ? {
            username: req.user.sub,
            authenticated: true,
            firstName: req.user.firstName,
            lastName: req.user.lastName,
            email: req.user.email,
            imageURL: req.user.imageURL,
            isAdmin: req.user.isAdmin,
        } : null;
        const devportalMode = orgDetails.CONFIGURATION?.devportalMode || 'DEFAULT';

        const rawContent = apiFlow.FILE_CONTENT;
        let fileContentStr = '';
        if (rawContent != null) {
            fileContentStr = Buffer.isBuffer(rawContent) ? rawContent.toString('utf8') : String(rawContent);
        }

        const templateContent = {
            flow: {
                flowId: apiFlow.ID,
                name: apiFlow.NAME,
                description: apiFlow.DESCRIPTION,
                agentPrompt: apiFlow.AGENT_PROMPT,
                status: apiFlow.STATUS,
                agentVisibility: apiFlow.AGENT_VISIBILITY || constants.AGENT_VISIBILITY.VISIBLE,
                contentType: apiFlow.CONTENT_TYPE,
                content: fileContentStr,
                createdAt: apiFlow.CREATED_AT ? new Date(apiFlow.CREATED_AT).toLocaleDateString() : '',
                updatedAt: apiFlow.UPDATED_AT ? new Date(apiFlow.UPDATED_AT).toLocaleDateString() : ''
            },
            orgName,
            viewName,
            baseUrl: `/${orgName}/views/${viewName}`,
            profile,
            devportalMode
        };

        const dbLayout = await loadLayoutFromAPI(orgID, viewName);
        let html;
        if (dbLayout) {
            const templatePath = path.join(process.cwd(), 'src/defaultContent/pages/api-workflows/detail/page.hbs');
            const templateResponse = fs.readFileSync(templatePath, 'utf8');
            const styleContent = await orgDao.getContent({ orgId: orgID, fileType: 'style', viewName: viewName, fileName: 'main.css' });
            const themedLayout = styleContent
                ? dbLayout.replace(/\/styles\//g, `${constants.DEVPORTAL_API.orgPath(orgID)}/views/${viewName}/layout?fileType=style&fileName=`)
                : dbLayout;
            html = await renderGivenTemplate(templateResponse, themedLayout, templateContent);
        } else {
            html = await renderTemplateFromAPI(templateContent, orgID, orgName, 'pages/api-workflows/detail', viewName);
        }
        res.send(html);
    } catch (error) {
        logger.error('Error loading API flow detail', {
            error: error.message,
            stack: error.stack,
            orgName,
            viewName,
            handle
        });
        error.status = 500;
        return next(error);
    }
};

const getFlowPromptJSON = async (req, res) => {
    const { orgName, viewName, handle } = req.params;

    try {
        const orgDetails = await orgDao.get(orgName);
        if (!orgDetails) {
            return res.status(404).json({ error: 'Organization not found' });
        }

        const orgID = orgDetails.ID;

        if (await isAiDisabledForPortal(orgID, viewName)) {
            return res.status(404).json({ error: 'Not Found' });
        }

        const viewId = await resolveViewId(orgID, viewName);

        const apiFlow = await apiFlowDao.getPublishedByHandle(orgID, viewId, handle, { agentVisibility: 'VISIBLE' });

        if (!apiFlow) {
            return res.status(404).json({ error: 'API Workflow not found or not published' });
        }

        const rawContent = apiFlow.FILE_CONTENT;
        let content = null;
        if (rawContent != null) {
            content = Buffer.isBuffer(rawContent) ? rawContent.toString('utf8') : String(rawContent);
        }

        res.status(200).json({
            flowId: apiFlow.ID,
            handle: apiFlow.HANDLE,
            name: apiFlow.NAME,
            description: apiFlow.DESCRIPTION,
            agentPrompt: apiFlow.AGENT_PROMPT,
            contentType: apiFlow.CONTENT_TYPE,
            content,
            sources: extractSourceDescriptions(apiFlow)
        });
    } catch (error) {
        logger.error('Error fetching API workflow prompt', {
            error: error.message,
            stack: error.stack,
            orgName,
            viewName,
            handle
        });
        res.status(500).json({ error: 'Error fetching API workflow' });
    }
};

const getWorkflowDetailMd = async (req, res) => {
    const { orgName, viewName, handle } = req.params;

    try {
        const orgDetails = await orgDao.get(orgName);
        if (!orgDetails) {
            return res.status(404).send('# Error\n\nOrganization not found.');
        }

        const orgID = orgDetails.ID;

        if (await isAiDisabledForPortal(orgID, viewName)) {
            return res.status(404).send('# Not Found\n\nThis resource is not available for agents.');
        }

        const viewId = await resolveViewId(orgID, viewName);

        const apiFlow = await apiFlowDao.getPublishedByHandle(orgID, viewId, handle, { agentVisibility: 'VISIBLE' });

        if (!apiFlow) {
            return res.status(404).send('# Error\n\nWorkflow is not available for Agents or not Published.');
        }

        // Get raw content as string
        const rawContent = apiFlow.FILE_CONTENT;
        let workflowContent = '';
        if (rawContent != null) {
            workflowContent = Buffer.isBuffer(rawContent) ? rawContent.toString('utf8') : String(rawContent);
        }

        // Convert to Markdown format if the content is Arazzo JSON
        let markdownContent = workflowContent;
        if (apiFlow.CONTENT_TYPE === 'ARAZZO') {
            try {
                const arazoJson = JSON.parse(workflowContent);
                const rawSources = extractSourceDescriptions(apiFlow);
                const sources = await resolveSourceUrls(rawSources, orgName, viewName, orgID);
                markdownContent = generateWorkflowMarkdown(arazoJson, apiFlow, orgName, viewName, sources);
            } catch (e) {
                logger.warn('Could not parse Arazzo JSON, using raw content', { handle, error: e.message });
                markdownContent = workflowContent;
            }
        }

        res.setHeader('Content-Type', 'text/markdown; charset=utf-8');
        res.send(markdownContent);
    } catch (error) {
        logger.error('Error fetching workflow detail as markdown', {
            error: error.message,
            stack: error.stack,
            orgName,
            viewName,
            handle
        });
        res.status(500).send('# Error\n\nFailed to load workflow.');
    }
};

const generateWorkflowMarkdown = (arazoJson, apiFlow, orgName, viewName, sources = []) => {
    const templatePath = path.join(process.cwd(), 'src/defaultContent/pages/api-workflows/workflow-markdown.hbs');
    const templateContent = fs.readFileSync(templatePath, 'utf8');
    const template = Handlebars.compile(templateContent);

    const toEnvVarPrefix = (name) =>
        name.toUpperCase().replace(/[^A-Z0-9]+/g, '_').replace(/^_+|_+$/g, '');

    const baseUrl = `/${orgName}/views/${viewName}`;
    const data = {
        flow: {
            name: apiFlow.NAME,
            handle: apiFlow.HANDLE,
            status: apiFlow.STATUS,
            description: apiFlow.DESCRIPTION
        },
        sources: sources.map(s => ({
            ...s,
            envVarApiKey: `${toEnvVarPrefix(s.name)}_API_KEY`,
            envVarOAuth2Prefix: toEnvVarPrefix(s.name)
        })),
        baseUrl,
        arazoJson: JSON.stringify(arazoJson, null, 2)
    };

    return template(data);
};

const generateWorkflowsListMarkdown = (apiFlows, orgName, viewName, hiddenWorkflowCount = 0) => {
    const templatePath = path.join(process.cwd(), 'src/defaultContent/pages/api-workflows/workflows-list-markdown.hbs');
    const templateContent = fs.readFileSync(templatePath, 'utf8');
    const template = Handlebars.compile(templateContent);

    const baseUrl = `/${orgName}/views/${viewName}`;
    const data = {
        flows: apiFlows.map(flow => ({
            name: flow.NAME,
            handle: flow.HANDLE
        })),
        baseUrl,
        orgName,
        viewName,
        hiddenWorkflowCount,
        hasHiddenWorkflows: hiddenWorkflowCount > 0,
        portalUrl: baseUrl,
    };

    return template(data);
};

const getAllPublishedFlowsMD = async (req, res) => {
    const { orgName, viewName } = req.params;

    try {
        const orgDetails = await orgDao.get(orgName);
        if (!orgDetails) {
            return res.status(404).send('# Error\n\nOrganization not found.');
        }

        const orgID = orgDetails.ID;

        if (await isAiDisabledForPortal(orgID, viewName)) {
            return res.status(404).send('# Not Found\n\nThis resource is not available for agents.');
        }

        const viewId = await resolveViewId(orgID, viewName);

        const allPublishedFlows = await apiFlowDao.listPublished(orgID, viewId);
        const apiFlows = allPublishedFlows.filter(f => (f.AGENT_VISIBILITY || constants.AGENT_VISIBILITY.VISIBLE) !== constants.AGENT_VISIBILITY.HIDDEN);
        const hiddenWorkflowCount = allPublishedFlows.length - apiFlows.length;

        const md = generateWorkflowsListMarkdown(apiFlows, orgName, viewName, hiddenWorkflowCount);

        res.setHeader('Content-Type', 'text/markdown; charset=utf-8');
        res.send(md);
    } catch (error) {
        logger.error('Error fetching published flows as markdown', {
            error: error.message,
            stack: error.stack,
            orgName,
            viewName
        });
        res.status(500).send('# Error\n\nFailed to load workflows.');
    }
};

const generatePrompt = async (req, res) => {
    const { name, description, apis, orgName, viewName, handle } = req.body;
    try {
        const baseUrl = config.baseUrl || `${req.protocol}://${req.get('host')}`;
        const prompt = apiFlowService.generateAgentPrompt(name, description, apis || [], orgName || '', viewName || 'default', baseUrl, handle || '');
        res.status(200).json({ agentPrompt: prompt });
    } catch (error) {
        logger.error('Error generating agent prompt', { error: error.message });
        res.status(500).json({ message: 'Error generating agent prompt' });
    }
};

const getWorkflowArazzoSpec = async (req, res) => {
    const { orgName, viewName, handle } = req.params;

    try {
        const orgDetails = await orgDao.get(orgName);
        if (!orgDetails) {
            return res.status(404).json({ error: 'Organization not found' });
        }

        const orgID = orgDetails.ID;

        if (await isAiDisabledForPortal(orgID, viewName)) {
            return res.status(404).json({ error: 'Not Found' });
        }

        const viewId = await resolveViewId(orgID, viewName);

        const apiFlow = await apiFlowDao.getPublishedByHandle(orgID, viewId, handle);
        if (!apiFlow) {
            return res.status(404).json({ error: 'API Workflow not found or not published' });
        }

        if ((apiFlow.AGENT_VISIBILITY || constants.AGENT_VISIBILITY.VISIBLE) === constants.AGENT_VISIBILITY.HIDDEN) {
            return res.status(404).json({ error: 'API Workflow not found or not published' });
        }

        if (apiFlow.CONTENT_TYPE !== 'ARAZZO') {
            return res.status(404).json({ error: 'This workflow does not have an Arazzo specification' });
        }

        const rawContent = apiFlow.FILE_CONTENT;
        const content = Buffer.isBuffer(rawContent) ? rawContent.toString('utf8') : String(rawContent);

        res.setHeader('Content-Type', 'application/json; charset=utf-8');
        res.status(200).send(content);
    } catch (error) {
        logger.error('Error fetching Arazzo spec', {
            error: error.message,
            stack: error.stack,
            orgName,
            viewName,
            handle
        });
        res.status(500).json({ error: 'Error fetching Arazzo specification' });
    }
};

module.exports = {
    loadAPIFlows,
    getAllPublishedFlowsMD,
    loadAPIFlowDetail,
    getFlowPromptJSON,
    getWorkflowDetailMd,
    getWorkflowArazzoSpec,
    generatePrompt
};
