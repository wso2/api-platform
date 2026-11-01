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

import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { renderWithProviders, screen, waitFor } from '@/test/utils';
import PolicyParameterEditor from '../PolicyParameterEditor';
import { fixture, type FixtureName } from './fixtures';

const byTestId = (id: string) => screen.queryByTestId(id);
const submit = () => screen.getByTestId('policy-param-submit');

function renderEditor(
  name: FixtureName,
  props: Partial<React.ComponentProps<typeof PolicyParameterEditor>> = {},
) {
  const onSubmit = vi.fn();
  const onCancel = vi.fn();
  const utils = renderWithProviders(
    <PolicyParameterEditor
      policyDefinition={fixture(name)}
      onSubmit={onSubmit}
      onCancel={onCancel}
      {...props}
    />,
  );
  return { ...utils, onSubmit, onCancel };
}

describe('PolicyParameterEditor', () => {
  it('CORS can be added straight away with its defaults (Cypress TC-96)', async () => {
    const { user, onSubmit } = renderEditor('cors');
    expect(submit()).not.toBeDisabled();
    expect(submit()).toHaveTextContent('Add');
    await user.click(submit());
    expect(onSubmit).toHaveBeenCalledTimes(1);
    expect(onSubmit.mock.calls[0][0]).toMatchObject({ allowedOrigins: ['*'] });
  });

  it('oauth2-generator: shows one credential mode at a time', async () => {
    const { user, onSubmit } = renderEditor('oauth2-generator');

    // token endpoint mode first; bearer token hidden; required fields keep Add disabled
    expect(byTestId('field-tokenEndpoint')).toBeInTheDocument();
    expect(byTestId('field-bearerToken')).not.toBeInTheDocument();
    expect(submit()).toBeDisabled();

    // labels come from the key, not the raw key
    expect(screen.getByText('Client ID')).toBeInTheDocument();

    await user.type(byTestId('field-tokenEndpoint')!, 'https://idp.example.com/token');
    await user.type(byTestId('field-clientId')!, 'orders-gateway');
    await user.type(byTestId('field-clientSecret')!, 's3cret');
    expect(byTestId('field-clientSecret')).toHaveAttribute('type', 'password');
    expect(submit()).not.toBeDisabled();

    // switch to the static bearer token
    await user.click(screen.getByRole('radio', { name: 'Bearer token' }));
    expect(byTestId('field-tokenEndpoint')).not.toBeInTheDocument();
    expect(byTestId('field-bearerToken')).toBeInTheDocument();
    expect(submit()).toBeDisabled();

    await user.type(byTestId('field-bearerToken')!, 'abc');
    await user.click(submit());
    const sent = onSubmit.mock.calls[0][0];
    expect(sent.bearerToken).toBe('abc');
    expect(sent).not.toHaveProperty('tokenEndpoint');
    expect(sent).not.toHaveProperty('clientSecret');
  });

  it('oauth2-generator: the password grant adds username and password', async () => {
    const { user } = renderEditor('oauth2-generator');
    expect(byTestId('field-username')).not.toBeInTheDocument();
    await user.click(screen.getByRole('combobox', { name: 'Grant type' }));
    await user.click(await screen.findByRole('option', { name: 'password' }));
    // required now, so shown in the main section rather than under Advanced Settings
    await waitFor(() => expect(byTestId('field-username')).toBeVisible());
    expect(byTestId('field-password')).toBeInTheDocument();
  });

  it('oauth2-generator: integer lists are shown (tokenPurgeStatusCodes)', async () => {
    const { user } = renderEditor('oauth2-generator');
    await user.click(screen.getByRole('button', { name: 'Advanced Settings' }));
    expect(byTestId('field-tokenPurgeStatusCodes')).toBeInTheDocument();
    expect(screen.getByText('401')).toBeInTheDocument();
  });

  it('adds, edits, and removes map entries', async () => {
    const { user } = renderEditor('oauth2-generator');
    await user.click(screen.getByTestId('field-tokenRequestParams-add'));
    const key = screen.getByRole('textbox', { name: 'Key' });
    await user.type(key, 'scope');
    expect(key).toHaveValue('scope');
    await user.click(screen.getByRole('button', { name: 'Delete scope' }));
    expect(screen.queryByRole('textbox', { name: 'Key' })).not.toBeInTheDocument();
  });

  it('updates status as required fields are completed and toggles inline errors', async () => {
    const { user } = renderEditor('oauth2-generator');
    expect(screen.getByTestId('policy-status')).toHaveTextContent('3 to fix');
    await user.click(screen.getByRole('button', { name: 'Show or hide the list of problems' }));
    await user.click(screen.getByRole('checkbox', { name: 'Show all errors' }));
    expect(screen.getByTestId('field-tokenEndpoint')).toHaveAttribute('aria-invalid', 'true');
    await user.click(screen.getByRole('radio', { name: 'Bearer token' }));
    expect(screen.getByTestId('policy-status')).toHaveTextContent('1 to fix');
    await user.type(screen.getByTestId('field-bearerToken'), 'token');
    expect(screen.getByTestId('policy-status')).toHaveTextContent('Ready');
  });

  it('lists the problems in a collapsed panel and jumps to a field from it', async () => {
    const { user } = renderEditor('oauth2-generator');
    expect(byTestId('policy-problems')).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Show or hide the list of problems' }));
    const panel = screen.getByTestId('policy-problems');
    expect(panel).toHaveTextContent('Needs attention');
    expect(panel).toHaveTextContent('tokenEndpoint');
    await user.click(screen.getAllByRole('button', { name: 'Go to field' })[0]);
    expect(screen.getByTestId('field-tokenEndpoint')).toHaveFocus();
  });

  it('keeps advanced fields inside the expandable section', async () => {
    const { user } = renderEditor('oauth2-generator');
    const toggle = screen.getByRole('button', { name: 'Advanced Settings' });
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    await user.click(toggle);
    expect(toggle).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByTestId('field-tokenPurgeStatusCodes')).toBeVisible();
    await user.click(toggle);
    await waitFor(() =>
      expect(screen.getByTestId('field-tokenPurgeStatusCodes')).not.toBeVisible(),
    );
  });

  it('editing an existing config selects its mode and says Update', () => {
    renderEditor('oauth2-generator', { existingValues: { bearerToken: 'abc' } });
    expect(screen.getByRole('radio', { name: 'Bearer token' })).toBeChecked();
    expect(submit()).toHaveTextContent('Update');
    expect(submit()).not.toBeDisabled();
  });

  it('shows errors after a submit attempt, next to the field', async () => {
    const { user, onSubmit } = renderEditor('set-headers');
    await user.click(screen.getByRole('button', { name: 'Expand Request' }));
    await user.click(screen.getByTestId('add-request.headers-item'));
    await user.type(byTestId('field-request.headers.0.name')!, 'bad header!');
    await user.click(submit());
    expect(onSubmit).not.toHaveBeenCalled();
    expect(byTestId('policy-param-errors')).toBeInTheDocument();
    expect(screen.getAllByText(/Doesn't match the expected format/).length).toBeGreaterThan(0);
  });

  it('readOnly shows values but no submit button', () => {
    renderEditor('cors', {
      readOnly: true,
      existingValues: { allowedOrigins: ['https://a.example.com'] },
    });
    expect(byTestId('policy-param-submit')).not.toBeInTheDocument();
    expect(screen.getByTestId('policy-param-cancel')).toHaveTextContent('Close');
    expect(screen.getByText('https://a.example.com')).toBeInTheDocument();
  });
});
