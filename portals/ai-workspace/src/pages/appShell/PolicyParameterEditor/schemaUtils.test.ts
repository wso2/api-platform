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
  getOneOfGroups,
  getOneOfSiblingPaths,
  hasCompleteOneOfGroup,
  initializeDefaultValues,
  schemaToTreeNodes,
  validateOneOf,
} from './schemaUtils';
import { ParameterSchema } from './types';

// Trimmed-down oauth2-generator; the oneOf matches the Policy Hub definition
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
    {
      required: ['tokenEndpoint', 'clientId', 'clientSecret'],
      not: { required: ['bearerToken'] },
    },
    {
      required: ['bearerToken'],
      not: {
        anyOf: [
          { required: ['tokenEndpoint'] },
          { required: ['clientId'] },
          { required: ['clientSecret'] },
        ],
      },
    },
  ],
};

// A oneOf whose second option is an object
const mtlsSchema: ParameterSchema = {
  type: 'object',
  properties: {
    apiKey: { type: 'string' },
    mtls: { type: 'object', properties: { cert: { type: 'string' }, key: { type: 'string' } } },
  },
  oneOf: [{ required: ['apiKey'] }, { required: ['mtls'] }],
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

  it('checks an optional nested object only once it holds a value', () => {
    const schema: ParameterSchema = {
      type: 'object',
      properties: {
        signingKey: {
          type: 'object',
          properties: { name: { type: 'string' }, inline: { type: 'string' }, path: { type: 'string' } },
          oneOf: [{ required: ['inline'] }, { required: ['path'] }],
        },
      },
    };
    expect(errorMap(schema, { signingKey: { name: '', inline: '', path: '' } })).toEqual({});
    // `name` keeps the object in the payload, so its oneOf applies
    expect(errorMap(schema, { signingKey: { name: 'key', inline: '', path: '' } })).toEqual({
      'signingKey.inline': 'Configure inline or path',
      'signingKey.path': 'Configure inline or path',
    });
    expect(errorMap(schema, { signingKey: { name: 'key', inline: 'pem' } })).toEqual({});
    expect(errorMap(schema, { signingKey: { inline: 'pem', path: '/key.pem' } })).toEqual({
      'signingKey.inline': 'Cannot be combined with path',
      'signingKey.path': 'Cannot be combined with inline',
    });
  });

  it('treats a default-initialized object member as not configured', () => {
    // initializeDefaultValues pre-fills the nested object with empty strings
    const values = initializeDefaultValues(mtlsSchema);
    expect(errorMap(mtlsSchema, values)).toEqual({
      apiKey: 'Configure apiKey or mtls',
      mtls: 'Configure apiKey or mtls',
    });
    expect(hasCompleteOneOfGroup(mtlsSchema, values)).toBe(false);
    expect(errorMap(mtlsSchema, { ...values, apiKey: 'k' })).toEqual({});
    expect(hasCompleteOneOfGroup(mtlsSchema, { ...values, mtls: { cert: 'c', key: '' } })).toBe(true);
  });
});

// Policy Hub shapes with overlapping options kept apart by `not`
// (semantic-prompt-guard 1.0, token-based-ratelimit 1.0)
const phrasesSchema: ParameterSchema = {
  type: 'object',
  properties: {
    allowedPhrases: { type: 'array', items: { type: 'string' } },
    deniedPhrases: { type: 'array', items: { type: 'string' } },
  },
  oneOf: [
    { required: ['allowedPhrases'], not: { required: ['deniedPhrases'] } },
    { required: ['deniedPhrases'], not: { required: ['allowedPhrases'] } },
    { required: ['allowedPhrases', 'deniedPhrases'] },
  ],
};

const limits = ['promptTokenLimits', 'completionTokenLimits', 'totalTokenLimits'];
const tokenLimitsSchema: ParameterSchema = {
  type: 'object',
  properties: Object.fromEntries(
    limits.map((l) => [l, { type: 'array', items: { type: 'object', properties: { count: { type: 'integer' } } } }])
  ) as ParameterSchema['properties'],
  oneOf: [
    ...limits.map((l) => ({
      required: [l],
      not: { anyOf: limits.filter((o) => o !== l).map((o) => ({ required: [o] })) },
    })),
    { required: [limits[0], limits[1]], not: { required: [limits[2]] } },
    { required: [limits[0], limits[2]], not: { required: [limits[1]] } },
    { required: [limits[1], limits[2]], not: { required: [limits[0]] } },
    { required: limits },
  ],
};

