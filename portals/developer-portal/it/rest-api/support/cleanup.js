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

// Per-suite resource cleanup so specs don't pile up APIs / MCP servers / labels /
// views / subscriptions / etc. in the single shared org (file-based auth is
// one-org, so every spec runs against `default`). globalTeardown can't do this —
// it runs in its own process with no view of what the specs created — so cleanup
// happens in an afterAll registered for every suite via jest.config.js's
// setupFilesAfterEnv (support/autoCleanup.js). This module's state is shared
// within a single spec file's worker (Node's require cache), which is exactly the
// scope that afterAll tears down.
//
// Coverage:
//   - JSON creations via client.as(role).post('/<collection>', {...}) are tracked
//     AUTOMATICALLY by the client (autoTrackFromResponse below) — no per-spec code.
//   - Multipart creations (POST /apis, /mcp-servers) go through fixtures.createApi
//     or a spec's own helper, which call trackResource() explicitly.

// NOTE: `client` is required lazily inside cleanupResources() — client.js requires
// this module (for the auto-track hook), so requiring it back at top would be a
// load-time cycle. By call time (afterAll) client is fully initialised.

// Top-level collections whose POST /<collection> creates a resource deletable via
// DELETE /<collection>/<id>. Only EXACT collection-root paths are tracked, so
// sub-resource posts (e.g. /apis/{id}/api-keys, /subscriptions/{id}/change-plan)
// are never mistaken for a top-level resource.
const TRACKABLE_COLLECTIONS = new Set([
    'apis', 'mcp-servers', 'labels', 'views', 'applications',
    'key-managers', 'subscriptions', 'api-workflows', 'webhook-subscribers',
]);

// { collection, id, role } entries for top-level resources to DELETE /{collection}/{id}.
const registry = [];

// Register a created top-level resource for deletion after the suite.
//   collection — the API path segment, e.g. 'apis', 'mcp-servers', 'labels'.
//   id         — the resource id/handle used in DELETE /{collection}/{id}.
//   role       — the auth role that owns it (must be logged in); it created the
//                resource, so it has both a live session and the delete scope.
function trackResource(collection, id, role = 'admin') {
    if (collection && id) {
        registry.push({ collection, id, role });
    }
}

// Called by the client after every as(role).post(). Registers a resource when the
// POST targeted a top-level collection root and succeeded. The id comes from the
// request body (`{ id }` for labels/views/apps/…) or the response
// (`id`/`subscriptionId`).
function autoTrackFromResponse(path, body, res, role) {
    const segments = String(path || '').split('?')[0].replace(/^\/+/, '').split('/');
    if (segments.length !== 1) return; // sub-resource / nested path — not a top-level create
    const collection = segments[0];
    if (!TRACKABLE_COLLECTIONS.has(collection)) return;
    if (!res || res.status < 200 || res.status >= 300) return;
    const id = (body && body.id) || (res.body && (res.body.id || res.body.subscriptionId));
    trackResource(collection, id, role);
}

// Delete everything tracked, newest-first so dependents (e.g. a subscription) are
// removed before their parent (e.g. the API it points at). A resource whose
// delete is refused because a sibling still references it (the API delete guards
// return 409 while subscriptions/active keys exist) is retried on the next pass,
// once that sibling is gone; passes stop as soon as one makes no progress.
// Best-effort throughout: a 404 means a test already deleted it, and cleanup
// failures (blocked deletes, an expired session) never fail the suite.
async function cleanupResources() {
    const client = require('./client'); // lazy — avoids a load-time require cycle
    let pending = registry.splice(0); // drain the registry into the working set
    const MAX_PASSES = 3;
    for (let pass = 0; pass < MAX_PASSES && pending.length; pass++) {
        const blocked = [];
        // Newest-first within a pass.
        for (let i = pending.length - 1; i >= 0; i--) {
            const entry = pending[i];
            try {
                const res = await client.as(entry.role).del(`/${entry.collection}/${entry.id}`);
                // 2xx = deleted, 404 = already gone; anything else (409/500) may
                // clear once a sibling is removed, so retry it next pass.
                if (res.status >= 400 && res.status !== 404) blocked.push(entry);
            } catch (_) {
                // session gone or transport error — drop it, cleanup is best-effort
            }
        }
        if (blocked.length === pending.length) break; // no progress — stop retrying
        pending = blocked;
    }
}

module.exports = { trackResource, autoTrackFromResponse, cleanupResources, registry };
