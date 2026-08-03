import { describe, expect, it } from 'vitest';

import {
  defaultForSchema,
  getByPath,
  initValues,
  type ParameterSchema,
  setByPath,
  topLevelRequiredMissing,
} from './policySchema';

describe('policySchema path helpers', () => {
  const root = {
    name: 'cors',
    headers: [
      { name: 'X-A', value: '1' },
      { name: 'X-B', value: '2' },
    ],
    nested: { flag: true },
  };

  it('reads object, array-index, and nested paths', () => {
    expect(getByPath(root, 'name')).toBe('cors');
    expect(getByPath(root, 'headers.1.name')).toBe('X-B');
    expect(getByPath(root, 'nested.flag')).toBe(true);
  });

  it('returns undefined for empty path (root) only via missing segments', () => {
    expect(getByPath(root, 'missing')).toBeUndefined();
    expect(getByPath(root, 'headers.5.name')).toBeUndefined();
    expect(getByPath(undefined, 'x')).toBeUndefined();
  });

  it('sets a nested object value immutably', () => {
    const next = setByPath(root, 'nested.flag', false);
    expect(getByPath(next, 'nested.flag')).toBe(false);
    expect(root.nested.flag).toBe(true); // original untouched
  });

  it('sets an array-index value, creating arrays as needed', () => {
    const next = setByPath(root, 'headers.0.value', '9');
    expect(getByPath(next, 'headers.0.value')).toBe('9');
    expect(getByPath(next, 'headers.1.name')).toBe('X-B'); // sibling kept
    expect(root.headers[0].value).toBe('1'); // original untouched
  });

  it('creates intermediate structures from an empty root', () => {
    const next = setByPath({}, 'a.0.b', 'deep');
    expect(getByPath(next, 'a.0.b')).toBe('deep');
    expect(Array.isArray((next as { a: unknown }).a)).toBe(true);
  });
});

describe('defaultForSchema', () => {
  it('uses an explicit default when present', () => {
    expect(defaultForSchema({ type: 'string', default: 'hi' })).toBe('hi');
  });

  it('defaults objects to a shape with defaulted children only', () => {
    const schema: ParameterSchema = {
      type: 'object',
      properties: {
        flag: { type: 'boolean' },
        name: { type: 'string' }, // no default -> omitted
        items: { type: 'array' },
      },
    };
    expect(defaultForSchema(schema)).toEqual({ flag: false, items: [] });
  });

  it('defaults arrays to [] and booleans to false', () => {
    expect(defaultForSchema({ type: 'array' })).toEqual([]);
    expect(defaultForSchema({ type: 'boolean' })).toBe(false);
    expect(defaultForSchema({ type: 'string' })).toBeUndefined();
  });
});

describe('initValues', () => {
  const schema: ParameterSchema = {
    type: 'object',
    properties: { flag: { type: 'boolean' }, name: { type: 'string' } },
  };

  it('returns schema defaults when there are no existing values', () => {
    expect(initValues(schema)).toEqual({ flag: false });
  });

  it('overlays existing values on top of defaults in edit mode', () => {
    expect(initValues(schema, { name: 'kept' })).toEqual({
      flag: false,
      name: 'kept',
    });
  });
});

describe('topLevelRequiredMissing', () => {
  const schema: ParameterSchema = {
    type: 'object',
    properties: { a: { type: 'string' }, b: { type: 'string' } },
    required: ['a'],
  };

  it('is true when a required value is empty/missing', () => {
    expect(topLevelRequiredMissing(schema, {})).toBe(true);
    expect(topLevelRequiredMissing(schema, { a: '' })).toBe(true);
  });

  it('is false when all required values are present', () => {
    expect(topLevelRequiredMissing(schema, { a: 'x' })).toBe(false);
  });

  it('is false when there are no required fields', () => {
    expect(topLevelRequiredMissing({ type: 'object' }, {})).toBe(false);
  });
});
