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
 * Tiny middleware-chain helper for OpenAPI operation handlers.
 *
 * Example:
 *   const { requireCsrfForMutatingApi } = require('../../../middlewares/csrfProtection');
 *   const apiKeyController = require('../../../controllers/apiKeyController');
 *   exports.generateApiKey = compose(requireCsrfForMutatingApi, apiKeyController.generateApiKey);
 */

function compose(...fns) {
    const stack = fns.flat().filter(fn => typeof fn === 'function');
    return function composed(req, res, next) {
        let i = 0;
        const run = (err) => {
            if (err) return next(err);
            if (res.headersSent) return;
            const fn = stack[i++];
            if (!fn) return next();
            try {
                const ret = fn(req, res, run);
                if (ret && typeof ret.catch === 'function') ret.catch(run);
            } catch (e) {
                run(e);
            }
        };
        run();
    };
}

module.exports = { compose };
