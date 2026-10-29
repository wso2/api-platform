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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { describe, expect, it, vi } from 'vitest';

import { renderWithProviders, screen } from '../../../../test/utils';
import { EmptyState, ErrorState, LoadingState } from './PublishStates';

vi.mock('../../../../Components/AILoader', () => ({ default: () => <div data-testid="loader" /> }));

describe('LoadingState', () => {
  it('shows the loader with a default label', () => {
    renderWithProviders(<LoadingState />);

    expect(screen.getByTestId('loader')).toBeInTheDocument();
    expect(screen.getByText('Loading')).toBeInTheDocument();
  });

  it('shows the label it is given instead', () => {
    renderWithProviders(<LoadingState label="Loading MCP Hubs" />);

    expect(screen.getByText('Loading MCP Hubs')).toBeInTheDocument();
    expect(screen.queryByText('Loading')).not.toBeInTheDocument();
  });
});

describe('EmptyState', () => {
  it('shows its title alone when it has nothing more to say', () => {
    renderWithProviders(<EmptyState title="No MCP Hubs Available" />);

    expect(screen.getByRole('heading', { name: 'No MCP Hubs Available' })).toBeInTheDocument();
  });

  it('shows the description and the action that fills it', () => {
    renderWithProviders(
      <EmptyState action={<button type="button">Add one</button>} description="Add a hub first." title="Nothing" />
    );

    expect(screen.getByText('Add a hub first.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add one' })).toBeInTheDocument();
  });
});

describe('ErrorState', () => {
  it('falls back to a generic title', () => {
    renderWithProviders(<ErrorState />);

    expect(screen.getByRole('alert')).toHaveTextContent('Something went wrong');
  });

  it('shows the title and message it is given', () => {
    renderWithProviders(<ErrorState message="Ask your admin for access." title="You don’t have permission" />);

    const alert = screen.getByRole('alert');
    expect(alert).toHaveTextContent('You don’t have permission');
    expect(alert).toHaveTextContent('Ask your admin for access.');
  });
});
