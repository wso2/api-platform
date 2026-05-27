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
const express = require('express');
const router = express.Router();
const apiFlowsController = require('../controllers/apiFlowsController');
const registerPartials = require('../middlewares/registerPartials');
const util = require('../utils/util');

// API Workflows gallery - no authentication required
router.get('/:orgName/views/:viewName/api-workflows', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, registerPartials, util.enforcePortalMode, apiFlowsController.loadAPIFlows);

// Get all published workflows as Markdown
router.get('/:orgName/views/:viewName/api-workflows.md', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, apiFlowsController.getAllPublishedFlowsMD);

// Get raw Arazzo specification as JSON
router.get('/:orgName/views/:viewName/api-workflows/:handle/arazzo.json', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, apiFlowsController.getWorkflowArazzoSpec);

// API to get workflow prompt as JSON
router.get('/:orgName/views/:viewName/api-workflows/:handle/prompt', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, apiFlowsController.getFlowPromptJSON);

// Get workflow detail as Markdown
router.get('/:orgName/views/:viewName/api-workflows/:handle.md', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, apiFlowsController.getWorkflowDetailMd);

// API Workflow detail page (generic :handle route - must be last)
router.get('/:orgName/views/:viewName/api-workflows/:handle', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, registerPartials, util.enforcePortalMode, apiFlowsController.loadAPIFlowDetail);

// Generate agent prompt from metadata
router.post('/:orgName/views/:viewName/api-flows/generate-prompt', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, apiFlowsController.generatePrompt);

module.exports = router;
