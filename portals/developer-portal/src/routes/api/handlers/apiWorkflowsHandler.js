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
 *
 */

/*
 * Tag: API Workflows
 *
 * Mutating ops are CSRF-protected with compose(requireCsrfForMutatingApi, serviceFn). Non-mutating ops are just serviceFn.
 */

const apiWorkflowService = require('../../../services/apiWorkflowService');
const { requireCsrfForMutatingApi } = require('../../../middlewares/csrfProtection');
const { compose } = require('./compose');

module.exports = {
    createApiWorkflow: compose(requireCsrfForMutatingApi, apiWorkflowService.createAPIWorkflow),
    getAllApiWorkflows: apiWorkflowService.getAllAPIWorkflows,
    getApiWorkflow: apiWorkflowService.getAPIWorkflow,
    updateApiWorkflow: compose(requireCsrfForMutatingApi, apiWorkflowService.updateAPIWorkflow),
    deleteApiWorkflow: compose(requireCsrfForMutatingApi, apiWorkflowService.deleteAPIWorkflow),
    generatePrompt: compose(requireCsrfForMutatingApi, apiWorkflowService.generatePrompt),
};
