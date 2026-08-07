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
 

/* Live-reload: connects to the dev SSE endpoint. Only loaded when NODE_ENV=development. */
(function () {
    var connected = false, reloadPending = false;
    function connect() {
        // The SSE endpoint is mounted under the portal's BASE_PATH (see liveReload.js,
        // registered on the same parent router as every other route in app.js).
        var basePath = (window.apiPortalApi && window.apiPortalApi.basePath) || '';
        var es = new EventSource(basePath + '/__dev_reload');
        es.onopen = function () {
            if (connected && reloadPending) { location.reload(); return; }
            connected = true;
        };
        es.onmessage = function () { reloadPending = true; };
        es.onerror = function () {
            es.close();
            if (!connected) return;
            setTimeout(connect, 1000);
        };
    }
    connect();
}());
