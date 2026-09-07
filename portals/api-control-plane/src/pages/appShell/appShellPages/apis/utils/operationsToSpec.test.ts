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

import { describe, expect, it } from 'vitest';

import type { RestApi } from '@/api/resources/restApis';
import { aRestApi } from '@/test/msw';
import { restApiToOpenApiSpec } from './operationsToSpec';

const withOperations = (operations: RestApi['operations']): RestApi =>
  aRestApi({ operations }) as RestApi;

describe('restApiToOpenApiSpec', () => {
  it('groups operations under their path, keyed by lowercased method', () => {
    const spec = restApiToOpenApiSpec(
      withOperations([
        {
          description: 'Find pet by ID',
          name: 'getPetById',
          request: { method: 'GET', path: '/pet/{petId}' },
        },
        { name: 'deletePet', request: { method: 'DELETE', path: '/pet/{petId}' } },
        { request: { method: 'POST', path: '/pet' } },
      ]),
    );

    expect(spec.paths).toEqual({
      '/pet': { post: { responses: {} } },
      '/pet/{petId}': {
        delete: { responses: {}, summary: 'deletePet' },
        get: { description: 'Find pet by ID', responses: {}, summary: 'getPetById' },
      },
    });
  });

  it('titles the document from the API, falling back to its handle', () => {
    expect(
      restApiToOpenApiSpec(aRestApi({ description: 'Pizza', version: '2.1' }) as RestApi).info,
    ).toEqual({ description: 'Pizza', title: 'Pizza Shack', version: '2.1' });

    expect(
      restApiToOpenApiSpec(aRestApi({ displayName: '  ', version: '' }) as RestApi).info,
    ).toEqual({ title: 'pizza-shack', version: '' });
  });

  it('drops operations the viewer could not place: no path, no method, unknown method', () => {
    const spec = restApiToOpenApiSpec(
      withOperations([
        { request: { method: 'GET', path: '   ' } },
        // Both casts stand in for wire data the generated enum says is
        // impossible; the guard exists because the response is not validated.
        { request: { method: '' as 'GET', path: '/pet' } },
        // Not a path-item key: it would land in the document as an
        // extension field rather than as an operation.
        { request: { method: 'CONNECT' as 'GET', path: '/pet' } },
      ]),
    );

    expect(spec.paths).toEqual({});
  });

  it('yields an empty, still-renderable document for an API with no operations', () => {
    expect(restApiToOpenApiSpec(aRestApi() as RestApi)).toEqual({
      info: { title: 'Pizza Shack', version: 'v1' },
      openapi: '3.0.3',
      paths: {},
    });
  });
});
