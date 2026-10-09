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

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { resetHttpClient } from '@/api/core/http';
import { accepts, failure, recorder, type Recorder } from '@/test/msw';
import { server } from '@/test/server';
import { fireEvent, renderWithProviders, screen, waitFor } from '@/test/utils';
import { ContractSourceForm, fetchContractForPreview } from './ContractSourceForm';

let validateRequests: Recorder;

beforeEach(() => {
  resetHttpClient();
  validateRequests = recorder();
  server.use(
    accepts(
      'post',
      '/rest-apis/validate-openapi',
      { isValid: true, errors: [], content: VALID_SPEC },
      { record: validateRequests },
    ),
  );
});

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
    // Removing is not an error, so the neutral guidance in the card comes back.
    expect(screen.getByText(/Accepted file types:/)).toBeInTheDocument();
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

  it('waits for typing to pause; leaving the field reads it at once', async () => {
    const onContractChange = vi.fn();
    const { user } = renderWithProviders(
      <ContractSourceForm onContractChange={onContractChange} />,
    );

    await user.type(screen.getByLabelText(/Spec URL/), 'https://example.com/openapi.yaml');
    // A URL passes through many invalid prefixes on the way in; none of them
    // is worth a request.
    expect(validateRequests.count()).toBe(0);
    expect(screen.queryByRole('button', { name: /Fetch/i })).not.toBeInTheDocument();

    await user.tab();

    await waitFor(() =>
      expect(onContractChange).toHaveBeenCalledWith(
        expect.objectContaining({ dialect: 'openapi-3.0' }),
      ),
    );
    expect(validateRequests.count()).toBe(1);
  });

  it('clears fetched state and re-fetches when the same URL is entered again', async () => {
    const onContractChange = vi.fn();
    const { user } = renderWithProviders(
      <ContractSourceForm onContractChange={onContractChange} />,
    );
    const field = screen.getByLabelText(/Spec URL/);

    await user.type(field, 'https://example.com/openapi.yaml');
    await user.tab();
    await waitFor(() =>
      expect(onContractChange).toHaveBeenCalledWith(
        expect.objectContaining({ dialect: 'openapi-3.0' }),
      ),
    );
    await user.click(screen.getByRole('button', { name: 'Clear URL' }));

    expect(field).toHaveValue('');
    await waitFor(() => expect(onContractChange).toHaveBeenLastCalledWith(null));

    await user.type(field, 'https://example.com/openapi.yaml');
    await user.tab();

    await waitFor(() => expect(validateRequests.count()).toBe(2));
    await waitFor(() =>
      expect(onContractChange).toHaveBeenLastCalledWith(
        expect.objectContaining({ dialect: 'openapi-3.0' }),
      ),
    );
  });

  it('does not read it again when the field is left untouched', async () => {
    const onContractChange = vi.fn();
    const { user } = renderWithProviders(
      <ContractSourceForm onContractChange={onContractChange} />,
    );

    const field = screen.getByLabelText(/Spec URL/);
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

    expect(validateRequests.count()).toBe(1);
  });

  it('reads a URL once typing pauses, without the field having to be left', async () => {
    const { user } = renderWithProviders(<ContractSourceForm onContractChange={() => {}} />);
    const field = screen.getByLabelText(/Spec URL/);

    await user.type(field, 'https://example.com/openapi.yaml');
    expect(validateRequests.count()).toBe(0);

    await waitFor(() => expect(validateRequests.count()).toBe(1), { timeout: 2000 });
    expect(field).toHaveFocus();
    expect(field).toHaveValue('https://example.com/openapi.yaml');
  });

  it('reads a sample once, not again when the typing pause comes round', async () => {
    const { user } = renderWithProviders(<ContractSourceForm onContractChange={() => {}} />);

    await user.click(screen.getByRole('button', { name: 'Try a sample' }));
    await waitFor(() => expect(validateRequests.count()).toBe(1));
    // Past the 700 ms typing pause, which used to ask again.
    await new Promise((resolve) => setTimeout(resolve, 1000));

    expect(validateRequests.count()).toBe(1);
  });

  it('reads a pasted URL straight away', async () => {
    const { user } = renderWithProviders(<ContractSourceForm onContractChange={() => {}} />);

    await user.click(screen.getByLabelText(/Spec URL/));
    await user.paste('https://example.com/openapi.yaml');

    // Well inside the typing pause: a paste is a finished value.
    await waitFor(() => expect(validateRequests.count()).toBe(1), { timeout: 400 });
  });

  it('says what it found in one quiet line once the spec is accepted', async () => {
    const { user } = renderWithProviders(<ContractSourceForm onContractChange={() => {}} />);

    await user.type(screen.getByLabelText(/Spec URL/), 'https://example.com/openapi.yaml');
    await user.tab();

    expect(await screen.findByText('OpenAPI 3.0.0 · 1 route found')).toHaveAttribute(
      'role',
      'status',
    );
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('offers a live API to the endpoint tab, instead of the parser’s complaints', async () => {
    server.use(
      accepts('post', '/rest-apis/validate-openapi', {
        errors: [{ message: 'unsupported openapi version' }],
        isValid: false,
      }),
    );
    const onUseAsEndpoint = vi.fn();
    const { user } = renderWithProviders(
      <ContractSourceForm onContractChange={() => {}} onUseAsEndpoint={onUseAsEndpoint} />,
    );

    await user.type(screen.getByLabelText(/Spec URL/), 'https://api.example.com/orders');
    await user.tab();
    await user.click(await screen.findByRole('button', { name: 'Use it as an endpoint instead' }));

    expect(onUseAsEndpoint).toHaveBeenCalledWith('https://api.example.com/orders');
    expect(screen.queryByText('Validation failed:')).not.toBeInTheDocument();
  });

  it('offers the endpoint tab when an address that isn’t a spec can’t be read at all', async () => {
    server.use(failure('post', '/rest-apis/validate-openapi', 400, 'VALIDATION_FAILED'));
    const onUseAsEndpoint = vi.fn();
    const { user } = renderWithProviders(
      <ContractSourceForm onContractChange={() => {}} onUseAsEndpoint={onUseAsEndpoint} />,
    );

    await user.type(screen.getByLabelText(/Spec URL/), 'https://api.example.com/orders');
    await user.tab();
    await user.click(await screen.findByRole('button', { name: 'Use it as an endpoint instead' }));

    expect(onUseAsEndpoint).toHaveBeenCalledWith('https://api.example.com/orders');
    expect(screen.getAllByRole('alert')).toHaveLength(1);
  });

  it('keeps the parser’s reasons for an address that is a spec, and shows one notice', async () => {
    server.use(
      accepts('post', '/rest-apis/validate-openapi', {
        errors: [{ message: 'paths must be an object' }],
        isValid: false,
      }),
    );
    const { user } = renderWithProviders(
      <ContractSourceForm onContractChange={() => {}} onUseAsEndpoint={vi.fn()} />,
    );

    await user.type(screen.getByLabelText(/Spec URL/), 'https://example.com/openapi.yaml');
    await user.tab();

    expect(await screen.findByText('Validation failed:')).toBeInTheDocument();
    expect(screen.getAllByRole('alert')).toHaveLength(1);
    expect(
      screen.queryByRole('button', { name: 'Use it as an endpoint instead' }),
    ).not.toBeInTheDocument();
  });

  it('starts from a handed-over URL and reads it straight away', async () => {
    renderWithProviders(
      <ContractSourceForm
        initialUrl="https://example.com/openapi.yaml"
        onContractChange={() => {}}
      />,
    );

    expect(screen.getByLabelText(/Spec URL/)).toHaveValue('https://example.com/openapi.yaml');
    await waitFor(() => expect(validateRequests.count()).toBe(1), { timeout: 400 });
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
