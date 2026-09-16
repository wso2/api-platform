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
  it('shows a skeleton preview of the Query/Mutation/Object shape alongside the empty state', () => {
    renderWithProviders(<GraphqlSchemaExplorer />);

    expect(screen.getByText('QUERY')).toBeInTheDocument();
    expect(screen.getByText('MUTATION')).toBeInTheDocument();
    expect(screen.getByText('OBJECT')).toBeInTheDocument();
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

  it('gives Query, Mutation and each type kind their own chip color instead of a flat gray', () => {
    renderWithProviders(<GraphqlSchemaExplorer sdl={SDL} />);

    expect(screen.getByText('Query').closest('.MuiChip-root')).toHaveClass('MuiChip-colorInfo');
    expect(screen.getByText('Mutation').closest('.MuiChip-root')).toHaveClass('MuiChip-colorSuccess');
    // `Country` and `Review` are both OBJECT kind, so both their kind chips
    // apply the same color — checking one of them is enough.
    expect(screen.getAllByText('OBJECT')[0].closest('.MuiChip-root')).toHaveClass(
      'MuiChip-colorPrimary',
    );
    // ENUM shares the neutral `default` gray with SCALAR — the two "terminal
    // value" kinds — since only 5 tones are visually distinct in this theme
    // for 6 kinds (see `SchemaChipColor`'s doc comment).
    expect(screen.getByText('ENUM').closest('.MuiChip-root')).toHaveClass('MuiChip-colorDefault');
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
