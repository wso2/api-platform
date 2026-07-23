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
const { config } = require('../../config/configLoader');
const tryoutProxyController = require('../../controllers/tryoutProxyController');

// Mounted with `use` rather than a wildcard route: the target URL is appended
// to this path by Stoplight Elements ("…/tryout-proxy/https://host/path?q=1"),
// so the tail is an arbitrary URL rather than a well-formed path, and prefix
// mounting matches it without depending on how path-to-regexp treats the "//"
// in the scheme. Every HTTP method is accepted — try-it can issue any of them.
router.use(
    '/:orgName/views/:viewName/:apiType/:apiHandle/tryout-proxy',
    // The body is relayed byte-for-byte whatever its content type, so it is read
    // as a raw Buffer. app.js deliberately skips its JSON/urlencoded parsers for
    // this path — once those consume the stream the original bytes are gone.
    express.raw({ type: '*/*', limit: config.tryout?.maxRequestBytes || 1048576 }),
    tryoutProxyController.proxyTryoutRequest
);

module.exports = router;
