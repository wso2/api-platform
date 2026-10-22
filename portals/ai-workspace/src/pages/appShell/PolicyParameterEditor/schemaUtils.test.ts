/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC. and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein is strictly forbidden, unless permitted by WSO2 in accordance with
 * the WSO2 Commercial License available at http://wso2.com/licenses.
 * For specific language governing the permissions and limitations under
 * this license, please see the license as well as any agreement you've
 * entered into with WSO2 governing the purchase of this software and any
 * associated services.
 */

import { describe, expect, it } from 'vitest';
import {
  formatOneOfGroups,
  hasCompleteOneOfGroup,
  schemaToTreeNodes,
  validateOneOf,
} from './schemaUtils';
import { ParameterSchema } from './types';

// Trimmed-down oauth2-generator parameters: token endpoint credentials OR a
// bearer token, never both.
const oauth2Schema: ParameterSchema = {
  type: 'object',
  properties: {
    tokenEndpoint: { type: 'string' },
    clientId: { type: 'string' },
    clientSecret: { type: 'string' },
    grantType: { type: 'string', default: 'client_credentials' },
    bearerToken: { type: 'string' },
  },
  oneOf: [
    { required: ['tokenEndpoint', 'clientId', 'clientSecret'] },
    { required: ['bearerToken'] },
  ],
};

const errorMap = (schema: ParameterSchema, values: Record<string, unknown>) =>
  Object.fromEntries(
    validateOneOf(schema, values).map((e) => [e.path, e.message])
  );

describe('validateOneOf', () => {
  it('accepts a complete credentials group', () => {
    expect(
      errorMap(oauth2Schema, {
        tokenEndpoint: 'https://idp/token',
        clientId: 'id',
        clientSecret: 'secret',
        grantType: 'client_credentials',
        bearerToken: '',
      })
    ).toEqual({});
  });

  it('accepts a bearer token alone, ignoring non-oneOf defaults', () => {
    expect(
      errorMap(oauth2Schema, {
        tokenEndpoint: '',
        clientId: '',
        clientSecret: '',
        grantType: 'client_credentials',
        bearerToken: 'abc',
      })
    ).toEqual({});
  });

  it('reports the missing fields of a partially filled group', () => {
    expect(
      errorMap(oauth2Schema, { tokenEndpoint: 'https://idp/token', clientId: '' })
    ).toEqual({
      clientId: 'This field is required',
      clientSecret: 'This field is required',
    });
  });

  it('reports conflicting groups on every filled field', () => {
    expect(
      errorMap(oauth2Schema, {
        tokenEndpoint: 'https://idp/token',
        clientId: 'id',
        bearerToken: 'abc',
      })
    ).toEqual({
      tokenEndpoint: 'Cannot be combined with bearerToken',
      clientId: 'Cannot be combined with bearerToken',
      bearerToken:
        'Cannot be combined with tokenEndpoint, clientId and clientSecret',
    });
  });

  it('asks for a group when nothing is configured', () => {
    const message =
      'Configure tokenEndpoint, clientId and clientSecret or bearerToken';
    expect(errorMap(oauth2Schema, {})).toEqual({
      tokenEndpoint: message,
      bearerToken: message,
    });
  });

  it('skips an empty optional nested object but validates a filled one', () => {
    const schema: ParameterSchema = {
      type: 'object',
      properties: {
        signingKey: {
          type: 'object',
          properties: {
            inline: { type: 'string' },
            path: { type: 'string' },
          },
          oneOf: [{ required: ['inline'] }, { required: ['path'] }],
        },
      },
    };
    expect(errorMap(schema, { signingKey: { inline: '', path: '' } })).toEqual(
      {}
    );
    expect(
      errorMap(schema, { signingKey: { inline: 'pem', path: '/key.pem' } })
    ).toEqual({
      'signingKey.inline': 'Cannot be combined with path',
      'signingKey.path': 'Cannot be combined with inline',
    });
  });

  it('treats empty arrays as not configured', () => {
    const schema: ParameterSchema = {
      type: 'object',
      properties: {
        allowedPhrases: { type: 'array', items: { type: 'string' } },
        deniedPhrases: { type: 'array', items: { type: 'string' } },
      },
      oneOf: [
        { required: ['allowedPhrases'] },
        { required: ['deniedPhrases'] },
      ],
    };
    expect(
      errorMap(schema, { allowedPhrases: ['a'], deniedPhrases: [] })
    ).toEqual({});
  });
});

describe('hasCompleteOneOfGroup', () => {
  it('is true for schemas without oneOf', () => {
    expect(hasCompleteOneOfGroup({ type: 'object', properties: {} }, {})).toBe(
      true
    );
  });

  it('requires one fully configured group', () => {
    expect(hasCompleteOneOfGroup(oauth2Schema, { tokenEndpoint: 'x' })).toBe(
      false
    );
    expect(hasCompleteOneOfGroup(oauth2Schema, { bearerToken: 'abc' })).toBe(
      true
    );
  });
});

describe('oneOf rendering helpers', () => {
  it('formats groups for messages', () => {
    expect(
      formatOneOfGroups([
        ['tokenEndpoint', 'clientId', 'clientSecret'],
        ['bearerToken'],
      ])
    ).toBe('tokenEndpoint, clientId and clientSecret or bearerToken');
  });

  it('marks oneOf fields so they are not labelled optional', () => {
    const nodes = Object.fromEntries(
      schemaToTreeNodes(oauth2Schema).map((n) => [n.name, n])
    );
    expect(nodes.bearerToken.isOneOfField).toBe(true);
    expect(nodes.clientId.isOneOfField).toBe(true);
    expect(nodes.grantType.isOneOfField).toBe(false);
    expect(nodes.bearerToken.isRequired).toBe(false);
  });
});
