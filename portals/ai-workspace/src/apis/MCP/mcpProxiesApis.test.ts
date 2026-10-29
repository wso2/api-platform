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

import { beforeEach, describe, expect, it, vi } from 'vitest';

import { get, getBlob, getText, post, put, putFormData, putText } from '../../clients/choreoApiClient';
import {
  deprecateMcpProxyOnApiPortal,
  getMcpProxyApiPortalDraftDefinition,
  getMcpProxyApiPortalDraftLandingPage,
  getMcpProxyApiPortalDraftThumbnail,
  getMcpProxyApiPortalPublicationDefinition,
  getMcpProxyApiPortalPublicationLandingPage,
  getMcpProxyApiPortalPublicationThumbnail,
  getMcpProxyDocumentContent,
  listMcpProxyApiPortalPublications,
  listMcpProxyDocuments,
  mcpProxiesApis,
  publishSavedMcpProxyDraft,
  saveMcpProxyApiPortalDraft,
  saveMcpProxyApiPortalDraftDefinition,
  saveMcpProxyApiPortalDraftLandingPage,
  saveMcpProxyApiPortalDraftThumbnail,
} from './mcpProxiesApis';

vi.mock('../../clients/choreoApiClient', () => ({
  get: vi.fn(),
  getBlob: vi.fn(),
  getText: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  putFormData: vi.fn(),
  putText: vi.fn(),
  del: vi.fn(),
}));

vi.mock('../../utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), info: vi.fn(), debug: vi.fn() },
}));

const BASE = 'https://control-plane.test';
const HUB = 'hub/one';
const PROXY = 'travel mcp';
const PORTAL_PATH = '/api-portals/hub%2Fone/apis/mcp-proxy/travel%20mcp';

beforeEach(() => {
  vi.mocked(get).mockReset();
  vi.mocked(post).mockReset();
  vi.mocked(put).mockReset();
  vi.mocked(getText).mockReset();
  vi.mocked(getBlob).mockReset();
  vi.mocked(putFormData).mockReset();
  vi.mocked(putText).mockReset();
});

describe('listMcpProxyApiPortalPublications', () => {
  it('asks for this MCP proxy across every portal, as one page of up to 100', async () => {
    const summary = { list: [], pagination: { total: 0, offset: 0, limit: 100 } };
    vi.mocked(get).mockResolvedValue(summary);

    const result = await listMcpProxyApiPortalPublications(PROXY, BASE);

    expect(get).toHaveBeenCalledWith(
      '/api-publications',
      { apiType: 'mcp-proxy', apiId: PROXY, limit: 100 },
      BASE
    );
    expect(result).toBe(summary);
  });

  it('propagates a failed read rather than reporting no portals', async () => {
    vi.mocked(get).mockRejectedValue(new Error('forbidden'));

    await expect(listMcpProxyApiPortalPublications(PROXY, BASE)).rejects.toThrow('forbidden');
  });
});

describe('saveMcpProxyApiPortalDraft', () => {
  it('replaces the draft with the given details and returns what was stored', async () => {
    const draft = { displayName: 'Travel', version: '1.0', subscriptionPlanIds: [], docIds: [] };
    const stored = { ...draft, hasThumbnail: false };
    vi.mocked(put).mockResolvedValue(stored);

    const result = await saveMcpProxyApiPortalDraft(HUB, PROXY, draft, BASE);

    expect(put).toHaveBeenCalledWith(`${PORTAL_PATH}/draft`, draft, BASE);
    expect(result).toBe(stored);
  });
});

describe('the draft definition', () => {
  it('is saved to the draft, as the object it is', async () => {
    vi.mocked(put).mockResolvedValue(undefined);
    const definition = { tools: [{ name: 'search' }], resources: [], prompts: [] };

    await saveMcpProxyApiPortalDraftDefinition(HUB, PROXY, definition, BASE);

    expect(put).toHaveBeenCalledWith(`${PORTAL_PATH}/draft/definition`, definition, BASE);
  });

  it('is read from the draft', async () => {
    const definition = { tools: [], resources: [], prompts: [] };
    vi.mocked(get).mockResolvedValue(definition);

    expect(await getMcpProxyApiPortalDraftDefinition(HUB, PROXY, BASE)).toBe(definition);
    expect(get).toHaveBeenCalledWith(`${PORTAL_PATH}/draft/definition`, undefined, BASE);
  });

  it('is read from the live listing', async () => {
    const definition = { tools: [{ name: 'live' }] };
    vi.mocked(get).mockResolvedValue(definition);

    expect(await getMcpProxyApiPortalPublicationDefinition(HUB, PROXY, BASE)).toBe(definition);
    expect(get).toHaveBeenCalledWith(`${PORTAL_PATH}/publication/definition`, undefined, BASE);
  });

  it('lets a missing definition surface as the error, for the caller to read as "none saved"', async () => {
    const notFound = Object.assign(new Error('not found'), { status: 404 });
    vi.mocked(get).mockRejectedValue(notFound);

    await expect(getMcpProxyApiPortalDraftDefinition(HUB, PROXY, BASE)).rejects.toBe(notFound);
  });
});

