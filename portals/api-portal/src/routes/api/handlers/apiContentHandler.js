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
 * Tag: API Content
 */
const apiMetadataService = require('../../../services/apiMetadataService');
const { requireCsrfForMutatingApi } = require('../../../middlewares/csrfProtection');
const { compose } = require('./compose');

// Mutating operations require a CSRF token for browser cookie sessions (the admin UI's
// content-upload flow); Bearer / API-key / mTLS callers skip the check inside the middleware,
// so CLI and service integrations are unaffected. Mirrors organizationContentHandler.
module.exports = {
    createApiContent: compose(requireCsrfForMutatingApi, apiMetadataService.createAPIContent),
    replaceApiContent: compose(requireCsrfForMutatingApi, apiMetadataService.updateAPIContent),
    getApiContentFile: apiMetadataService.getAPIFile,
    deleteApiContentFile: compose(requireCsrfForMutatingApi, apiMetadataService.deleteAPIFile),
};
