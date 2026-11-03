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
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { apiUrl, failure, recorder, type Recorder } from '../../../test/msw';
import { server } from '../../../test/server';
import { http, resetHttpClient } from '../../core/http';
import {
  getApiPublicationDraftLandingPage,
  getApiPublicationDraftThumbnail,
  getApiPublicationLandingPage,
  getApiPublicationThumbnail,
  saveApiPublicationDraftLandingPage,
  saveApiPublicationDraftThumbnail,
} from './apiPublications.endpoints';

/**
 * Contract tests for the thumbnail reads on one API's draft and publication, which return raw
 * bytes (a Blob with the stored content type), and for the landing page, which is Markdown text
 * both ways. The thumbnail upload is checked at the http client, as jsdom cannot send multipart.
 */

const DRAFT_THUMBNAIL = '/api-portals/acme-portal/apis/rest-api/orders-api/draft/thumbnail';
const PUBLICATION_THUMBNAIL = '/api-portals/acme-portal/apis/rest-api/orders-api/publication/thumbnail';
const DRAFT_LANDING_PAGE = '/api-portals/acme-portal/apis/rest-api/orders-api/draft/landing-page';
const PUBLICATION_LANDING_PAGE = '/api-portals/acme-portal/apis/rest-api/orders-api/publication/landing-page';
const PNG_BYTES = new Uint8Array([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

const servePng = (path: string) =>
  mswHttp.get(apiUrl(path), async ({ request }) => {
    await requests.capture(request);
    return new HttpResponse(PNG_BYTES, { headers: { 'Content-Type': 'image/png' } });
  });

describe('getApiPublicationDraftThumbnail', () => {
  it('GETs the draft thumbnail as a Blob labelled with its content type', async () => {
    server.use(servePng(DRAFT_THUMBNAIL));

    const result = await getApiPublicationDraftThumbnail('acme-portal', 'rest-api', 'orders-api');

    expect(result?.contentType).toBe('image/png');
    expect(result?.blob).toBeInstanceOf(Blob);
    expect(requests.last()?.url.pathname).toBe(`/api/v0.9${DRAFT_THUMBNAIL}`);
  });

  it('URL-encodes the path segments', async () => {
    server.use(servePng('/api-portals/:portal/apis/:type/:api/draft/thumbnail'));

    await getApiPublicationDraftThumbnail('a b', 'rest-api', 'c/d');

    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/api-portals/a%20b/apis/rest-api/c%2Fd/draft/thumbnail',
    );
  });

  it('surfaces a 404 as ApiError, since a draft without a thumbnail is not read', async () => {
    server.use(failure('get', DRAFT_THUMBNAIL, 404, 'NOT_FOUND'));

    await expect(
      getApiPublicationDraftThumbnail('acme-portal', 'rest-api', 'orders-api'),
    ).rejects.toMatchObject({ status: 404 });
  });
});

describe('getApiPublicationThumbnail', () => {
  it('GETs the published thumbnail as a Blob labelled with its content type', async () => {
    server.use(servePng(PUBLICATION_THUMBNAIL));

    const result = await getApiPublicationThumbnail('acme-portal', 'rest-api', 'orders-api');

    expect(result?.contentType).toBe('image/png');
    expect(result?.blob).toBeInstanceOf(Blob);
    expect(requests.last()?.url.pathname).toBe(`/api/v0.9${PUBLICATION_THUMBNAIL}`);
  });
});

describe('saveApiPublicationDraftThumbnail', () => {
  // jsdom cannot put real multipart on the wire, so the request is checked at the http client.
  it('PUTs the image as the multipart `file` of the draft thumbnail', async () => {
    const put = vi.spyOn(http, 'put').mockResolvedValue(undefined);
    try {
      await saveApiPublicationDraftThumbnail('acme-portal', 'rest-api', 'orders-api', new Blob([PNG_BYTES], { type: 'image/png' }));

      const [path, body, options] = put.mock.calls[0];
      expect(path).toContain(DRAFT_THUMBNAIL);
      expect(body).toBeInstanceOf(FormData);
      expect((body as FormData).get('file')).toBeInstanceOf(Blob);
      expect(options).toMatchObject({ operationName: 'saveApiPublicationDraftThumbnail' });
    } finally {
      put.mockRestore();
    }
  });
});

describe('landing page', () => {
  const serveMarkdown = (method: 'get' | 'put', path: string, body?: string) =>
    mswHttp[method](apiUrl(path), async ({ request }) => {
      await requests.capture(request);
      return body === undefined
        ? new HttpResponse(null, { status: 204 })
        : new HttpResponse(body, { headers: { 'Content-Type': 'text/markdown' } });
    });

  it('reads the draft and the published landing page as text', async () => {
    server.use(
      serveMarkdown('get', DRAFT_LANDING_PAGE, '# Draft'),
      serveMarkdown('get', PUBLICATION_LANDING_PAGE, '# Published'),
    );

    await expect(getApiPublicationDraftLandingPage('acme-portal', 'rest-api', 'orders-api')).resolves.toEqual({
      contentType: 'text/markdown',
      text: '# Draft',
    });
    await expect(getApiPublicationLandingPage('acme-portal', 'rest-api', 'orders-api')).resolves.toMatchObject({
      text: '# Published',
    });
  });

  it('PUTs the Markdown as it is, labelled text/markdown', async () => {
    server.use(serveMarkdown('put', DRAFT_LANDING_PAGE));

    await saveApiPublicationDraftLandingPage('acme-portal', 'rest-api', 'orders-api', '# Orders\n\n- **GET** /orders');

    const call = requests.last();
    expect(call?.url.pathname).toBe(`/api/v0.9${DRAFT_LANDING_PAGE}`);
    expect(call?.headers.get('content-type')).toContain('text/markdown');
    expect(call?.body).toBe('# Orders\n\n- **GET** /orders');
  });

  it('surfaces a 404 as ApiError, since a page the flag does not announce is not read', async () => {
    server.use(failure('get', DRAFT_LANDING_PAGE, 404, 'NOT_FOUND'));

    await expect(
      getApiPublicationDraftLandingPage('acme-portal', 'rest-api', 'orders-api'),
    ).rejects.toMatchObject({ status: 404 });
  });
});
