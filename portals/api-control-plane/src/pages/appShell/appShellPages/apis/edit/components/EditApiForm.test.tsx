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

import { describe, expect, it, vi } from 'vitest';

import { aRestApi, type RestApiFixture } from '@/test/msw';
import { renderWithProviders, screen } from '@/test/utils';
import { EditApiForm, type ApiBasicInfoFormValues } from './EditApiForm';

const anApi = (overrides: Partial<RestApiFixture> = {}) =>
  aRestApi({
    context: '/pizza',
    description: 'Pizza ordering',
    displayName: 'Pizza Shack',
    id: 'pizza-shack',
    upstream: { main: { url: 'https://upstream.test' } },
    version: '1.0.0',
    ...overrides,
  });

const renderForm = (
  options: {
    api?: RestApiFixture;
    fieldErrors?: Record<string, string>;
    isSaving?: boolean;
    onSubmit?: (values: ApiBasicInfoFormValues) => void;
  } = {},
) => {
  const onSubmit = options.onSubmit ?? vi.fn();
  const onCancel = vi.fn();

  const result = renderWithProviders(
    <EditApiForm
      api={options.api ?? anApi()}
      fieldErrors={options.fieldErrors}
      isSaving={options.isSaving ?? false}
      onCancel={onCancel}
      onSubmit={onSubmit}
    />,
  );

  return { ...result, onCancel, onSubmit };
};

const save = (user: ReturnType<typeof renderForm>['user']) =>
  user.click(screen.getByRole('button', { name: /Save changes/ }));

describe('EditApiForm — initial values', () => {
  it('opens with the API it was given', () => {
    renderForm();

    expect(screen.getByLabelText(/^Name/)).toHaveValue('Pizza Shack');
    expect(screen.getByLabelText(/Version/)).toHaveValue('1.0.0');
    expect(screen.getByLabelText(/Context/)).toHaveValue('/pizza');
    expect(screen.getByLabelText(/Description/)).toHaveValue('Pizza ordering');
    expect(screen.getByLabelText(/Target URL/)).toHaveValue('https://upstream.test');
  });

  it('shows the identifier but does not let it be edited', () => {
    // `PUT /rest-apis/{restApiId}` rejects a body whose `id` differs from the
    // path and there is no rename operation, so the handle is reference only.
    renderForm();

    const identifier = screen.getByLabelText(/Identifier/);
    expect(identifier).toHaveValue('pizza-shack');
    expect(identifier).toBeDisabled();
  });

  it('leaves an absent description as an empty field rather than "undefined"', () => {
    renderForm({ api: anApi({ description: undefined }) });

    expect(screen.getByLabelText(/Description/)).toHaveValue('');
  });
});

describe('EditApiForm — validation', () => {
  it('refuses to submit without a name, and says so', async () => {
    const { onSubmit, user } = renderForm();

    await user.clear(screen.getByLabelText(/^Name/));
    await save(user);

    expect(screen.getByText('Enter a name.')).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it('requires the context to be an absolute path', async () => {
    const { onSubmit, user } = renderForm();

    await user.clear(screen.getByLabelText(/Context/));
    await user.type(screen.getByLabelText(/Context/), 'pizza');
    await save(user);

    expect(
      screen.getByText('Start with / and use only letters, numbers, hyphens, dots and slashes.'),
    ).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it('rejects a version carrying a space', async () => {
    const { onSubmit, user } = renderForm();

    await user.clear(screen.getByLabelText(/Version/));
    await user.type(screen.getByLabelText(/Version/), '1 0');
    await save(user);

    expect(
      screen.getByText(
        'Use letters, numbers, dots, hyphens and underscores — no spaces or slashes.',
      ),
    ).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it('rejects a target URL that is not a full http(s) URL', async () => {
    const { onSubmit, user } = renderForm();

    await user.clear(screen.getByLabelText(/Target URL/));
    await user.type(screen.getByLabelText(/Target URL/), 'upstream.test');
    await save(user);

    expect(
      screen.getByText('Enter a full URL, for example https://api.example.com.'),
    ).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it('stays quiet until a rule is actually broken', () => {
    renderForm();

    expect(screen.queryByText('Enter a name.')).not.toBeInTheDocument();
  });

  it('binds a rejected save back onto the field the server named', () => {
    renderForm({ fieldErrors: { context: 'Context already in use by another API.' } });

    expect(screen.getByText('Context already in use by another API.')).toBeInTheDocument();
  });
});

describe('EditApiForm — submitting', () => {
  it('hands back the five fields, trimmed', async () => {
    const { onSubmit, user } = renderForm();

    await user.clear(screen.getByLabelText(/^Name/));
    await user.type(screen.getByLabelText(/^Name/), '  Pizza Shack v2  ');
    await user.clear(screen.getByLabelText(/Description/));
    await user.type(screen.getByLabelText(/Description/), 'Now with sides');
    await save(user);

    expect(onSubmit).toHaveBeenCalledWith({
      context: '/pizza',
      description: 'Now with sides',
      displayName: 'Pizza Shack v2',
      targetUrl: 'https://upstream.test',
      version: '1.0.0',
    });
  });

  it('locks every control while the save is in flight', () => {
    renderForm({ isSaving: true });

    expect(screen.getByLabelText(/^Name/)).toBeDisabled();
    expect(screen.getByRole('button', { name: /Save changes/ })).toBeDisabled();
    expect(screen.getByRole('button', { name: /Cancel/ })).toBeDisabled();
  });

  it('leaves without saving when cancelled', async () => {
    const { onCancel, onSubmit, user } = renderForm();

    await user.click(screen.getByRole('button', { name: /Cancel/ }));

    expect(onCancel).toHaveBeenCalledTimes(1);
    expect(onSubmit).not.toHaveBeenCalled();
  });
});

describe('EditApiForm — shared upstream', () => {
  const withRef = anApi({ upstream: { main: { ref: 'retail-backend' } } });

  it('locks the target URL and names the upstream it points at', () => {
    renderForm({ api: withRef });

    expect(screen.getByLabelText(/Target URL/)).toBeDisabled();
    expect(
      screen.getByText(/Routed through the shared upstream “retail-backend”/),
    ).toBeInTheDocument();
  });

  it('still submits: an API with no URL of its own is not an invalid one', async () => {
    const { onSubmit, user } = renderForm({ api: withRef });

    await save(user);

    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({ displayName: 'Pizza Shack', targetUrl: '' }),
    );
  });
});
