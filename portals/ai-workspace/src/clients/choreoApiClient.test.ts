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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { handleUnauthorizedResponse } from '../auth/logout';
import { CSRF_HEADER, CSRF_VALUE } from '../config.env';
import { getBlob, getText, putFormData, putText } from './choreoApiClient';

vi.mock('../auth/logout', () => ({ handleUnauthorizedResponse: vi.fn() }));
vi.mock('../utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), info: vi.fn(), debug: vi.fn() },
}));

const BASE = 'https://control-plane.test';
const fetchMock = vi.fn();

beforeEach(() => {
  fetchMock.mockReset();
  vi.mocked(handleUnauthorizedResponse).mockReset();
  vi.stubGlobal('fetch', fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

const failure = (status: number, body: unknown) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });

describe('getText', () => {
  it('reads the body as text with the content type it was stored under', async () => {
    fetchMock.mockResolvedValue(
      new Response('# Setup', { status: 200, headers: { 'Content-Type': 'text/markdown; charset=utf-8' } })
    );

    const result = await getText('/apis/mcp-proxy/travel/docs/setup/content', BASE);

    expect(result).toEqual({ text: '# Setup', contentType: 'text/markdown; charset=utf-8' });
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe(`${BASE}/apis/mcp-proxy/travel/docs/setup/content`);
    expect(init).toMatchObject({ method: 'GET', credentials: 'include' });
  });

  it('sends the CSRF header but does not ask for JSON, since the body is not JSON', async () => {
    fetchMock.mockResolvedValue(new Response('text', { status: 200 }));

    await getText('/x', BASE);

    expect(fetchMock.mock.calls[0][1].headers[CSRF_HEADER]).toBe(CSRF_VALUE);
    expect(fetchMock.mock.calls[0][1].headers).not.toHaveProperty('Accept');
    expect(fetchMock.mock.calls[0][1].headers).not.toHaveProperty('Content-Type');
  });

  it('reads an empty (204) response as empty text', async () => {
    fetchMock.mockResolvedValue(new Response(null, { status: 204 }));

    expect(await getText('/x', BASE)).toEqual({ text: '', contentType: '' });
  });

  it('throws the API error a failed response describes, and hands the response to the session guard', async () => {
    fetchMock.mockResolvedValue(failure(404, { code: 'NOT_FOUND', message: 'no such document' }));

    await expect(getText('/x', BASE)).rejects.toMatchObject({ status: 404, code: 'NOT_FOUND', message: 'no such document' });
    expect(handleUnauthorizedResponse).toHaveBeenCalledTimes(1);
  });

  it('throws an error that carries the status when the failure has no JSON body', async () => {
    fetchMock.mockResolvedValue(new Response('<html>bad gateway</html>', { status: 502 }));

    await expect(getText('/x', BASE)).rejects.toMatchObject({ status: 502 });
  });
});

describe('putText', () => {
  it('sends the text as the content type given, and resolves on an empty (204) answer', async () => {
    fetchMock.mockResolvedValue(new Response(null, { status: 204 }));

    await expect(putText('/draft/landing-page', '# Hello', 'text/markdown', BASE)).resolves.toBeUndefined();

    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe(`${BASE}/draft/landing-page`);
    expect(init).toMatchObject({ method: 'PUT', credentials: 'include', body: '# Hello' });
    expect(init.headers['Content-Type']).toBe('text/markdown');
  });

  it('carries the CSRF header, which the BFF requires on every mutation', async () => {
    fetchMock.mockResolvedValue(new Response(null, { status: 204 }));

    await putText('/x', 'text', 'text/markdown', BASE);

    expect(fetchMock.mock.calls[0][1].headers[CSRF_HEADER]).toBe(CSRF_VALUE);
  });

  it('throws the API error a refusal describes', async () => {
    fetchMock.mockResolvedValue(failure(404, { code: 'DRAFT_NOT_FOUND', message: 'no draft' }));

    await expect(putText('/x', 'text', 'text/markdown', BASE)).rejects.toMatchObject({
      status: 404,
      code: 'DRAFT_NOT_FOUND',
    });
  });
});

describe('getBlob', () => {
  it('reads the body as bytes that keep the content type they were stored under', async () => {
    fetchMock.mockResolvedValue(new Response(new Uint8Array([137, 80, 78, 71]), { status: 200, headers: { 'Content-Type': 'image/png' } }));

    const blob = await getBlob('/draft/thumbnail', BASE);

    expect(blob.type).toBe('image/png');
    expect(blob.size).toBe(4);
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe(`${BASE}/draft/thumbnail`);
    expect(init).toMatchObject({ method: 'GET', credentials: 'include' });
    expect(init.headers[CSRF_HEADER]).toBe(CSRF_VALUE);
  });

  it('throws the API error a failed response describes, so a missing thumbnail can be told from a bad one', async () => {
    fetchMock.mockResolvedValue(failure(404, { code: 'NOT_FOUND', message: 'no thumbnail' }));

    await expect(getBlob('/x', BASE)).rejects.toMatchObject({ status: 404, code: 'NOT_FOUND' });
    expect(handleUnauthorizedResponse).toHaveBeenCalledTimes(1);
  });
});

describe('putFormData', () => {
  it('sends the form as it is, leaving the multipart boundary to the browser, and resolves on an empty (204) answer', async () => {
    fetchMock.mockResolvedValue(new Response(null, { status: 204 }));
    const form = new FormData();
    form.append('file', new Blob(['png'], { type: 'image/png' }), 'logo.png');

    await expect(putFormData('/draft/thumbnail', form, BASE)).resolves.toBeUndefined();

    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe(`${BASE}/draft/thumbnail`);
    expect(init).toMatchObject({ method: 'PUT', credentials: 'include', body: form });
    expect(init.headers).not.toHaveProperty('Content-Type');
    expect(init.headers[CSRF_HEADER]).toBe(CSRF_VALUE);
  });

  it('throws the API error a refusal describes', async () => {
    fetchMock.mockResolvedValue(failure(400, { code: 'PUBLICATION_VALIDATION_FAILED', message: 'unsupported image' }));

    await expect(putFormData('/x', new FormData(), BASE)).rejects.toMatchObject({
      status: 400,
      code: 'PUBLICATION_VALIDATION_FAILED',
    });
  });
});
