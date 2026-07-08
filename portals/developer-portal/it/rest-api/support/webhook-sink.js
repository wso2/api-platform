// --------------------------------------------------------------------
// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
// --------------------------------------------------------------------

// A minimal HTTP receiver that specs register as a dp_webhook_subscribers
// target_url, so the real dispatcher/delivery worker can POST to it and tests
// can assert on the payload/signature actually delivered over the wire.

const http = require('http');

// WEBHOOK_SINK_URL (docker-compose.test*.yaml) gives the *hostname* the devportal
// container must use to reach this test container (its own service name, e.g.
// "rest-api-tests") — plain "localhost" wouldn't resolve across containers. But every
// */webhook-events.spec.js file needs its OWN port, since Jest runs spec files in
// parallel worker processes within this one container and they'd otherwise all try to
// bind whatever single port the env var's URL happens to carry. So: take the hostname
// from WEBHOOK_SINK_URL when set, but always keep the caller's own defaultPort — never
// the env var's port.
function resolveSinkUrl(defaultPort) {
    const hostname = process.env.WEBHOOK_SINK_URL
        ? new URL(process.env.WEBHOOK_SINK_URL).hostname
        : 'localhost';
    return new URL(`http://${hostname}:${defaultPort}`);
}

function createWebhookSink() {
    const received = [];
    let server;
    // Overrides the HTTP status returned to the NEXT request only, then resets —
    // lets tests simulate a delivery failure without a long-lived stateful mode.
    let nextResponseStatus = 200;

    function respondWith(status) {
        nextResponseStatus = status;
    }

    function start(port = 0) {
        return new Promise((resolve, reject) => {
            server = http.createServer((req, res) => {
                let rawBody = '';
                req.on('data', (chunk) => { rawBody += chunk; });
                req.on('end', () => {
                    const status = nextResponseStatus;
                    nextResponseStatus = 200;
                    // Guard the parse: a non-JSON payload would otherwise throw
                    // inside this server callback and crash the Jest worker. On
                    // failure keep `body` null so the assertion fails clearly
                    // (with rawBody available) instead of the exception escaping.
                    let body = null;
                    if (rawBody) {
                        try {
                            body = JSON.parse(rawBody);
                        } catch {
                            body = null;
                        }
                    }
                    received.push({
                        headers: req.headers,
                        rawBody,
                        body,
                        receivedAt: new Date(),
                    });
                    res.writeHead(status, { 'Content-Type': 'application/json' });
                    res.end(JSON.stringify({ ok: status < 300 }));
                });
            });
            server.on('error', reject);
            server.listen(port, () => resolve(server.address().port));
        });
    }

    function stop() {
        return new Promise((resolve) => (server ? server.close(resolve) : resolve()));
    }

    // Delivered envelope shape (src/services/webhooks/deliveryWorker.js):
    // { event_id, event_type, occurred_at, org: { ref_id }, encrypted_fields: [], data: {...} }
    // Signature, when the subscriber has a secret, arrives as X-Devportal-Signature.
    function findDeliveryFor(eventType) {
        return received.find((r) => r.body?.event_type === eventType);
    }

    return { start, stop, received, findDeliveryFor, respondWith };
}

module.exports = { createWebhookSink, resolveSinkUrl };
