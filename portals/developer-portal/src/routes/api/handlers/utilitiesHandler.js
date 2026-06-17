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
 * Tag: Utilities
 */

const path = require('path');
const os = require('os');
const fs = require('fs').promises;
const { requireCsrfForMutatingApi } = require('../../middlewares/csrfProtection');
const { compose } = require('./compose');

function isSafeFileName(fileName) {
    return Boolean(
        fileName &&
        fileName !== '.' &&
        fileName !== '..' &&
        path.basename(fileName) === fileName
    );
}

async function createTempArazzoFileImpl(req, res, next) {
    const { content, filename } = req.body || {};
    if (!content || typeof content !== 'string') {
        return res.status(400).json({ error: 'content is required' });
    }
    const safeName = (filename || 'workflow.arazzo.yaml')
        .replace(/[^a-zA-Z0-9._-]/g, '-')
        .replace(/\.\.+/g, '.')
        .substring(0, 120);
    if (!isSafeFileName(safeName)) {
        return res.status(400).json({ error: 'filename is invalid' });
    }
    try {
        const tmpDir = await fs.mkdtemp(path.join(os.tmpdir(), 'arazzo-'));
        const tmpPath = path.join(tmpDir, safeName);
        await fs.writeFile(tmpPath, content, 'utf8');
        res.json({ path: tmpPath });
    } catch (err) {
        next(err);
    }
}

module.exports = {
    createTempArazzoFile: compose(requireCsrfForMutatingApi, createTempArazzoFileImpl),
};
