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

import { beforeEach, describe, expect, it } from 'vitest';

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
) =>
  renderWithProviders(
    <GeneralCreateApiForm
      initialValues={initialValues}
      onBack={() => {}}
      onSubmit={() => {}}
      serverErrors={serverErrors}
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
    const { user } = renderForm({
      context: '/public/orders',
      displayName: 'Orders API',
      id: 'orders-v2',
      version: '2.1',
    });

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
