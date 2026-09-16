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

import type { GraphqlApiCreationFormState } from '../types';
import { toCreateGraphQLApiBody } from './createGraphqlApiBody';

const baseState: GraphqlApiCreationFormState = {
  id: 'countries-api',
  displayName: 'Countries API',
  version: '1.0.0',
  context: '/countries-api/v1.0.0',
  endpointUrl: 'https://backend.example.com/graphql',
  schemaSource: 'introspection',
};

const scope = { projectId: 'retail' };

/**
 * The backend rejects a create request carrying more than one schema-source
 * field as a structural mismatch (400 VALIDATION_FAILED) — a real bug this
 * mapping used to produce by sending both `sdl` and `sdlUrl` whenever both
 * happened to be populated on form state, regardless of the declared
 * `schemaSource`. These assertions are the regression guard for that fix:
 * exactly one schema field (or none, for `introspection`) ever appears.
 */
describe('toCreateGraphQLApiBody — schema-source field selection', () => {
  it('sends only `sdl` for an inline schema, never `sdlUrl`', () => {
    const body = toCreateGraphQLApiBody(
      { ...baseState, schemaSource: 'inline', sdl: 'type Query { hello: String }', sdlUrl: 'https://leftover.example.com/schema.graphql' },
      scope,
    );

    expect(body.metadata).toMatchObject({ schemaSource: 'inline', sdl: 'type Query { hello: String }' });
    expect(body.metadata).not.toHaveProperty('sdlUrl');
    expect(body).not.toHaveProperty('sdlFile');
  });

  it('sends only `sdlUrl` for a URL-sourced schema, never `sdl`', () => {
    const body = toCreateGraphQLApiBody(
      {
        ...baseState,
        schemaSource: 'url',
        sdlUrl: 'https://raw.githubusercontent.com/example/schema.graphql',
        // The already-resolved preview text — must never be forwarded once a
        // URL is the declared source, or the backend rejects both at once.
        sdl: 'type Query { hello: String }',
      },
      scope,
    );

    expect(body.metadata).toMatchObject({
      schemaSource: 'url',
      sdlUrl: 'https://raw.githubusercontent.com/example/schema.graphql',
    });
    expect(body.metadata).not.toHaveProperty('sdl');
  });

  it('sends neither `sdl` nor `sdlUrl` for introspection, only the endpoint', () => {
    const body = toCreateGraphQLApiBody(
      { ...baseState, schemaSource: 'introspection', sdl: 'type Query { hello: String }' },
      scope,
    );

    expect(body.metadata).not.toHaveProperty('sdl');
    expect(body.metadata).not.toHaveProperty('sdlUrl');
    expect(body.metadata.upstream).toEqual({ main: { url: 'https://backend.example.com/graphql' } });
  });

  it('attaches `sdlFile` at the top level only when the source is `file`', () => {
    const file = new File(['type Query { hello: String }'], 'schema.graphql');

    const fileBody = toCreateGraphQLApiBody(
      { ...baseState, schemaSource: 'file', sdlFile: file },
      scope,
    );
    expect(fileBody.sdlFile).toBe(file);
    expect(fileBody.metadata).not.toHaveProperty('sdl');
    expect(fileBody.metadata).not.toHaveProperty('sdlUrl');

    // A leftover `sdlFile` on form state (e.g. from switching away from the
    // Upload tab) must not leak into a request for a different source.
    const introspectionBody = toCreateGraphQLApiBody(
      { ...baseState, schemaSource: 'introspection', sdlFile: file },
      scope,
    );
    expect(introspectionBody).not.toHaveProperty('sdlFile');
  });
});

describe('toCreateGraphQLApiBody — identity and optional fields', () => {
  it('trims text fields and drops a blank description rather than sending an empty string', () => {
    const body = toCreateGraphQLApiBody(
      { ...baseState, displayName: '  Countries API  ', description: '   ' },
      scope,
    );

    expect(body.metadata.displayName).toBe('Countries API');
    expect(body.metadata).not.toHaveProperty('description');
  });

  it('keeps a non-blank description, trimmed', () => {
    const body = toCreateGraphQLApiBody(
      { ...baseState, description: '  Public schema for country data.  ' },
      scope,
    );

    expect(body.metadata.description).toBe('Public schema for country data.');
  });

  it('omits `id` when blank, letting the server generate the handle', () => {
    const body = toCreateGraphQLApiBody({ ...baseState, id: '   ' }, scope);

    expect(body.metadata).not.toHaveProperty('id');
  });

  it('stamps the kind and project scope the server expects', () => {
    const body = toCreateGraphQLApiBody(baseState, { projectId: 'retail' });

    expect(body.metadata.kind).toBe('GraphQLApi');
    expect(body.metadata.projectId).toBe('retail');
  });
});
