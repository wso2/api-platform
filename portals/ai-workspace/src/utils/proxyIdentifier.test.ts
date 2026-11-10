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
import {
  MAX_PROXY_ID_LENGTH,
  AGENT_PROXY_ID_TAKEN_CODE,
  MCP_PROXY_ID_TAKEN_CODE,
  createWithUniqueId,
  buildProxyContext,
  nextProxyIdAfterFailure,
  proxyIdWithCounter,
  resolveUniqueProxyId,
  toProxyId,
} from './proxyIdentifier';

const taken = (...ids: string[]) => async (c: string) => ids.includes(c);
const clash = { code: 'LLM_PROXY_EXISTS' };

describe('toProxyId', () => {
  it('narrows a display name to an identifier', () => {
    expect(toProxyId('OpenAI Proxy')).toBe('openai-proxy');
    expect(toProxyId('  My / Proxy!  ')).toBe('my-proxy');
    expect(toProxyId('!!!')).toBe('');
  });
});

describe('proxyIdWithCounter', () => {
  it('leaves the plain identifier alone and suffixes the rest', () => {
    expect(proxyIdWithCounter('openai-proxy', 1)).toBe('openai-proxy');
    expect(proxyIdWithCounter('openai-proxy', 2)).toBe('openai-proxy-2');
  });

  it('shortens the base so the suffix fits the length limit', () => {
    const base = 'a'.repeat(MAX_PROXY_ID_LENGTH);
    const id = proxyIdWithCounter(base, 12);
    expect(id).toHaveLength(MAX_PROXY_ID_LENGTH);
    expect(id.endsWith('-12')).toBe(true);
  });

  it('does not leave a dangling hyphen where the base was cut', () => {
    const base = `${'a'.repeat(37)}-bcd`; // cut falls right after the hyphen
    expect(proxyIdWithCounter(base, 2)).not.toContain('--');
  });
});

describe('resolveUniqueProxyId', () => {
  it('uses the plain identifier when it is free', async () => {
    expect(await resolveUniqueProxyId('OpenAI Proxy', taken())).toEqual({ id: 'openai-proxy', n: 1 });
  });

  it('appends a counter when the identifier is taken', async () => {
    expect(await resolveUniqueProxyId('OpenAI Proxy', taken('openai-proxy'))).toEqual({
      id: 'openai-proxy-2',
      n: 2,
    });
    expect(
      await resolveUniqueProxyId('OpenAI Proxy', taken('openai-proxy', 'openai-proxy-2'))
    ).toEqual({ id: 'openai-proxy-3', n: 3 });
  });

  it('does not ask about counters before startAt', async () => {
    const asked: string[] = [];
    const choice = await resolveUniqueProxyId(
      'OpenAI Proxy',
      async (c) => {
        asked.push(c);
        return c === 'openai-proxy-3';
      },
      { startAt: 3 }
    );
    expect(choice).toEqual({ id: 'openai-proxy-4', n: 4 });
    expect(asked).toEqual(['openai-proxy-3', 'openai-proxy-4']);
  });

  it('keeps the counter apart from digits in the name', async () => {
    expect((await resolveUniqueProxyId('Proxy1', taken('proxy1'))).id).toBe('proxy1-2');
  });

  it('returns an empty choice for a name with no usable characters', async () => {
    expect(await resolveUniqueProxyId('!!!', taken())).toEqual({ id: '', n: 0 });
  });

  it('gives up rather than looping forever', async () => {
    await expect(resolveUniqueProxyId('a', async () => true)).rejects.toThrow();
  });
});

describe('nextProxyIdAfterFailure', () => {
  const base = { displayName: 'OpenAI Proxy', failedCounter: 1, attempt: 0 };

  it('picks the next free identifier after an identifier clash', async () => {
    const next = await nextProxyIdAfterFailure({
      ...base,
      error: clash,
      exists: taken('openai-proxy-2'),
    });
    expect(next).toEqual({ id: 'openai-proxy-3', n: 3 });
  });

  it('never re-asks about the identifier that just failed', async () => {
    const asked: string[] = [];
    await nextProxyIdAfterFailure({
      ...base,
      error: clash,
      exists: async (c) => {
        asked.push(c);
        return false;
      },
    });
    expect(asked).toEqual(['openai-proxy-2']);
  });

  it('does not retry for any other failure', async () => {
    expect(
      await nextProxyIdAfterFailure({ ...base, error: { code: 'VALIDATION_FAILED' }, exists: taken() })
    ).toBeNull();
    expect(await nextProxyIdAfterFailure({ ...base, error: new Error('network'), exists: taken() })).toBeNull();
  });

  it('stops once the retries are used up', async () => {
    expect(await nextProxyIdAfterFailure({ ...base, attempt: 2, error: clash, exists: taken() })).toBeNull();
  });

  it('reports a failed lookup and falls back to the original error', async () => {
    const seen: unknown[] = [];
    const next = await nextProxyIdAfterFailure({
      ...base,
      error: clash,
      exists: async () => {
        throw new Error('forbidden');
      },
      onLookupError: (e) => seen.push(e),
    });
    expect(next).toBeNull();
    expect(seen).toHaveLength(1);
  });
});

