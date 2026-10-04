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

import { resetHttpClient } from '@/api/core/http';
import { server } from '@/test/server';
import { collection } from '@/test/msw';
import { makeConsoleScope } from '@/test/mockScope';
import { renderWithProviders, screen, waitFor } from '@/test/utils';
import { GeneralCreateApiForm } from './GeneralCreateApiForm';

const scope = makeConsoleScope();
const route = '/organizations/api-platform-demo/projects/retail-apis/apis/create';

type FormProps = Parameters<typeof GeneralCreateApiForm>[0];

const renderForm = (
  initialValues?: FormProps['initialValues'],
  serverErrors?: FormProps['serverErrors'],
  overrides?: Partial<FormProps>,
) =>
  renderWithProviders(
    <GeneralCreateApiForm
      initialValues={initialValues}
      onBack={() => {}}
      onSubmit={() => {}}
      serverErrors={serverErrors}
      {...overrides}
    />,
    { route, scope },
  );

/** What the wizard hands back after a rejected create. */
const submitted = {
  context: '/retail-apis/orders-api/v1.0',
  displayName: 'Orders API',
  id: 'orders-api',
  version: '1.0',
} as const;

/**
 * The identifier field probes availability as it settles, so the listing that
 * probe reads has to exist or the render errors on an unhandled request.
 */
beforeEach(() => {
  resetHttpClient();
  server.use(collection('/rest-apis', []));
});

describe('GeneralCreateApiForm — initial values', () => {
  it('explains that the scratch backend is a placeholder until it is replaced', async () => {
    const { user } = renderForm({
      displayName: 'Untitled API',
      upstream: { main: { url: 'https://example.com' } },
    });

    expect(
      screen.getByText(/using https:\/\/example\.com as a placeholder backend/i),
    ).toBeInTheDocument();

    const targetUrl = screen.getByLabelText(/Target URL/);
    await user.clear(targetUrl);
    await user.type(targetUrl, 'https://api.example.org');

    expect(
      screen.queryByText(/using https:\/\/example\.com as a placeholder backend/i),
    ).not.toBeInTheDocument();
  });

  it('stays quiet once the user has been into the backend field, whatever they typed', async () => {
    // The notice is about an endpoint the user never chose. A user who types
    // the placeholder domain deliberately has chosen one, so telling them it
    // is a placeholder is noise — the string is not what decides this.
    const { user } = renderForm({
      displayName: 'Untitled API',
      upstream: { main: { url: 'https://example.com' } },
    });

    const targetUrl = screen.getByLabelText(/Target URL/);
    await user.clear(targetUrl);
    await user.type(targetUrl, 'https://example.com');

    expect(
      screen.queryByText(/using https:\/\/example\.com as a placeholder backend/i),
    ).not.toBeInTheDocument();
  });

  it('stays quiet across the remount a rejected create causes', async () => {
    // The form is unmounted while the progress screen stands in for it, so a
    // flag kept here would forget the user had already chosen this URL. The
    // wizard holds that provenance and hands it back.
    renderForm(
      { displayName: 'Untitled API', upstream: { main: { url: 'https://example.com' } } },
      undefined,
      { initialUpstreamEdited: true },
    );

    expect(
      screen.queryByText(/using https:\/\/example\.com as a placeholder backend/i),
    ).not.toBeInTheDocument();
  });

  it('reports the first backend edit so the wizard can hand it back', async () => {
    const onUpstreamEdited = vi.fn();
    const { user } = renderForm(
      { displayName: 'Untitled API', upstream: { main: { url: 'https://example.com' } } },
      undefined,
      { onUpstreamEdited },
    );

    await user.type(screen.getByLabelText(/Target URL/), '/v1');

    expect(onUpstreamEdited).toHaveBeenCalled();
  });

  it('says nothing about a placeholder when the draft named a real backend', () => {
    renderForm({
      displayName: 'Orders API',
      upstream: { main: { url: 'https://orders.internal' } },
    });

    expect(
      screen.queryByText(/using https:\/\/example\.com as a placeholder backend/i),
    ).not.toBeInTheDocument();
  });

  it('derives the base path from project, identifier and version when the draft names none', () => {
    renderForm({ displayName: 'Orders API', version: '2.1' });

    expect(screen.getByLabelText(/Identifier/)).toHaveValue('orders-api');
    // The platform's own base path shape, not anything read off a document.
    expect(screen.getByLabelText(/Context/)).toHaveValue(
      `/${scope.activeScope.projectHandler}/orders-api/v2.1`,
    );
  });

  it('keeps an identifier and base path the draft already carries', () => {
    // This is the restore path: a failed create hands back what the user
    // actually submitted, and neither field may be regenerated over the top.
    renderForm({
      context: '/public/orders',
      displayName: 'Orders API',
      id: 'orders-v2',
      version: '2.1',
    });

    expect(screen.getByLabelText(/Identifier/)).toHaveValue('orders-v2');
    expect(screen.getByLabelText(/Context/)).toHaveValue('/public/orders');
  });

  it('leaves a restored base path alone when the display name is edited afterwards', async () => {
    const { user } = renderForm(
      { context: '/public/orders', displayName: 'Orders API', id: 'orders-v2', version: '2.1' },
      undefined,
      { initialBasePathEdited: true, initialIdentifierEdited: true },
    );

    await user.type(screen.getByLabelText(/^Name/), ' v2');

    expect(screen.getByLabelText(/Identifier/)).toHaveValue('orders-v2');
    expect(screen.getByLabelText(/Context/)).toHaveValue('/public/orders');
  });
});

describe('GeneralCreateApiForm — a rejected submission', () => {
  it('shows the server’s reason on the field it names', async () => {
    renderForm(submitted, {
      fields: { id: 'An API with this identifier already exists.' },
      unmapped: [],
    });

    expect(
      await screen.findByText('An API with this identifier already exists.'),
    ).toBeInTheDocument();
    // Pinned to the input, not just announced: the message is what the
    // identifier field describes itself with.
    expect(screen.getByLabelText(/Identifier/)).toHaveAccessibleDescription(
      'An API with this identifier already exists.',
    );
  });

  it('moves focus to the first field the server named', async () => {
    renderForm(submitted, {
      fields: { context: 'This base path is already in use.' },
      unmapped: [],
    });

    await waitFor(() => expect(screen.getByLabelText(/Context/)).toHaveFocus());
  });

  it('retracts the message once that value is edited', async () => {
    const { user } = renderForm(submitted, {
      fields: { id: 'An API with this identifier already exists.' },
      unmapped: [],
    });

    await user.type(screen.getByLabelText(/Identifier/), '-v2');

    // The server judged what was sent; it has no opinion on what is being
    // typed now, so the objection goes with the value that caused it.
    expect(
      screen.queryByText('An API with this identifier already exists.'),
    ).not.toBeInTheDocument();
  });

  it('summarises a rejection that names no field at all', () => {
    // A conflict arrives as prose with no `errors[]`, and there is nothing to
    // pin it to — so the form says it once, at the top, and keeps saying it.
    renderForm(submitted, {
      fields: {},
      message: 'An API with context /orders and version 1.0 already exists.',
      unmapped: [],
    });

    expect(screen.getByRole('alert')).toHaveTextContent(
      'An API with context /orders and version 1.0 already exists.',
    );
  });

  it('lists a field error that belongs to no input on this form', () => {
    renderForm(submitted, { fields: {}, unmapped: ['Unknown project.'] });

    expect(screen.getByRole('alert')).toHaveTextContent('Unknown project.');
  });

  it('says nothing when the last submission was not rejected', () => {
    renderForm(submitted);

    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });
});
