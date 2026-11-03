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

import { http as mswHttp, HttpResponse } from 'msw';
import { beforeEach, describe, expect, it } from 'vitest';

import {
  accepts,
  apiUrl,
  failure,
  listEnvelope,
  noContent,
  recorder,
  resource,
  type Recorder,
} from '../../../test/msw';
import { server } from '../../../test/server';
import { ApiError } from '../../core/errors';
import { resetHttpClient } from '../../core/http';
import {
  createApiDocument,
  deleteApiDocument,
  getApiDocument,
  getApiDocumentContent,
  listApiDocuments,
  updateApiDocument,
  type ApiDocument,
} from './apiDocuments.endpoints';
import { nextDocumentsOffset } from './apiDocuments.queries';

/**
 * Contract tests for `/apis/{apiType}/{apiId}/docs`.
 *
 * Writes are multipart. As in the secrets tests, jsdom cannot serialise a
 * FormData body onto the wire, so what is asserted is that a multipart body is
 * built (not labelled JSON) and that absent optional fields stay absent.
 */

const COLLECTION = '/apis/rest-api/orders-api/docs';

const aDocument = (overrides: Partial<ApiDocument> = {}): ApiDocument => ({
  contentType: 'text/markdown; charset=utf-8',
  displayName: 'Getting started',
  id: 'getting-started',
  type: 'HowTo',
  ...overrides,
});

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('listApiDocuments', () => {
  it('GETs the API’s collection with paging and type filter', async () => {
    server.use(resource(COLLECTION, listEnvelope([]), { record: requests }));

    await listApiDocuments('rest-api', 'orders-api', { limit: 5, offset: 10, type: 'HowTo' });

    const request = requests.last();
    expect(request?.method).toBe('GET');
    expect(request?.url.pathname).toBe('/api/v0.9/apis/rest-api/orders-api/docs');
    expect(request?.params.get('limit')).toBe('5');
    expect(request?.params.get('offset')).toBe('10');
    expect(request?.params.get('type')).toBe('HowTo');
  });

  it('URL-encodes the API handle', async () => {
    server.use(resource('/apis/rest-api/:apiId/docs', listEnvelope([]), { record: requests }));

    await listApiDocuments('rest-api', 'a b/c');

    expect(requests.last()?.url.pathname).toBe('/api/v0.9/apis/rest-api/a%20b%2Fc/docs');
  });
});

describe('getApiDocument', () => {
  it('GETs one document’s metadata', async () => {
    server.use(resource(`${COLLECTION}/getting-started`, aDocument(), { record: requests }));

    const document = await getApiDocument('rest-api', 'orders-api', 'getting-started');

    expect(document.displayName).toBe('Getting started');
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/apis/rest-api/orders-api/docs/getting-started');
  });

  it('surfaces a missing document as NOT_FOUND', async () => {
    server.use(failure('get', `${COLLECTION}/gone`, 404, 'NOT_FOUND'));

    await expect(getApiDocument('rest-api', 'orders-api', 'gone')).rejects.toMatchObject({
      code: 'NOT_FOUND',
    });
    await expect(getApiDocument('rest-api', 'orders-api', 'gone')).rejects.toBeInstanceOf(ApiError);
  });
});

describe('getApiDocumentContent', () => {
  it('GETs the content sub-resource as text with its stored content type', async () => {
    server.use(
      mswHttp.get(apiUrl(`${COLLECTION}/getting-started/content`), async ({ request }) => {
        await requests.capture(request);
        return new HttpResponse('# Getting started\n\n{"not": "parsed"}', {
          headers: { 'Content-Type': 'text/markdown; charset=utf-8' },
        });
      })
    );

    const content = await getApiDocumentContent('rest-api', 'orders-api', 'getting-started');

    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/apis/rest-api/orders-api/docs/getting-started/content'
    );
    // Left as text: a body that happens to contain JSON is not parsed.
    expect(content).toEqual({
      contentType: 'text/markdown; charset=utf-8',
      text: '# Getting started\n\n{"not": "parsed"}',
    });
  });

  it('reads a 204 (nothing stored) as empty text', async () => {
    server.use(
      mswHttp.get(apiUrl(`${COLLECTION}/empty/content`), () => new HttpResponse(null, { status: 204 }))
    );

    const content = await getApiDocumentContent('rest-api', 'orders-api', 'empty');

    expect(content.text).toBe('');
  });
});

describe('createApiDocument', () => {
  it('POSTs a multipart body, omitting absent fields', async () => {
    server.use(accepts('post', COLLECTION, aDocument(), { record: requests }));

    await createApiDocument('rest-api', 'orders-api', {
      displayName: 'Getting started',
      fileName: undefined,
      inlineContent: '# Getting started',
      type: 'HowTo',
    });

    const request = requests.last();
    expect(request?.method).toBe('POST');
    expect(request?.headers.get('content-type') ?? '').not.toContain('application/json');
    expect(request?.body).not.toContain('undefined');
  });
});

describe('updateApiDocument', () => {
  it('PUTs to the document', async () => {
    server.use(accepts('put', `${COLLECTION}/getting-started`, aDocument(), { record: requests }));

    await updateApiDocument('rest-api', 'orders-api', 'getting-started', { displayName: 'Start here', type: 'HowTo' });

    expect(requests.last()?.method).toBe('PUT');
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/apis/rest-api/orders-api/docs/getting-started');
  });
});

describe('deleteApiDocument', () => {
  it('DELETEs the document', async () => {
    server.use(noContent('delete', `${COLLECTION}/getting-started`, { record: requests }));

    await deleteApiDocument('rest-api', 'orders-api', 'getting-started');

    expect(requests.last()?.method).toBe('DELETE');
  });
});

describe('nextDocumentsOffset', () => {
  it('advances by the page just loaded until the total is reached', () => {
    expect(nextDocumentsOffset(listEnvelope([aDocument()], { limit: 1, offset: 0, total: 3 }))).toBe(1);
    expect(nextDocumentsOffset(listEnvelope([aDocument()], { limit: 1, offset: 2, total: 3 }))).toBeUndefined();
  });

  it('stops on an empty page even if the total disagrees', () => {
    expect(nextDocumentsOffset(listEnvelope([], { limit: 10, offset: 10, total: 30 }))).toBeUndefined();
  });
});
