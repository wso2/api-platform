/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

/**
 * The definition "design from scratch" starts from: one collection and one
 * item, with the four operations most APIs begin with. It is a document the
 * user goes on to edit (by hand, or by asking the AI to refine it), so its
 * text is content rather than UI copy and deliberately does not go through
 * `react-intl` — the same way a code sample or a backend payload doesn't.
 */
export const DEFAULT_API_SKELETON: Record<string, unknown> = {
  openapi: '3.0.3',
  info: {
    title: 'Untitled API',
    version: '1.0.0',
    description: 'A starting point. Edit the operations, or ask AI to refine them.',
  },
  paths: {
    '/resources': {
      get: {
        operationId: 'listResources',
        summary: 'List or retrieve resources',
        responses: { '200': { description: 'A page of resources.' } },
      },
      post: {
        operationId: 'createResource',
        summary: 'Create a resource',
        responses: { '201': { description: 'The resource that was created.' } },
      },
    },
    '/resources/{resourceId}': {
      parameters: [
        {
          in: 'path',
          name: 'resourceId',
          required: true,
          schema: { type: 'string' },
        },
      ],
      put: {
        operationId: 'updateResource',
        summary: 'Update a resource',
        responses: { '200': { description: 'The resource after the update.' } },
      },
      delete: {
        operationId: 'deleteResource',
        summary: 'Delete a resource',
        responses: { '204': { description: 'The resource was deleted.' } },
      },
    },
  },
};