describe('publishSavedMcpProxyDraft', () => {
  it('publishes with no body, since the server publishes what the draft holds', async () => {
    const publication = { apiPortalId: HUB, status: 'PUBLISHED' };
    vi.mocked(post).mockResolvedValue(publication);

    const result = await publishSavedMcpProxyDraft(HUB, PROXY, BASE);

    expect(post).toHaveBeenCalledWith(`${PORTAL_PATH}/publish`, undefined, BASE);
    expect(result).toBe(publication);
  });
});

describe('deprecateMcpProxyOnApiPortal', () => {
  it('deprecates with no body and returns the updated listing', async () => {
    const publication = { apiPortalId: HUB, status: 'DEPRECATED' };
    vi.mocked(post).mockResolvedValue(publication);

    const result = await deprecateMcpProxyOnApiPortal(HUB, PROXY, BASE);

    expect(post).toHaveBeenCalledWith(`${PORTAL_PATH}/deprecate`, undefined, BASE);
    expect(result).toBe(publication);
  });
});

describe('listMcpProxyDocuments', () => {
  const doc = (id: string) => ({ id, type: 'HowTo', displayName: id });
  const page = (ids: string[], total: number, offset = 0) => ({
    list: ids.map(doc),
    pagination: { total, offset, limit: 100 },
  });

  it('reads this proxy’s documents from the mcp-proxy collection, escaping its id', async () => {
    vi.mocked(get).mockResolvedValue(page(['setup', 'faq'], 2));

    const documents = await listMcpProxyDocuments(PROXY, BASE);

    expect(documents.map(({ id }) => id)).toEqual(['setup', 'faq']);
    expect(get).toHaveBeenCalledTimes(1);
    expect(get).toHaveBeenCalledWith('/apis/mcp-proxy/travel%20mcp/docs', { limit: 100, offset: 0 }, BASE);
  });

  it('keeps reading, from where the last page ended, until the total is held', async () => {
    vi.mocked(get).mockResolvedValueOnce(page(['a', 'b'], 3)).mockResolvedValueOnce(page(['c'], 3, 2));

    const documents = await listMcpProxyDocuments(PROXY, BASE);

    expect(documents.map(({ id }) => id)).toEqual(['a', 'b', 'c']);
    expect(get).toHaveBeenNthCalledWith(2, '/apis/mcp-proxy/travel%20mcp/docs', { limit: 100, offset: 2 }, BASE);
  });

  it('lists a document once when one created mid-read pushes it onto the next page too', async () => {
    vi.mocked(get).mockResolvedValueOnce(page(['a', 'b'], 4)).mockResolvedValueOnce(page(['b', 'c'], 4, 2));

    const documents = await listMcpProxyDocuments(PROXY, BASE);

    expect(documents.map(({ id }) => id)).toEqual(['a', 'b', 'c']);
    expect(get).toHaveBeenCalledTimes(2);
  });

  it('stops on an empty page even when the total says more exist', async () => {
    vi.mocked(get).mockResolvedValueOnce(page(['a'], 5)).mockResolvedValueOnce(page([], 5, 1));

    expect(await listMcpProxyDocuments(PROXY, BASE)).toHaveLength(1);
    expect(get).toHaveBeenCalledTimes(2);
  });
});

describe('the landing page', () => {
  it('is read from the draft as Markdown', async () => {
    vi.mocked(getText).mockResolvedValue({ text: '# Draft', contentType: 'text/markdown' });

    expect(await getMcpProxyApiPortalDraftLandingPage(HUB, PROXY, BASE)).toBe('# Draft');
    expect(getText).toHaveBeenCalledWith(`${PORTAL_PATH}/draft/landing-page`, BASE);
  });

  it('is read from the live listing as Markdown', async () => {
    vi.mocked(getText).mockResolvedValue({ text: '# Live', contentType: 'text/markdown' });

    expect(await getMcpProxyApiPortalPublicationLandingPage(HUB, PROXY, BASE)).toBe('# Live');
    expect(getText).toHaveBeenCalledWith(`${PORTAL_PATH}/publication/landing-page`, BASE);
  });

  it('is saved to the draft as Markdown', async () => {
    vi.mocked(putText).mockResolvedValue(undefined);

    await saveMcpProxyApiPortalDraftLandingPage(HUB, PROXY, '# Mine', BASE);

    expect(putText).toHaveBeenCalledWith(`${PORTAL_PATH}/draft/landing-page`, '# Mine', 'text/markdown', BASE);
  });

  it('lets a missing landing page surface as the error, for the caller to read as "none saved"', async () => {
    const notFound = Object.assign(new Error('not found'), { status: 404 });
    vi.mocked(getText).mockRejectedValue(notFound);

    await expect(getMcpProxyApiPortalDraftLandingPage(HUB, PROXY, BASE)).rejects.toBe(notFound);
  });
});

