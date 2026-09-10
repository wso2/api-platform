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

import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import { accepts, recorder, type Recorder } from '@/test/msw';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor } from '@/test/utils';
import { CreateApiKeyDialog } from './CreateApiKeyDialog';

const ORG = 'api-platform-demo';
const API_ID = 'pizza-shack';
const KEYS = `/rest-apis/${API_ID}/api-keys`;

/** The value the server would mint. Only ever exists in the create response. */
const ISSUED = 'wso2_ak_0a85417f7775a55182ee48b5cf475a75';

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

/** The mutation resolves its organization from `ApiScopeContext`, so the scope
 * provider has to be mounted for the request to be allowed out at all. */
function setup() {
  const onClose = vi.fn();
  const utils = renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <CreateApiKeyDialog onClose={onClose} open restApiId={API_ID} />
    </ApiScopeProvider>,
  );
  return { ...utils, onClose };
}

describe('CreateApiKeyDialog', () => {
  it('disables Create key until the key is named', async () => {
    const { user } = setup();

    expect(screen.getByRole('button', { name: 'Create key' })).toBeDisabled();

    await user.type(screen.getByLabelText(/Key name/), 'Production key');

    expect(screen.getByRole('button', { name: 'Create key' })).toBeEnabled();
  });

  it('restates the chosen duration as a calendar date', async () => {
    const { user } = setup();

    // Prefilled, so the preview is there before anything is typed.
    expect(screen.getByText(/90 days from today/)).toBeInTheDocument();

    const duration = screen.getByLabelText('Expiry duration');
    await user.clear(duration);
    await user.type(duration, '1');

    expect(screen.getByText(/1 day from today/)).toBeInTheDocument();
  });

  it('rejects a duration that is not a positive whole number', async () => {
    const { user } = setup();

    const duration = screen.getByLabelText('Expiry duration');
    await user.clear(duration);
    await user.tab();

    expect(screen.getByText(/Enter a whole number between 1 and 3650/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Create key' })).toBeDisabled();
  });

  it('asks the server to generate the key, then shows it once', async () => {
    server.use(
      accepts(
        'post',
        KEYS,
        { status: 'success', message: 'created', keyId: 'key-1', apiKey: ISSUED },
        { record: requests },
      ),
    );
    const { user, onClose } = setup();

    await user.type(screen.getByLabelText(/Key name/), '  Production key  ');
    await user.click(screen.getByRole('button', { name: 'Create key' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    // No `apiKey` in the request: omitting it is what makes the server mint one.
    expect(JSON.parse(requests.last()!.body)).toEqual({
      displayName: 'Production key',
      expiresIn: { duration: 90, unit: 'days' },
    });

    // Second step: the plaintext, and no way back to the form.
    await screen.findByText('API key created');
    expect(screen.getByLabelText('API key')).toHaveValue(ISSUED);
    expect(onClose).not.toHaveBeenCalled();

    await user.click(screen.getByRole('button', { name: 'I have copied the key' }));
    expect(onClose).toHaveBeenCalled();
  });

  it('sends the duration in the unit the user picked', async () => {
    server.use(
      accepts(
        'post',
        KEYS,
        { status: 'success', message: 'created', apiKey: ISSUED },
        {
          record: requests,
        },
      ),
    );
    const { user } = setup();

    await user.type(screen.getByLabelText(/Key name/), 'Short lived');
    await user.click(screen.getByRole('combobox', { name: 'Expiry time unit' }));
    await user.click(screen.getByRole('option', { name: 'Hours' }));
    await user.click(screen.getByRole('button', { name: 'Create key' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(JSON.parse(requests.last()!.body).expiresIn).toEqual({ duration: 90, unit: 'hours' });
  });

  it('copies the issued key to the clipboard', async () => {
    // `userEvent.setup()` (inside `renderWithProviders`) installs the clipboard
    // stub, so this reads back through the same API the dialog wrote to.
    server.use(accepts('post', KEYS, { status: 'success', message: 'created', apiKey: ISSUED }));
    const { user } = setup();

    await user.type(screen.getByLabelText(/Key name/), 'Production key');
    await user.click(screen.getByRole('button', { name: 'Create key' }));
    await screen.findByText('API key created');

    await user.click(screen.getByRole('button', { name: 'Copy' }));

    await screen.findByRole('button', { name: 'Copied' });
    expect(await navigator.clipboard.readText()).toBe(ISSUED);
  });

  it('closes without a second step when the server returns no key', async () => {
    // `apiKey` comes back only for a server-generated key. With nothing to
    // reveal, holding the user on a step with nothing to copy would be a dead end.
    server.use(accepts('post', KEYS, { status: 'success', message: 'created' }));
    const { user, onClose } = setup();

    await user.type(screen.getByLabelText(/Key name/), 'Injected key');
    await user.click(screen.getByRole('button', { name: 'Create key' }));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(screen.queryByText('API key created')).not.toBeInTheDocument();
  });
});