describe('validateOneOf with overlapping options kept apart by `not`', () => {
  it('accepts each valid semantic-prompt-guard configuration', () => {
    expect(errorMap(phrasesSchema, { allowedPhrases: ['a'], deniedPhrases: [] })).toEqual({});
    expect(errorMap(phrasesSchema, { deniedPhrases: ['d'] })).toEqual({});
    expect(errorMap(phrasesSchema, { allowedPhrases: ['a'], deniedPhrases: ['d'] })).toEqual({});
  });

  it('still requires one of the semantic-prompt-guard options', () => {
    expect(Object.keys(errorMap(phrasesSchema, {})).sort()).toEqual(['allowedPhrases', 'deniedPhrases']);
  });

  it('accepts every non-empty combination of token limits', () => {
    const subsets = [[0], [1], [2], [0, 1], [0, 2], [1, 2], [0, 1, 2]];
    subsets.forEach((subset) => {
      const values = Object.fromEntries(subset.map((i) => [limits[i], [{ count: 1 }]]));
      expect(errorMap(tokenLimitsSchema, values)).toEqual({});
    });
  });

  it('still requires at least one token limit', () => {
    expect(Object.keys(errorMap(tokenLimitsSchema, {})).length).toBeGreaterThan(0);
  });

  it("names the field an option's `not` rules out when no other option claims it", () => {
    const schema: ParameterSchema = {
      type: 'object',
      properties: { a: { type: 'string' }, b: { type: 'string' }, c: { type: 'string' } },
      oneOf: [{ required: ['b', 'c'], not: { required: ['a'] } }],
    };
    expect(errorMap(schema, { a: 'x', b: 'y', c: 'z' })).toEqual({
      a: 'Cannot be combined with b and c',
      b: 'Cannot be combined with a',
      c: 'Cannot be combined with a',
    });
  });

  it('reports an unsatisfiable option without a dangling message', () => {
    const schema: ParameterSchema = {
      type: 'object',
      properties: { a: { type: 'string' } },
      oneOf: [{ required: ['a'], not: { required: ['a'] } }],
    };
    expect(errorMap(schema, { a: 'x' })).toEqual({
      a: 'This combination of fields is not allowed',
    });
  });

  it('keeps reporting a real conflict between options without `not`', () => {
    const schema: ParameterSchema = {
      type: 'object',
      properties: { text: { type: 'string' }, messages: { type: 'array', items: { type: 'string' } } },
      oneOf: [{ required: ['text'] }, { required: ['messages'] }],
    };
    expect(errorMap(schema, { text: 't', messages: ['m'] })).toEqual({
      text: 'Cannot be combined with messages',
      messages: 'Cannot be combined with text',
    });
  });
});

describe('oneOf shapes the editor does not understand', () => {
  // Options told apart by a const value, which the editor does not evaluate
  const discriminated = {
    type: 'object',
    properties: {
      mode: { type: 'string', enum: ['jwt', 'basic'] },
      key: { type: 'string' },
      user: { type: 'string' },
    },
    oneOf: [
      { properties: { mode: { const: 'jwt' } }, required: ['mode', 'key'] },
      { properties: { mode: { const: 'basic' } }, required: ['mode', 'user'] },
    ],
  } as unknown as ParameterSchema;

  it('skips validation instead of guessing', () => {
    expect(errorMap(discriminated, { mode: 'jwt', key: 'k', user: 'u' })).toEqual({});
    expect(errorMap(discriminated, {})).toEqual({});
    expect(hasCompleteOneOfGroup(discriminated, {})).toBe(true);
  });

  it('treats the fields as ordinary optional fields (no banner, no oneOf labels)', () => {
    expect(getOneOfGroups(discriminated)).toEqual([]);
    expect(getOneOfSiblingPaths(discriminated, 'key')).toEqual([]);
    expect(schemaToTreeNodes(discriminated).some((n) => n.isOneOfField)).toBe(false);
  });

  it('skips a oneOf with an option that names no required field', () => {
    const schema = {
      type: 'object',
      properties: { a: { type: 'string' }, b: { type: 'string' } },
      oneOf: [{ required: ['a'] }, { not: { required: ['a'] } }],
    } as unknown as ParameterSchema;
    expect(errorMap(schema, {})).toEqual({});
    expect(getOneOfGroups(schema)).toEqual([]);
  });

  it('still understands required / anyOf / not nested in `not`, plus titles', () => {
    const schema = {
      type: 'object',
      properties: { a: { type: 'string' }, b: { type: 'string' } },
      oneOf: [
        { title: 'A only', required: ['a'], not: { anyOf: [{ required: ['b'] }] } },
        { title: 'B only', required: ['b'], not: { not: { not: { required: ['a'] } } } },
      ],
    } as unknown as ParameterSchema;
    expect(getOneOfGroups(schema)).toEqual([['a'], ['b']]);
    expect(errorMap(schema, { a: 'x' })).toEqual({});
  });
});

describe('hasCompleteOneOfGroup', () => {
  it('is true without a oneOf, else needs one fully configured group', () => {
    expect(hasCompleteOneOfGroup({ type: 'object', properties: {} }, {})).toBe(true);
    expect(hasCompleteOneOfGroup(oauth2Schema, { tokenEndpoint: 'x' })).toBe(false);
    expect(hasCompleteOneOfGroup(oauth2Schema, { bearerToken: 'abc' })).toBe(true);
  });
});

describe('oneOf rendering helpers', () => {
  it('finds the oneOf siblings of a field, including in array items', () => {
    expect(getOneOfSiblingPaths(oauth2Schema, 'clientId')).toEqual([
      'tokenEndpoint',
      'clientId',
      'clientSecret',
      'bearerToken',
    ]);
    expect(getOneOfSiblingPaths(oauth2Schema, 'grantType')).toEqual([]);

    const nested: ParameterSchema = {
      type: 'object',
      properties: {
        keys: {
          type: 'array',
          items: {
            type: 'object',
            properties: { inline: { type: 'string' }, path: { type: 'string' } },
            oneOf: [{ required: ['inline'] }, { required: ['path'] }],
          },
        },
      },
    };
    expect(getOneOfSiblingPaths(nested, 'keys.2.path')).toEqual([
      'keys.2.inline',
      'keys.2.path',
    ]);
    expect(getOneOfSiblingPaths(nested, 'missing.path')).toEqual([]);
  });

  it('finds the oneOf siblings of an object option when a field inside it changes', () => {
    expect(getOneOfSiblingPaths(mtlsSchema, 'mtls.cert')).toEqual(['apiKey', 'mtls']);
  });

  it('formats groups for messages', () => {
    expect(formatOneOfGroups([['a', 'b', 'c'], ['d']])).toBe('a, b and c or d');
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