describe('the thumbnail', () => {
  const image = new Blob(['png'], { type: 'image/png' });

  it('is read from the draft', async () => {
    vi.mocked(getBlob).mockResolvedValue(image);

    expect(await getMcpProxyApiPortalDraftThumbnail(HUB, PROXY, BASE)).toBe(image);
    expect(getBlob).toHaveBeenCalledWith(`${PORTAL_PATH}/draft/thumbnail`, BASE);
  });

  it('is read from the live listing', async () => {
    vi.mocked(getBlob).mockResolvedValue(image);

    expect(await getMcpProxyApiPortalPublicationThumbnail(HUB, PROXY, BASE)).toBe(image);
    expect(getBlob).toHaveBeenCalledWith(`${PORTAL_PATH}/publication/thumbnail`, BASE);
  });

  it('is saved to the draft as the multipart file the server expects', async () => {
    vi.mocked(putFormData).mockResolvedValue(undefined);

    await saveMcpProxyApiPortalDraftThumbnail(HUB, PROXY, image, BASE);

    const [path, form, base] = vi.mocked(putFormData).mock.calls[0];
    expect(path).toBe(`${PORTAL_PATH}/draft/thumbnail`);
    expect(base).toBe(BASE);
    expect((form.get('file') as File).size).toBe(image.size);
    expect([...form.keys()]).toEqual(['file']);
  });

  it('lets a missing thumbnail surface as the error, for the caller to read as "none saved"', async () => {
    const notFound = Object.assign(new Error('not found'), { status: 404 });
    vi.mocked(getBlob).mockRejectedValue(notFound);

    await expect(getMcpProxyApiPortalDraftThumbnail(HUB, PROXY, BASE)).rejects.toBe(notFound);
  });
});

describe('getMcpProxyDocumentContent', () => {
  it('reads one document’s body with the content type it was stored under, escaping both ids', async () => {
    const body = { text: '# Setup', contentType: 'text/markdown; charset=utf-8' };
    vi.mocked(getText).mockResolvedValue(body);

    expect(await getMcpProxyDocumentContent(PROXY, 'setup guide', BASE)).toBe(body);
    expect(getText).toHaveBeenCalledWith('/apis/mcp-proxy/travel%20mcp/docs/setup%20guide/content', BASE);
  });
});

describe('mcpProxiesApis', () => {
  it('exposes the existing publishing calls alongside the new ones', () => {
    expect(mcpProxiesApis).toMatchObject({
      getMcpProxyApiPortalPublication: expect.any(Function),
      getMcpProxyApiPortalDraft: expect.any(Function),
      publishMcpProxyToApiPortal: expect.any(Function),
      unpublishMcpProxyFromApiPortal: expect.any(Function),
      listMcpProxyApiPortalPublications: expect.any(Function),
      saveMcpProxyApiPortalDraft: expect.any(Function),
      saveMcpProxyApiPortalDraftDefinition: expect.any(Function),
      getMcpProxyApiPortalDraftDefinition: expect.any(Function),
      getMcpProxyApiPortalPublicationDefinition: expect.any(Function),
      publishSavedMcpProxyDraft: expect.any(Function),
      deprecateMcpProxyOnApiPortal: expect.any(Function),
      listMcpProxyDocuments: expect.any(Function),
      getMcpProxyDocumentContent: expect.any(Function),
      getMcpProxyApiPortalDraftLandingPage: expect.any(Function),
      saveMcpProxyApiPortalDraftLandingPage: expect.any(Function),
      getMcpProxyApiPortalPublicationLandingPage: expect.any(Function),
      getMcpProxyApiPortalDraftThumbnail: expect.any(Function),
      saveMcpProxyApiPortalDraftThumbnail: expect.any(Function),
      getMcpProxyApiPortalPublicationThumbnail: expect.any(Function),
    });
  });
});
