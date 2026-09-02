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
import { renderWithProviders, screen } from '@/test/utils';
import { GeneralCreateApiForm } from './GeneralCreateApiForm';

const scope = makeConsoleScope();
const route = '/organizations/api-platform-demo/projects/retail-apis/apis/create';

const renderForm = (initialValues?: Parameters<typeof GeneralCreateApiForm>[0]['initialValues']) =>
  renderWithProviders(
    <GeneralCreateApiForm initialValues={initialValues} onBack={() => {}} onSubmit={() => {}} />,
    { route, scope },
  );

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
    expect(screen.getByLabelText(/Base Path/)).toHaveValue(
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
    expect(screen.getByLabelText(/Base Path/)).toHaveValue('/public/orders');
  });

  it('leaves a restored base path alone when the display name is edited afterwards', async () => {
    const { user } = renderForm({
      context: '/public/orders',
      displayName: 'Orders API',
      id: 'orders-v2',
      version: '2.1',
    });

    await user.type(screen.getByLabelText(/Display name/), ' v2');

    expect(screen.getByLabelText(/Identifier/)).toHaveValue('orders-v2');
    expect(screen.getByLabelText(/Base Path/)).toHaveValue('/public/orders');
  });
});