describe('buildProxyContext', () => {
  it('prefixes the project when there is one', () => {
    expect(buildProxyContext('default', 'openai-proxy')).toBe('/default/openai-proxy');
    expect(buildProxyContext('', 'openai-proxy')).toBe('/openai-proxy');
  });
});

describe('createWithUniqueId', () => {
  const mcpClash = { code: MCP_PROXY_ID_TAKEN_CODE };
  const run = (create: (c: { id: string; n: number }) => Promise<string>, exists = taken()) =>
    createWithUniqueId({
      displayName: 'Weather MCP',
      create,
      exists,
      takenCode: MCP_PROXY_ID_TAKEN_CODE,
    });

  it('creates under the plain identifier with no lookup when it is free', async () => {
    const asked: string[] = [];
    const out = await createWithUniqueId({
      displayName: 'Weather MCP',
      create: async (c) => c.id,
      exists: async (c) => {
        asked.push(c);
        return false;
      },
      takenCode: MCP_PROXY_ID_TAKEN_CODE,
    });
    expect(out.result).toBe('weather-mcp');
    expect(asked).toEqual([]);
  });

  it('retries under the next free identifier after a clash', async () => {
    const tried: string[] = [];
    const out = await run(async (c) => {
      tried.push(c.id);
      if (c.id === 'weather-mcp') throw mcpClash;
      return c.id;
    });
    expect(out.choice).toEqual({ id: 'weather-mcp-2', n: 2 });
    expect(tried).toEqual(['weather-mcp', 'weather-mcp-2']);
  });

  it('skips identifiers the lookup reports as taken', async () => {
    const out = await run(
      async (c) => {
        if (c.n < 3) throw mcpClash;
        return c.id;
      },
      taken('weather-mcp-2')
    );
    expect(out.choice.id).toBe('weather-mcp-3');
  });

  it('rethrows any other failure without retrying', async () => {
    let calls = 0;
    await expect(
      run(async () => {
        calls += 1;
        throw { code: 'VALIDATION_FAILED' };
      })
    ).rejects.toEqual({ code: 'VALIDATION_FAILED' });
    expect(calls).toBe(1);
  });

  it('gives up after the retries are used up and rethrows the clash', async () => {
    let calls = 0;
    await expect(
      run(async () => {
        calls += 1;
        throw mcpClash;
      })
    ).rejects.toEqual(mcpClash);
    expect(calls).toBe(3);
  });

  it("ignores the other resource's clash code", async () => {
    let calls = 0;
    await expect(
      run(async () => {
        calls += 1;
        throw { code: 'LLM_PROXY_EXISTS' };
      })
    ).rejects.toBeDefined();
    expect(calls).toBe(1);
  });
});

describe('createWithUniqueId for agent proxies', () => {
  it('retries on the agent proxy clash code and ignores the MCP one', async () => {
    const tried: string[] = [];
    const out = await createWithUniqueId({
      displayName: 'Travel Agent',
      takenCode: AGENT_PROXY_ID_TAKEN_CODE,
      exists: taken(),
      create: async (c) => {
        tried.push(c.id);
        if (c.n === 1) throw { code: AGENT_PROXY_ID_TAKEN_CODE };
        return c.id;
      },
    });
    expect(tried).toEqual(['travel-agent', 'travel-agent-2']);
    expect(out.choice.id).toBe('travel-agent-2');

    let calls = 0;
    await expect(
      createWithUniqueId({
        displayName: 'Travel Agent',
        takenCode: AGENT_PROXY_ID_TAKEN_CODE,
        exists: taken(),
        create: async () => {
          calls += 1;
          throw { code: MCP_PROXY_ID_TAKEN_CODE };
        },
      })
    ).rejects.toBeDefined();
    expect(calls).toBe(1);
  });
});
