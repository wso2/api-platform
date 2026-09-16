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

import { countNamedTypes, formatSdl, parseGraphQLSdl, summarizeSchema } from './graphqlSchema';

const SDL = `
  type Query {
    country(code: ID!): Country
    countries: [Country!]!
  }

  type Mutation {
    addReview(code: ID!, review: String!): Review
  }

  type Country {
    code: ID!
    name: String!
    "Deprecated in favour of continent"
    region: String @deprecated(reason: "Use continent instead")
    continent: Continent
  }

  type Continent {
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

  input CountryFilter {
    code: ID
  }
`;

describe('parseGraphQLSdl', () => {
  it('parses valid SDL into a schema', () => {
    const result = parseGraphQLSdl(SDL);

    expect('schema' in result).toBe(true);
  });

  it('reports a parse error instead of throwing, for invalid SDL', () => {
    const result = parseGraphQLSdl('type Query { broken');

    expect('error' in result).toBe(true);
    if ('error' in result) {
      expect(result.error).toEqual(expect.any(String));
      expect(result.error.length).toBeGreaterThan(0);
    }
  });
});

describe('formatSdl', () => {
  it('re-prints a schema in canonical formatting', () => {
    const parsed = parseGraphQLSdl(SDL);
    if (!('schema' in parsed)) throw new Error('expected the SDL to parse');

    const printed = formatSdl(parsed.schema);

    expect(printed).toContain('type Query');
    expect(printed).toContain('type Country');
  });
});

describe('summarizeSchema', () => {
  const parsed = parseGraphQLSdl(SDL);
  if (!('schema' in parsed)) throw new Error('expected the SDL to parse');
  const summary = summarizeSchema(parsed.schema);

  it('collects the Query and Mutation root fields, with their argument lists rendered', () => {
    expect(summary.queryFields.map((field) => field.name)).toEqual(['country', 'countries']);
    expect(summary.queryFields[0]).toMatchObject({ args: '(code: ID!)', type: 'Country' });
    expect(summary.queryFields[1]).toMatchObject({ args: '', type: '[Country!]!' });

    expect(summary.mutationFields.map((field) => field.name)).toEqual(['addReview']);
  });

  it('reports no subscription fields when the schema declares none', () => {
    expect(summary.subscriptionFields).toEqual([]);
  });

  it('excludes the root operation types and built-in scalars from the type list', () => {
    const typeNames = summary.types.map((type) => type.name);

    expect(typeNames).not.toContain('Query');
    expect(typeNames).not.toContain('Mutation');
    expect(typeNames).not.toContain('String');
    expect(typeNames).not.toContain('ID');
  });

  it('sorts every other named type alphabetically, with its own kind and fields', () => {
    const typeNames = summary.types.map((type) => type.name);
    expect(typeNames).toEqual(['Continent', 'Country', 'CountryFilter', 'Review', 'Status']);

    const country = summary.types.find((type) => type.name === 'Country');
    expect(country?.kind).toBe('OBJECT');
    expect(country?.fields?.map((field) => field.name)).toEqual([
      'code',
      'name',
      'region',
      'continent',
    ]);

    const filter = summary.types.find((type) => type.name === 'CountryFilter');
    expect(filter?.kind).toBe('INPUT_OBJECT');

    const status = summary.types.find((type) => type.name === 'Status');
    expect(status?.kind).toBe('ENUM');
    expect(status?.enumValues).toEqual(['ACTIVE', 'INACTIVE']);
  });

  it('flags a field carrying `@deprecated` and counts it in the schema-wide total', () => {
    const country = summary.types.find((type) => type.name === 'Country');
    const region = country?.fields?.find((field) => field.name === 'region');
    const continent = country?.fields?.find((field) => field.name === 'continent');

    expect(region?.deprecated).toBe(true);
    expect(continent?.deprecated).toBe(false);
    expect(summary.deprecatedFieldCount).toBe(1);
  });
});

describe('countNamedTypes', () => {
  it('counts every named type but not introspection meta types', () => {
    const parsed = parseGraphQLSdl(SDL);
    if (!('schema' in parsed)) throw new Error('expected the SDL to parse');

    // Query, Mutation, Country, Continent, Review, Status, CountryFilter (7),
    // plus ID and String from the SDL and Boolean pulled in by the always-
    // present `@skip`/`@include` directives' `if: Boolean!` argument (3) —
    // unlike `summarizeSchema`'s `types`, this count keeps root types and
    // scalars in, it only drops the leading-`__` introspection meta types.
    expect(countNamedTypes(parsed.schema)).toBe(10);
  });
});
