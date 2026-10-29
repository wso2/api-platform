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

import { mcpProxiesApis } from '../../../../apis/MCP/mcpProxiesApis';
import { renderWithProviders, screen, waitFor } from '../../../../test/utils';
import { DocumentPreview } from './DocumentPreview';

vi.mock('../../../../apis/MCP/mcpProxiesApis', () => ({ mcpProxiesApis: { getMcpProxyDocumentContent: vi.fn() } }));
vi.mock('../../../../Components/AILoader', () => ({ default: () => <div data-testid="loader" /> }));
vi.mock('../../../../utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), info: vi.fn(), debug: vi.fn() },
}));

const read = vi.mocked(mcpProxiesApis.getMcpProxyDocumentContent);

beforeEach(() => {
  read.mockReset();
});

describe('DocumentPreview', () => {
  it('reads the document of this proxy, with a loader until it arrives', async () => {
    read.mockResolvedValue({ contentType: 'text/markdown; charset=utf-8', text: '# Setup guide' });

    renderWithProviders(<DocumentPreview docId="setup" mcpProxyId="travel" />);
    expect(screen.getByTestId('loader')).toBeInTheDocument();

    expect(await screen.findByRole('heading', { name: 'Setup guide' })).toBeInTheDocument();
    expect(read).toHaveBeenCalledWith('travel', 'setup', expect.any(String));
  });

  it('shows Markdown as it reads, and raw HTML as text instead of running it', async () => {
    read.mockResolvedValue({ contentType: 'text/markdown', text: '<img src=x onerror=alert(1)> and [x](javascript:alert(1))' });

    const { container } = renderWithProviders(<DocumentPreview docId="setup" mcpProxyId="travel" />);

    await screen.findByText(/onerror=alert/);
    expect(container.querySelector('img')).toBeNull();
    expect(container.querySelector('a')).toBeNull();
  });

  it('says so when the format cannot be shown, instead of rendering bytes as Markdown', async () => {
    read.mockResolvedValue({ contentType: 'application/pdf', text: '%PDF-1.7' });

    renderWithProviders(<DocumentPreview docId="spec" mcpProxyId="travel" />);

    expect(await screen.findByText('This document’s format can’t be previewed here.')).toBeInTheDocument();
    expect(screen.queryByText('%PDF-1.7')).not.toBeInTheDocument();
  });

  it('says a document with nothing stored has no content', async () => {
    read.mockResolvedValue({ contentType: '', text: '' });

    renderWithProviders(<DocumentPreview docId="empty" mcpProxyId="travel" />);

    expect(await screen.findByText('This document has no content.')).toBeInTheDocument();
  });

  it('says so when the content cannot be read', async () => {
    read.mockRejectedValue(new Error('forbidden'));

    renderWithProviders(<DocumentPreview docId="setup" mcpProxyId="travel" />);

    expect(await screen.findByText('Unable to load the content of this document.')).toBeInTheDocument();
  });

  it('shows the loader again, not the last document, while another one is read', async () => {
    read.mockResolvedValueOnce({ contentType: 'text/markdown', text: '# First' });
    const { rerender } = renderWithProviders(<DocumentPreview docId="a" mcpProxyId="travel" />);
    await screen.findByRole('heading', { name: 'First' });

    read.mockReturnValueOnce(new Promise(() => undefined));
    rerender(<DocumentPreview docId="b" mcpProxyId="travel" />);

    await waitFor(() => expect(screen.getByTestId('loader')).toBeInTheDocument());
    expect(screen.queryByRole('heading', { name: 'First' })).not.toBeInTheDocument();
  });

  it('ignores a late answer for a document that is no longer shown', async () => {
    let answerFirst: (value: { contentType: string; text: string }) => void = () => undefined;
    read.mockReturnValueOnce(new Promise((resolve) => { answerFirst = resolve; }));
    const { rerender } = renderWithProviders(<DocumentPreview docId="a" mcpProxyId="travel" />);

    read.mockResolvedValueOnce({ contentType: 'text/markdown', text: '# Second' });
    rerender(<DocumentPreview docId="b" mcpProxyId="travel" />);
    await screen.findByRole('heading', { name: 'Second' });
    answerFirst({ contentType: 'text/markdown', text: '# Stale' });

    await waitFor(() => expect(screen.queryByRole('heading', { name: 'Stale' })).not.toBeInTheDocument());
    expect(screen.getByRole('heading', { name: 'Second' })).toBeInTheDocument();
  });
});
