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

import { renderWithProviders, screen, within } from '@/test/utils';
import { GraphqlSchemaExplorer } from './GraphqlSchemaExplorer';

const SDL = `
  scalar DateTime

  type Query {
    country(code: ID!): Country
    countries(limit: Int = 20, region: String = "Asia"): [Country!]!
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
    // The placeholder speaks for itself — no "Schema" heading above it.
    expect(screen.queryByText('Schema')).not.toBeInTheDocument();
    // Neither view toggle nor a source description makes sense with nothing loaded.
    expect(screen.queryByRole('button', { name: 'Explorer' })).not.toBeInTheDocument();
  });

  // Illustrative only — a hint at the shape a resolved schema takes, not a
  // dummy schema of its own, so it shows for every path with no `sdl` yet:
  // "Start with a schema" before importing, "Start from scratch" before
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

  // The backend's message for a failure without SDL errors is a fixed,
  // generic string ("…could not be used to derive a GraphQL schema…") that
  // the source form already reports in its own words, so repeating it here
  // only showed the same failure twice. The pane keeps its empty state.
  it('keeps the empty state for a failure without SDL errors, rather than repeating the form', () => {
    renderWithProviders(
      <GraphqlSchemaExplorer
        error={{ message: 'The provided endpoint could not be used to derive a GraphQL schema.' }}
      />,
    );

    expect(screen.getByText('Schema will show here')).toBeInTheDocument();
    expect(screen.queryByText(/could not be used to derive/)).not.toBeInTheDocument();
  });

  it('keeps the empty state for an empty failure object', () => {
    renderWithProviders(<GraphqlSchemaExplorer error={{}} />);

    expect(screen.getByText('Schema will show here')).toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });
});

describe('GraphqlSchemaExplorer — a resolved schema', () => {
  it('renders the Query/Mutation entry points and the other named types', () => {
    renderWithProviders(<GraphqlSchemaExplorer sdl={SDL} sourceDescription="Fetched from example.com" />);

    expect(screen.getByText('Schema')).toBeInTheDocument();
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

  it('filters the type list by an enum value, not just the enum type name', async () => {
    // Regression test: filteredTypes' matches() call used to check only
    // type.name, so searching a value the enum lists (rather than the enum's
    // own name) silently returned nothing — "search types and fields" implied
    // this should work, and the underlying summary data always had
    // enumValues available; the filter just never looked at it.
    const { user } = renderWithProviders(<GraphqlSchemaExplorer sdl={SDL} />);

    await user.type(screen.getByPlaceholderText('Search types and fields'), 'ACTIVE');

    expect(screen.getByRole('button', { name: /Status/ })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Country/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Review/ })).not.toBeInTheDocument();
  });

  it('finds a field of a non-root type and expands that type to show it', async () => {
    // Regression test: only root Query/Mutation fields were searched; a field
    // on any other type (Country.name) filtered its type out entirely, so the
    // search returned nothing despite promising "types and fields".
    const { user } = renderWithProviders(<GraphqlSchemaExplorer sdl={SDL} />);

    await user.type(screen.getByPlaceholderText('Search types and fields'), 'name');

    const countryRow = screen.getByRole('button', { name: /Country/ });
    expect(countryRow).toHaveAttribute('aria-expanded', 'true');
    expect(screen.queryByRole('button', { name: /Review/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Status/ })).not.toBeInTheDocument();
  });

  it('keeps a type matched by its own name collapsed', async () => {
    const { user } = renderWithProviders(<GraphqlSchemaExplorer sdl={SDL} />);

    await user.type(screen.getByPlaceholderText('Search types and fields'), 'Country');

    expect(screen.getByRole('button', { name: /Country/ })).toHaveAttribute('aria-expanded', 'false');
  });

  it('says so when nothing in the schema matches', async () => {
    const { user } = renderWithProviders(<GraphqlSchemaExplorer sdl={SDL} />);

    await user.type(screen.getByPlaceholderText('Search types and fields'), 'zzz');

    expect(screen.getByText('Nothing in this schema matches "zzz".')).toBeInTheDocument();
  });

  it('labels an enum by its values and an object by its fields', () => {
    renderWithProviders(<GraphqlSchemaExplorer sdl={SDL} />);

    expect(screen.getByText('2 values')).toBeInTheDocument();
    expect(screen.getAllByText('2 fields')).toHaveLength(1);
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

  it('draws each operation as its own REST-style row, badge colored by operation kind', () => {
    renderWithProviders(<GraphqlSchemaExplorer sdl={SDL} />);

    // One badge per operation field, colored from the shared Swagger palette
    // so a GraphQL schema reads like a REST resource list.
    expect(screen.getAllByText('QUERY')[0]).toHaveStyle({ backgroundColor: '#4286de' });
    expect(screen.getByText('MUTATION')).toHaveStyle({ backgroundColor: '#49cc90' });
  });

  it('wraps every non-root type in one collapsible Types group, each drawn like an operation row', async () => {
    const { user } = renderWithProviders(<GraphqlSchemaExplorer sdl={SDL} />);

    const group = screen.getByRole('button', { name: /Types.*4 types/ });
    expect(group).toHaveAttribute('aria-expanded', 'true');
    // Same row as QUERY/MUTATION, badged with the SDL keyword instead.
    expect(screen.getByRole('button', { name: 'Show details for TYPE Country' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Show details for ENUM Status' })).toBeInTheDocument();
    expect(screen.getAllByText('TYPE')[0]).toHaveStyle({ backgroundColor: '#6b7d99' });

    await user.click(group);

    expect(group).toHaveAttribute('aria-expanded', 'false');
  });

  it('wraps queries and mutations in their own collapsible groups, like Types, and omits an empty one', async () => {
    const { user } = renderWithProviders(<GraphqlSchemaExplorer sdl={SDL} />);

    const queries = screen.getByRole('button', { name: /Queries.*2 queries/ });
    expect(screen.getByRole('button', { name: /Mutations.*1 mutation/ })).toHaveAttribute('aria-expanded', 'true');
    // The schema declares no Subscription type, so no empty group is drawn for it.
    expect(screen.queryByRole('button', { name: /Subscriptions/ })).not.toBeInTheDocument();

    await user.click(queries);

    expect(queries).toHaveAttribute('aria-expanded', 'false');
  });

  it('shows an argument\'s default value next to its type', async () => {
    const { user } = renderWithProviders(<GraphqlSchemaExplorer sdl={SDL} />);

    const toggle = screen.getByRole('button', { name: 'Show details for QUERY countries' });
    await user.click(toggle);

    const details = within(document.getElementById(toggle.getAttribute('aria-controls') ?? '')!);
    expect(details.getByText('= 20')).toBeInTheDocument();
    expect(details.getByText('= "Asia"')).toBeInTheDocument();
  });

  it('draws a scalar as a flat row: no member count and nothing to expand', () => {
    renderWithProviders(<GraphqlSchemaExplorer sdl={SDL} />);

    expect(screen.getByText('DateTime')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Show details for SCALAR DateTime/ })).not.toBeInTheDocument();
    expect(screen.queryByText('0 fields')).not.toBeInTheDocument();
  });

  it('expands an operation row to show its arguments and return type', async () => {
    const { user } = renderWithProviders(<GraphqlSchemaExplorer sdl={SDL} />);

    expect(screen.queryByText('Arguments')).not.toBeInTheDocument();

    const toggle = screen.getByRole('button', { name: 'Show details for QUERY country' });
    await user.click(toggle);

    // Scoped to the row's own panel: the collapsed Country type row also
    // mounts a `code` field.
    const details = within(document.getElementById(toggle.getAttribute('aria-controls') ?? '')!);
    expect(details.getByText('Arguments')).toBeInTheDocument();
    expect(details.getByText('code')).toBeInTheDocument();
    expect(details.getByText('ID!')).toBeInTheDocument();
    expect(details.getByText('Returns')).toBeInTheDocument();
    expect(details.getByText('Country')).toBeInTheDocument();
  });

  it('as a card, heads itself once with the view toggle on the same row, like REST\'s Resources card', () => {
    renderWithProviders(<GraphqlSchemaExplorer sdl={SDL} variant="card" />);

    expect(screen.getAllByText('Schema')).toHaveLength(1);
    expect(screen.getByRole('button', { name: 'SDL' })).toBeInTheDocument();
  });

  it('as a card, keeps its heading over the empty state', () => {
    renderWithProviders(<GraphqlSchemaExplorer variant="card" />);

    expect(screen.getByText('Schema')).toBeInTheDocument();
    expect(screen.getByText('Schema will show here')).toBeInTheDocument();
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
