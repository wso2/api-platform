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

import { afterEach, describe, expect, it, vi } from 'vitest';

import { fireEvent, renderWithProviders, screen, waitFor } from '@/test/utils';
import { ContractSourceForm, fetchContractForPreview } from './ContractSourceForm';

const yamlFile = (name: string) =>
  new File(['openapi: 3.0.0'], name, { type: 'application/x-yaml' });

/** The smallest document the step accepts: a dialect, a title, an operation. */
const VALID_SPEC = [
  'openapi: 3.0.0',
  'info:',
  '  title: Orders',
  "  version: '1.0'",
  'servers:',
  '  - url: https://example.com',
  'paths:',
  '  /orders:',
  '    get:',
  '      responses:',
  "        '200':",
  '          description: ok',
].join('\n');

/** Serves `VALID_SPEC` to every request, and counts them. */
const stubSpecHost = () => {
  const fetchMock = vi.fn(() =>
    Promise.resolve({
      headers: new Headers(),
      ok: true,
      text: () => Promise.resolve(VALID_SPEC),
    }),
  );
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
};

/** The hidden `<input type="file">` inside the drop zone. */
const filePicker = (): HTMLInputElement => {
  const input = document.querySelector('input[type="file"]');
  if (input === null) throw new Error('file input not rendered');
  return input as HTMLInputElement;
};

describe('ContractSourceForm — file upload', () => {
  it('says a file was unsupported instead of silently dropping the valid one', async () => {
    const { user } = renderWithProviders(<ContractSourceForm onContractChange={() => {}} />);

    await user.click(screen.getByRole('button', { name: 'Upload' }));

    await user.upload(filePicker(), yamlFile('openapi.yaml'));
    expect(await screen.findByText('openapi.yaml')).toBeInTheDocument();

    // `user.upload` honours the input's `accept` attribute and would refuse to
    // hand the file over at all, so the change is fired directly — which is
    // what a real drop or an OS picker that ignores the filter does.
    fireEvent.change(filePicker(), {
      target: { files: [new File(['# notes'], 'README.md', { type: 'text/markdown' })] },
    });

    // The previous selection is gone either way — what matters is that the
    // user is told why, rather than the helper text reverting to neutral.
    expect(
      await screen.findByText(/That file type is not supported\. Accepted types:/),
    ).toBeInTheDocument();
  });

  it('clears the error, not the reason, when the user removes their own file', async () => {
    const { user } = renderWithProviders(<ContractSourceForm onContractChange={() => {}} />);

    await user.click(screen.getByRole('button', { name: 'Upload' }));
    await user.upload(filePicker(), yamlFile('openapi.yaml'));
    expect(await screen.findByText('openapi.yaml')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /Remove openapi\.yaml/ }));

    await waitFor(() => expect(screen.queryByText('openapi.yaml')).not.toBeInTheDocument());
    // Removing is not an error, so the neutral line comes back.
    expect(screen.getByText(/Accepted: /)).toBeInTheDocument();
    expect(screen.queryByText(/is not supported/)).not.toBeInTheDocument();
  });
});

describe('ContractSourceForm — offered sources', () => {
  it('offers URL and Upload only, while the other flows are undecided', () => {
    renderWithProviders(<ContractSourceForm onContractChange={() => {}} />);

    expect(screen.getByRole('button', { name: 'URL' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Upload' })).toBeInTheDocument();
    // Both imports are held back until their flow is settled; everything
    // behind them is still in the module.
    expect(screen.queryByRole('button', { name: 'GitHub' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'SwaggerHub' })).not.toBeInTheDocument();
  });
});

describe('ContractSourceForm — automatic fetch', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('reads the URL when the field is left, not while it is being typed', async () => {
    const fetchMock = stubSpecHost();
    const onContractChange = vi.fn();
    const { user } = renderWithProviders(
      <ContractSourceForm onContractChange={onContractChange} />,
    );

    await user.type(
      screen.getByLabelText(/URL for API Contract/),
      'https://example.com/openapi.yaml',
    );
    // A URL passes through many invalid prefixes on the way in; none of them
    // is worth a request.
    expect(fetchMock).not.toHaveBeenCalled();
    expect(screen.queryByRole('button', { name: /Fetch/i })).not.toBeInTheDocument();

    await user.tab();

    await waitFor(() =>
      expect(onContractChange).toHaveBeenCalledWith(
        expect.objectContaining({ dialect: 'openapi-3.0' }),
      ),
    );
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it('does not read it again when the field is left untouched', async () => {
    const fetchMock = stubSpecHost();
    const onContractChange = vi.fn();
    const { user } = renderWithProviders(
      <ContractSourceForm onContractChange={onContractChange} />,
    );

    const field = screen.getByLabelText(/URL for API Contract/);
    await user.type(field, 'https://example.com/openapi.yaml');
    await user.tab();
    await waitFor(() =>
      expect(onContractChange).toHaveBeenCalledWith(
        expect.objectContaining({ dialect: 'openapi-3.0' }),
      ),
    );

    // Focusing and leaving again would re-download the same document — and
    // discard whatever has been edited in the preview since.
    await user.click(field);
    await user.tab();

    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it('reads an uploaded file as soon as it is chosen', async () => {
    const onContractChange = vi.fn();
    const { user } = renderWithProviders(
      <ContractSourceForm onContractChange={onContractChange} />,
    );

    await user.click(screen.getByRole('button', { name: 'Upload' }));
    await user.upload(
      filePicker(),
      new File([VALID_SPEC], 'openapi.yaml', { type: 'application/x-yaml' }),
    );

    await waitFor(() =>
      expect(onContractChange).toHaveBeenCalledWith(
        expect.objectContaining({ dialect: 'openapi-3.0' }),
      ),
    );
  });
});

describe('fetchContractForPreview — URL source', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('rejects an oversized document on its declared length, before reading the body', async () => {
    const text = vi.fn(() => Promise.resolve('openapi: 3.0.0'));
    const fetchMock = vi.fn(() =>
      Promise.resolve({
        headers: new Headers({ 'content-length': String(64 * 1024 * 1024) }),
        ok: true,
        text,
      }),
    );
    vi.stubGlobal('fetch', fetchMock);

    await expect(
      fetchContractForPreview({
        apiTypeKey: 'rest',
        sourceKey: 'url',
        url: 'https://example.com/huge.yaml',
      }),
    ).resolves.toEqual({ status: 'oversized' });

    // The whole point: the body is never materialised in the tab.
    expect(text).not.toHaveBeenCalled();
  });

  it('gives the request a deadline so a stalled host cannot hang the Fetch button', async () => {
    let init: RequestInit | undefined;
    vi.stubGlobal('fetch', (_url: string, requestInit?: RequestInit) => {
      init = requestInit;
      return Promise.resolve({
        headers: new Headers(),
        ok: true,
        text: () =>
          Promise.resolve('openapi: 3.0.0\ninfo:\n  title: X\n  version: "1"\npaths: {}\n'),
      });
    });

    await fetchContractForPreview({
      apiTypeKey: 'rest',
      sourceKey: 'url',
      url: 'https://example.com/openapi.yaml',
    });

    expect(init?.signal).toBeInstanceOf(AbortSignal);
  });
});
