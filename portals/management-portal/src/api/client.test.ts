import { afterEach, describe, expect, it } from 'vitest';

import {
  normalizeApiError,
  postGraphql,
  setApiHttpRequest,
} from './client';
import { ApiError } from './types/errors';

const fakeAxiosError = (status?: number, message = 'boom') => ({
  isAxiosError: true,
  message,
  response: status ? { status, data: { message } } : undefined,
});

afterEach(() => {
  setApiHttpRequest(undefined);
});

describe('normalizeApiError', () => {
  it('passes through existing ApiError instances', () => {
    const original = new ApiError('nope', 'FORBIDDEN', 403);
    expect(normalizeApiError(original)).toBe(original);
  });

  it.each([
    [401, 'UNAUTHORIZED'],
    [403, 'FORBIDDEN'],
    [404, 'NOT_FOUND'],
    [500, 'SERVER_ERROR'],
    [503, 'SERVER_ERROR'],
  ])('maps HTTP %s to %s', (status, code) => {
    const result = normalizeApiError(fakeAxiosError(status));
    expect(result.code).toBe(code);
    expect(result.status).toBe(status);
  });

  it('maps a response-less axios error to NETWORK_ERROR', () => {
    expect(normalizeApiError(fakeAxiosError()).code).toBe('NETWORK_ERROR');
  });

  it('maps non-axios values to UNKNOWN', () => {
    expect(normalizeApiError(new Error('x')).code).toBe('UNKNOWN');
  });
});

describe('postGraphql', () => {
  it('returns data on success', async () => {
    setApiHttpRequest(async () => ({ data: { data: { value: 1 } } }));
    await expect(postGraphql('query {}')).resolves.toEqual({ value: 1 });
  });

  it('throws when the GraphQL errors array is present', async () => {
    setApiHttpRequest(async () => ({
      data: { errors: [{ message: 'bad query' }] },
    }));
    await expect(postGraphql('query {}')).rejects.toMatchObject({
      message: 'bad query',
      code: 'UNKNOWN',
    });
  });

  it('throws when data is null and there are no errors', async () => {
    setApiHttpRequest(async () => ({ data: { data: null, errors: null } }));
    await expect(postGraphql('query {}')).rejects.toBeInstanceOf(ApiError);
  });
});
