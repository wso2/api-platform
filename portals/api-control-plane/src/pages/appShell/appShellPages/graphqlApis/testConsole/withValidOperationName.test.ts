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

import { withValidOperationName } from './withValidOperationName';

describe('withValidOperationName', () => {
  // The bug this guards: GraphiQL keeps the starter tab's "Schema" name after
  // the query is replaced with an anonymous one, and the gateway rejects it.
  it('drops a stale operationName when the query is anonymous', () => {
    const result = withValidOperationName({ query: '{ hello }', operationName: 'Schema' });

    expect(result).toEqual({ query: '{ hello }' });
    expect(result).not.toHaveProperty('operationName');
  });

  it('drops an operationName that names no operation in the document', () => {
    expect(
      withValidOperationName({ query: 'query A { a } query B { b }', operationName: 'Schema' }),
    ).toEqual({ query: 'query A { a } query B { b }' });
  });

  it('keeps an operationName that names an operation in the document', () => {
    const params = { query: 'query A { a } query B { b }', operationName: 'B' };

    expect(withValidOperationName(params)).toBe(params);
  });

  it('keeps variables when dropping the operationName', () => {
    expect(
      withValidOperationName({
        query: 'query ($code: ID!) { country(code: $code) }',
        operationName: 'Schema',
        variables: { code: 'LK' },
      }),
    ).toEqual({ query: 'query ($code: ID!) { country(code: $code) }', variables: { code: 'LK' } });
  });

  it('ignores fragment names when matching', () => {
    expect(
      withValidOperationName({ query: '{ ...F } fragment F on Query { a }', operationName: 'F' }),
    ).toEqual({ query: '{ ...F } fragment F on Query { a }' });
  });

  it('leaves params untouched when there is no operationName', () => {
    const params = { query: '{ hello }', operationName: null };

    expect(withValidOperationName(params)).toBe(params);
  });

  // The server should report the real syntax error, not a fetcher-side one.
  it('leaves an unparseable document untouched', () => {
    const params = { query: '{ hello ', operationName: 'Schema' };

    expect(withValidOperationName(params)).toBe(params);
  });
});
