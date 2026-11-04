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

import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import ComingSoon from './ComingSoon';

describe('ComingSoon', () => {
  it('shows the default title and message', () => {
    render(<ComingSoon />);

    expect(screen.getByRole('heading', { name: 'Coming soon' })).toBeInTheDocument();
    expect(screen.getByText(/We're building this feature right now/)).toBeInTheDocument();
  });

  it('takes a custom title and message', () => {
    render(<ComingSoon title="Insights" message="Soon." />);

    expect(screen.getByRole('heading', { name: 'Insights' })).toBeInTheDocument();
    expect(screen.getByText('Soon.')).toBeInTheDocument();
  });

  it('has no home button unless one is asked for', () => {
    render(<ComingSoon />);

    expect(screen.queryByRole('button', { name: 'Go to home' })).not.toBeInTheDocument();
  });

  it('calls onGoHome when the home button is clicked', () => {
    const onGoHome = vi.fn();
    render(<ComingSoon onGoHome={onGoHome} />);

    fireEvent.click(screen.getByRole('button', { name: 'Go to home' }));
    expect(onGoHome).toHaveBeenCalledTimes(1);
  });

  it('hides the decorative hourglass from assistive tech', () => {
    const { container } = render(<ComingSoon />);

    expect(container.querySelector('.cs-art')).toHaveAttribute('aria-hidden', 'true');
  });
});
