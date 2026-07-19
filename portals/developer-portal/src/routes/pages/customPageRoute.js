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
const express = require('express');
const router = express.Router();
const contentController = require('../../controllers/customContentController');
const registerPartials = require('../../middlewares/registerPartials');
const { ensureAuthenticated } = require('../../middlewares/ensureAuthenticated');

// Exclude specific paths. Express 5 / path-to-regexp v8 no longer accepts bare `*`
// or prefix globs in string paths, so the glob entries are expressed as RegExps
// (a RegExp path is matched directly, bypassing path-to-regexp).
router.get([
    '/favicon.ico',
    /^\/images\//,
    /^\/technical-styles\//,
    /^\/styles\//,
    /^\/login/,
    /^\/portal/,
  ], (req, res) => {
    res.status(404).send('Not found');
  });

// Trailing `*` -> named wildcard `*splat` (v8 requires named wildcards). The handler
// derives the file path from req.originalUrl, so the captured param is unused.
router.get('/:orgName/views/:viewName/*splat', registerPartials, ensureAuthenticated, contentController.loadCustomContent);

module.exports = router;