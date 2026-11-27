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

import { describe, expect, it } from 'vitest';

import { renderWithProviders, screen } from '@/test/utils';

import { AppPage } from './AppPage';

describe('AppPage', () => {
  it('puts the console footer at the end of the page content', () => {
    renderWithProviders(<AppPage>Page body</AppPage>);

    expect(screen.getByText('Page body')).toBeInTheDocument();
    expect(screen.getByText(/Choreo/)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Terms of Use' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Privacy Policy' })).toBeInTheDocument();
  });

  it('leaves the footer off when a page passes footer={null}', () => {
    renderWithProviders(<AppPage footer={null}>Page body</AppPage>);

    expect(screen.queryByText(/Choreo/)).not.toBeInTheDocument();
  });
});
