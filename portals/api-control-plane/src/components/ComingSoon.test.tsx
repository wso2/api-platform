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
import { FormattedMessage } from 'react-intl';

import { renderWithProviders, screen } from '@/test/utils';
import { ComingSoon } from './ComingSoon';

describe('ComingSoon', () => {
  it('names the feature that is coming', () => {
    renderWithProviders(<ComingSoon feature="API insights" />);

    expect(screen.getByRole('heading', { name: 'Coming Soon' })).toBeInTheDocument();
    expect(screen.getByText('API insights will be available soon.')).toBeInTheDocument();
  });

  it('takes the feature as a translated message', () => {
    renderWithProviders(
      <ComingSoon feature={<FormattedMessage defaultMessage="Compliance reporting" id="test.feature" />} />,
    );

    expect(screen.getByText('Compliance reporting will be available soon.')).toBeInTheDocument();
  });

  it('shows the extra detail line only when given', () => {
    const { rerender } = renderWithProviders(<ComingSoon feature="Insights" />);
    expect(screen.queryByText('Try Logs meanwhile.')).not.toBeInTheDocument();

    rerender(<ComingSoon detail="Try Logs meanwhile." feature="Insights" />);
    expect(screen.getByText('Try Logs meanwhile.')).toBeInTheDocument();
  });

  it('links to the alternative page when one is offered', () => {
    renderWithProviders(
      <ComingSoon action={{ label: 'Open logs', to: '/organizations/acme/observability/logs' }} feature="Metrics" />,
    );

    expect(screen.getByRole('link', { name: 'Open logs' })).toHaveAttribute(
      'href',
      '/organizations/acme/observability/logs',
    );
  });

  it('has no link without an alternative page', () => {
    renderWithProviders(<ComingSoon feature="Metrics" />);

    expect(screen.queryByRole('link')).not.toBeInTheDocument();
  });

  it('draws the hourglass as decoration, hidden from assistive tech', () => {
    const { container } = renderWithProviders(<ComingSoon feature="Metrics" />);

    expect(container.querySelector('.cs-art')).toHaveAttribute('aria-hidden', 'true');
    expect(container.querySelector('.cs-hourglass')).toBeInTheDocument();
  });
});
