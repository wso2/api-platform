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
import { GraphqlSchemaExplorer } from './GraphqlSchemaExplorer';

const SDL = `
  type Query {
    country(code: ID!): Country
  }

  type Mutation {
    addReview(code: ID!): Review
  }

  type Country {
    code: ID!
    name: String!
  }

  type Review {
    id: ID!
  }

  enum Status {
    ACTIVE
    INACTIVE
  }
`;

describe('GraphqlSchemaExplorer — before anything has loaded', () => {
  it('shows the empty state when no `sdl` is given', () => {
    renderWithProviders(<GraphqlSchemaExplorer />);

    expect(screen.getByText('Schema will show here')).toBeInTheDocument();
    // Neither view toggle nor a source description makes sense with nothing loaded.
    expect(screen.queryByRole('button', { name: 'Explorer' })).not.toBeInTheDocument();
  });

  // Illustrative only — a hint at the shape a resolved schema takes, not a
  // dummy schema of its own, so it shows for every path with no `sdl` yet:
  // "Start with a schema" before importing, "Design from scratch" before
  // checking an endpoint, and a scratch endpoint where introspection is
  // disabled (which also never resolves an `sdl`).
  it('shows a skeleton preview of the Query/Mutation/Subscription shape alongside the empty state', () => {
    renderWithProviders(<GraphqlSchemaExplorer />);

    expect(screen.getByText('QUERY')).toBeInTheDocument();
    expect(screen.getByText('MUTATION')).toBeInTheDocument();
    expect(screen.getByText('SUBSCRIPTION')).toBeInTheDocument();
  });
});

// Pins the fix that lets the explorer show the backend's own reason a
// validation attempt failed, instead of falling back to the same "Schema
// will show here" empty state used before anything has even been attempted.
describe('GraphqlSchemaExplorer — a failed validation attempt', () => {
  it('shows each SDL error with its line and column, not the generic empty state', () => {
    renderWithProviders(
      <GraphqlSchemaExplorer
        error={{ sdlErrors: [{ column: 12, line: 3, message: 'Unexpected Name "this"' }] }}
      />,
    );

    expect(screen.getByText(/Line 3, column 12/)).toBeInTheDocument();
    expect(screen.getByText(/Unexpected Name "this"/)).toBeInTheDocument();
    expect(screen.queryByText('Schema will show here')).not.toBeInTheDocument();
  });

  it('falls back to the generic message when no sdlErrors are given (a url/introspection failure)', () => {
    renderWithProviders(
      <GraphqlSchemaExplorer error={{ message: 'Introspection could not be completed.' }} />,
    );

    expect(screen.getByText('Introspection could not be completed.')).toBeInTheDocument();
  });

  it('shows a sterile fallback when neither sdlErrors nor a message is given', () => {
    renderWithProviders(<GraphqlSchemaExplorer error={{}} />);

    expect(screen.getByText('Schema could not be resolved.')).toBeInTheDocument();
  });
});

describe('GraphqlSchemaExplorer — a resolved schema', () => {
  it('renders the Query/Mutation entry points and the other named types', () => {
    renderWithProviders(<GraphqlSchemaExplorer sdl={SDL} sourceDescription="Fetched from example.com" />);

    expect(screen.getByText('Fetched from example.com')).toBeInTheDocument();
    expect(screen.getByText('country')).toBeInTheDocument();
    expect(screen.getByText('addReview')).toBeInTheDocument();
    // Each type is its own expandable row (an Accordion), so `Country`'s
    // return-type mention inside the `country` field row above it doesn't
    // collide with the query: only the row's own header is a button.
    expect(screen.getByRole('button', { name: /Country/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Status/ })).toBeInTheDocument();
  });

  it('filters the type list by search text', async () => {
    const { user } = renderWithProviders(<GraphqlSchemaExplorer sdl={SDL} />);

    await user.type(screen.getByPlaceholderText('Search types and fields'), 'status');

    expect(screen.getByRole('button', { name: /Status/ })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Country/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Review/ })).not.toBeInTheDocument();
  });

  it('filters the type list by kind', async () => {
    const { user } = renderWithProviders(<GraphqlSchemaExplorer sdl={SDL} />);

    await user.click(screen.getByRole('combobox'));
    await user.click(await screen.findByRole('option', { name: 'ENUM' }));

    expect(screen.getByRole('button', { name: /Status/ })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Country/ })).not.toBeInTheDocument();
  });

  it('switches to the SDL view and shows the canonically-formatted text', async () => {
    const { user } = renderWithProviders(<GraphqlSchemaExplorer sdl={SDL} />);

    await user.click(screen.getByRole('button', { name: 'SDL' }));

    expect(screen.getByText(/type Query/)).toBeInTheDocument();
    // The field-row explorer is gone once the raw-text view is shown.
    expect(screen.queryByText('country')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Download SDL' })).toBeInTheDocument();
  });

  it('shows a parse error instead of the explorer for invalid SDL', () => {
    renderWithProviders(<GraphqlSchemaExplorer sdl="type Query { broken" />);

    expect(screen.getByText(/could not be parsed/)).toBeInTheDocument();
  });

  it('gives Query and Mutation their own chip color, with no chip at all on a type’s kind label', () => {
    renderWithProviders(<GraphqlSchemaExplorer sdl={SDL} />);

    expect(screen.getByText('QUERY').closest('.MuiChip-root')).toHaveClass('MuiChip-colorInfo');
    expect(screen.getByText('MUTATION').closest('.MuiChip-root')).toHaveClass('MuiChip-colorSuccess');
    // A type's own kind (OBJECT/ENUM/…) is plain text now, not a colored
    // chip — only the three root operations keep the chip treatment.
    expect(screen.getAllByText('OBJECT')[0].closest('.MuiChip-root')).toBeNull();
    expect(screen.getByText('ENUM').closest('.MuiChip-root')).toBeNull();
  });

  it('can switch back to Explorer after viewing SDL', async () => {
    const { user } = renderWithProviders(<GraphqlSchemaExplorer sdl={SDL} />);

    await user.click(screen.getByRole('button', { name: 'SDL' }));
    expect(screen.getByRole('button', { name: 'Explorer' })).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Explorer' }));

    expect(screen.getByText('country')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'SDL' })).toBeInTheDocument();
  });
});
