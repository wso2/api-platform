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

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ARRAY_MARKER, collectWriteOnlyPaths, promoteWriteOnlyParams } from './policySecrets';

const createSecret = vi.fn();

vi.mock('../apis/secretApis', () => ({
  createSecret: (...args: unknown[]) => createSecret(...args),
  generateSecretHandle: () => 'generated-handle',
  buildSecretPlaceholder: (name: string) => `{{ secret "${name}" }}`,
}));

const describedAs = { artifactName: 'my-provider', policyName: 'aws-authentication' };

beforeEach(() => {
  createSecret.mockReset();
  createSecret.mockImplementation(async ({ id }: { id: string }) => ({ id }));
});

/**
 * What the form is allowed to turn into a secret.
 *
 * The consequence of finding too little is a credential saved as typed, which
 * is the documented fallback; the consequence of finding too much is a
 * non-credential replaced by a reference the gateway then resolves to the
 * wrong thing. Both directions are pinned.
 */
describe('collectWriteOnlyPaths', () => {
  it('finds declarations in a JSON Schema object node', () => {
    const paths = collectWriteOnlyPaths({
      parameters: {
        type: 'object',
        properties: {
          region: { type: 'string' },
          awsSecretAccessKey: { type: 'string', writeOnly: true },
        },
      },
    });

    expect(paths).toEqual([['awsSecretAccessKey']]);
  });

  it('finds a declaration nested inside an object', () => {
    const paths = collectWriteOnlyPaths({
      parameters: {
        type: 'object',
        properties: {
          awsCredentials: {
            type: 'object',
            properties: {
              authenticationType: { type: 'string' },
              awsSecretAccessKey: { type: 'string', writeOnly: true },
            },
          },
        },
      },
    });

    expect(paths).toEqual([['awsCredentials', 'awsSecretAccessKey']]);
  });

  it('accepts a bare map of name to definition', () => {
    const paths = collectWriteOnlyPaths({
      parameters: { password: { type: 'string', writeOnly: true }, username: { type: 'string' } },
    });

    expect(paths).toEqual([['password']]);
  });

  it('accepts a list of named entries', () => {
    const paths = collectWriteOnlyPaths({
      parameters: [
        { name: 'clientSecret', type: 'string', writeOnly: true },
        { name: 'clientId', type: 'string' },
      ],
    });

    expect(paths).toEqual([['clientSecret']]);
  });

  it('expands a declaration on the root into one path per parameter', () => {
    const paths = collectWriteOnlyPaths({
      parameters: {
        type: 'object',
        writeOnly: true,
        properties: { username: { type: 'string' }, password: { type: 'string' } },
      },
    });

    expect(paths.sort()).toEqual([['password'], ['username']]);
  });

  // The documented fallback: an older gateway, or a definition predating the
  // annotation, declares nothing and everything is saved as typed.
  it('finds nothing when no parameter is declared write-only', () => {
    expect(
      collectWriteOnlyPaths({
        parameters: { type: 'object', properties: { region: { type: 'string' } } },
      }),
    ).toEqual([]);
  });

  it('finds nothing for a definition it cannot read', () => {
    expect(collectWriteOnlyPaths(undefined)).toEqual([]);
    expect(collectWriteOnlyPaths({})).toEqual([]);
    expect(collectWriteOnlyPaths({ parameters: 'not-a-schema' })).toEqual([]);
  });
});

describe('promoteWriteOnlyParams', () => {
  it('creates a secret and substitutes the reference', async () => {
    const promoted = await promoteWriteOnlyParams(
      { region: 'us-east-1', awsSecretAccessKey: 'AKIAEXAMPLE' },
      [['awsSecretAccessKey']],
      describedAs,
    );

    expect(createSecret).toHaveBeenCalledTimes(1);
    expect(createSecret).toHaveBeenCalledWith(
      expect.objectContaining({ value: 'AKIAEXAMPLE', type: 'GENERIC' }),
    );
    expect(promoted.awsSecretAccessKey).toBe('{{ secret "generated-handle" }}');
    expect(promoted.region).toBe('us-east-1');
  });

  it('substitutes a nested parameter without disturbing its siblings', async () => {
    const promoted = await promoteWriteOnlyParams(
      { awsCredentials: { authenticationType: 'iam-user-access-key', awsSecretAccessKey: 'shh' } },
      [['awsCredentials', 'awsSecretAccessKey']],
      describedAs,
    );

    expect(promoted.awsCredentials).toEqual({
      authenticationType: 'iam-user-access-key',
      awsSecretAccessKey: '{{ secret "generated-handle" }}',
    });
  });

  // Saving as typed is the documented behaviour when nothing is declared, so
  // no secret may be created on that path.
  it('creates nothing when the definition declared no write-only parameter', async () => {
    const params = { awsSecretAccessKey: 'AKIAEXAMPLE' };

    const promoted = await promoteWriteOnlyParams(params, [], describedAs);

    expect(createSecret).not.toHaveBeenCalled();
    expect(promoted).toBe(params);
  });

  it('leaves a value that is already a reference alone', async () => {
    const promoted = await promoteWriteOnlyParams(
      { awsSecretAccessKey: '{{ secret "existing-handle" }}' },
      [['awsSecretAccessKey']],
      describedAs,
    );

    expect(createSecret).not.toHaveBeenCalled();
    expect(promoted.awsSecretAccessKey).toBe('{{ secret "existing-handle" }}');
  });

  // The empty string is what a read returns for a withheld credential, so
  // promoting it would store a secret holding nothing and overwrite the real
  // one on the way through.
  it('leaves an empty value alone', async () => {
    const promoted = await promoteWriteOnlyParams(
      { awsSecretAccessKey: '' },
      [['awsSecretAccessKey']],
      describedAs,
    );

    expect(createSecret).not.toHaveBeenCalled();
    expect(promoted.awsSecretAccessKey).toBe('');
  });

  it('leaves a parameter the form never set alone', async () => {
    const promoted = await promoteWriteOnlyParams({ region: 'us-east-1' }, [['awsSessionToken']], describedAs);

    expect(createSecret).not.toHaveBeenCalled();
    expect(promoted).toEqual({ region: 'us-east-1' });
  });

  it('does not promote inside an array, where elements have no identity', async () => {
    const promoted = await promoteWriteOnlyParams(
      { backends: [{ apiKey: 'one' }] },
      [['backends', ARRAY_MARKER, 'apiKey']],
      describedAs,
    );

    expect(createSecret).not.toHaveBeenCalled();
    expect(promoted.backends).toEqual([{ apiKey: 'one' }]);
  });

  it('does not modify the parameters it was given', async () => {
    const params = { awsSecretAccessKey: 'AKIAEXAMPLE' };

    await promoteWriteOnlyParams(params, [['awsSecretAccessKey']], describedAs);

    expect(params.awsSecretAccessKey).toBe('AKIAEXAMPLE');
  });

  // Saving the credential as typed because the secret could not be stored
  // would be the one outcome this is meant to prevent.
  it('raises a failure to create the secret rather than saving as typed', async () => {
    createSecret.mockRejectedValueOnce(new Error('secret store unavailable'));

    await expect(
      promoteWriteOnlyParams({ awsSecretAccessKey: 'AKIAEXAMPLE' }, [['awsSecretAccessKey']], describedAs),
    ).rejects.toThrow('secret store unavailable');
  });
});
