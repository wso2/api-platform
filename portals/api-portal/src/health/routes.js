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
'use strict';

const FORBIDDEN_BODY = JSON.stringify({ status: 'forbidden' });

/**
 * Builds the handler behind /health and /ready. Both run the same checks and give the
 * same answer: liveness and readiness differ only in what the caller does with it.
 *
 * In order:
 *   1. A client outside the allowed set gets 403 before anything else happens: no check
 *      runs, the cache is not refreshed and nothing is logged. The client is the
 *      connection's peer address (req.socket.remoteAddress), never req.ip or a
 *      forwarding header, so neither `trust proxy` nor a forged X-Forwarded-For can
 *      move a caller into the allowed set.
 *   2. Methods other than GET and HEAD get 405.
 *   3. Otherwise the (possibly cached) result is served: 200 when healthy, 503 when not.
 */
function createHealthHandler({ checker, allowedClients }) {
    return (req, res, next) => {
        if (!allowedClients.allows(req.socket?.remoteAddress)) {
            res.status(403)
                .set('Cache-Control', 'no-store')
                .type('application/json')
                .send(FORBIDDEN_BODY);
            return;
        }
        if (req.method !== 'GET' && req.method !== 'HEAD') {
            res.set('Allow', 'GET, HEAD').status(405).end();
            return;
        }
        checker.evaluate().then((result) => {
            res.status(result.healthy ? 200 : 503)
                .set('Cache-Control', 'no-store')
                .json({ status: result.status, timestamp: result.timestamp, checks: result.checks });
        }, next);
    };
}

/**
 * Registers the health endpoints at the true root, where a container or Kubernetes
 * probe dials the pod directly, and under `basePath`, which is all a path-routing
 * ingress in front of a shared host ever forwards.
 */
function registerHealthRoutes(app, { checker, allowedClients, basePath }) {
    const handler = createHealthHandler({ checker, allowedClients });
    for (const path of ['/health', '/ready', `${basePath}/health`, `${basePath}/ready`]) {
        app.all(path, handler);
    }
}

module.exports = { createHealthHandler, registerHealthRoutes };
