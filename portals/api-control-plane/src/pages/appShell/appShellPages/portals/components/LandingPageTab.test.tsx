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
import { LandingPageTab, type LandingPageTabProps } from './LandingPageTab';

const REPLACES = /Publishing replaces the portal’s default section/;
const BLANK = 'A blank landing page is not published.';

const renderTab = (props: LandingPageTabProps) => renderWithProviders(<LandingPageTab {...props} />);

describe('LandingPageTab preview pane', () => {
  it('is named Preview while editing and Published page when read-only', () => {
    const { unmount } = renderTab({ text: '# Loans' });
    expect(screen.getByRole('region', { name: 'Preview' })).toBeInTheDocument();
    unmount();

    renderTab({ readOnly: true, text: '# Loans' });
    expect(screen.getByRole('region', { name: 'Published page' })).toBeInTheDocument();
  });
});

describe('LandingPageTab notices', () => {
  it('says publishing replaces the default section for a page generated from the API details', () => {
    renderTab({ generated: true, text: '## Overview\n' });

    expect(screen.getByText(REPLACES)).toBeInTheDocument();
    expect(screen.queryByText(BLANK)).not.toBeInTheDocument();
  });

  it.each(['', '  \n\t'])('says a blank page is not published, never that it replaces anything (%j)', (text) => {
    renderTab({ generated: true, text });

    expect(screen.getByText(BLANK)).toBeInTheDocument();
    expect(screen.queryByText(REPLACES)).not.toBeInTheDocument();
  });

  it('says nothing for a page that was written or already stored', () => {
    renderTab({ text: '# Loans' });

    expect(screen.queryByText(REPLACES)).not.toBeInTheDocument();
    expect(screen.queryByText(BLANK)).not.toBeInTheDocument();
  });

  it('says nothing in the published view', () => {
    renderTab({ readOnly: true, text: '' });

    expect(screen.queryByText(BLANK)).not.toBeInTheDocument();
  });
});
