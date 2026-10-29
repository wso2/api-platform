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

import { describe, expect, it } from 'vitest';

import {
  capabilityRows,
  definitionToJson,
  setAllCapabilities,
  toggleCapability,
  type CapabilityItem,
} from './mcpCapabilitySelection';

const search: CapabilityItem = { name: 'search', description: 'Find trips' };
const book: CapabilityItem = { name: 'book', description: 'Book a trip' };
const guide: CapabilityItem = { uri: 'file:///guide.md', name: 'guide' };

describe('capabilityRows', () => {
  it('lists the proxy’s own items in its order, each selected only if the definition holds it', () => {
    const rows = capabilityRows('tools', [search, book], { tools: [book] });

    expect(rows.map(({ key, selected, onProxy }) => ({ key, selected, onProxy }))).toEqual([
      { key: 'search', selected: false, onProxy: true },
      { key: 'book', selected: true, onProxy: true },
    ]);
  });

  it('keeps the definition’s version of an item it holds, so edits to its JSON survive', () => {
    const edited = { name: 'book', description: 'Edited' };

    const [, row] = capabilityRows('tools', [search, book], { tools: [edited] });

    expect(row.item).toBe(edited);
  });

  it('adds what only the definition holds after the proxy’s own, flagged as not on the proxy', () => {
    const gone = { name: 'retired' };

    const rows = capabilityRows('tools', [search], { tools: [search, gone] });

    expect(rows.map(({ key, onProxy, selected }) => [key, onProxy, selected])).toEqual([
      ['search', true, true],
      ['retired', false, true],
    ]);
  });

  it('tells resources apart by URI, not name', () => {
    const rows = capabilityRows('resources', [guide], { resources: [{ uri: 'file:///guide.md' }] });

    expect(rows).toHaveLength(1);
    expect(rows[0]).toMatchObject({ key: 'file:///guide.md', selected: true });
  });

  it('copes with a kind the definition lacks, or holds as something else', () => {
    expect(capabilityRows('tools', [search], {})[0].selected).toBe(false);
    expect(capabilityRows('tools', [search], { tools: 'nope' })[0].selected).toBe(false);
  });

  it('ignores definition entries that are not objects', () => {
    expect(capabilityRows('tools', [], { tools: ['text', null, 3, [], search] })).toHaveLength(1);
  });

  it('gives an item without a usable name a position-based key', () => {
    const rows = capabilityRows('tools', [{ description: 'anonymous' }, { name: '' }], {});

    expect(rows.map(({ key }) => key)).toEqual(['#0', '#1']);
  });
});

describe('toggleCapability', () => {
  it('selects an item the definition does not hold yet', () => {
    const next = toggleCapability({ tools: [] }, 'tools', [search, book], 'book');

    expect(next.tools).toEqual([book]);
  });

  it('deselects an item it holds', () => {
    const next = toggleCapability({ tools: [search, book] }, 'tools', [search, book], 'search');

    expect(next.tools).toEqual([book]);
  });

  it('keeps the proxy’s order, not the order items were ticked in', () => {
    const afterBook = toggleCapability({ tools: [] }, 'tools', [search, book], 'book');
    const afterSearch = toggleCapability(afterBook, 'tools', [search, book], 'search');

    expect(afterSearch.tools).toEqual([search, book]);
  });

  it('only changes the kind it was asked about', () => {
    const definition = { tools: [], resources: [guide], prompts: [{ name: 'p' }] };

    const next = toggleCapability(definition, 'tools', [search], 'search');

    expect(next.resources).toEqual([guide]);
    expect(next.prompts).toEqual([{ name: 'p' }]);
  });

  it('carries along anything else in the definition untouched', () => {
    const next = toggleCapability({ tools: [], version: '1.2', extra: { a: 1 } }, 'tools', [search], 'search');

    expect(next).toMatchObject({ version: '1.2', extra: { a: 1 } });
  });

  it('writes all three lists even when the definition had none', () => {
    const next = toggleCapability({}, 'tools', [search], 'search');

    expect(next).toEqual({ tools: [search], resources: [], prompts: [] });
  });

  it('can deselect an item that only the definition holds, which drops it for good', () => {
    const gone = { name: 'retired' };

    const next = toggleCapability({ tools: [search, gone] }, 'tools', [search], 'retired');

    expect(next.tools).toEqual([search]);
  });
});

describe('setAllCapabilities', () => {
  it('selects everything the proxy offers', () => {
    expect(setAllCapabilities({ tools: [] }, 'tools', [search, book], true).tools).toEqual([search, book]);
  });

  it('clears the kind, including what only the definition held', () => {
    const next = setAllCapabilities({ tools: [search, { name: 'retired' }] }, 'tools', [search], false);

    expect(next.tools).toEqual([]);
  });
});

describe('definitionToJson', () => {
  it('writes readable JSON that reads back as the same definition', () => {
    const definition = { tools: [search], resources: [], prompts: [] };

    const text = definitionToJson(definition);

    expect(text).toContain('\n');
    expect(JSON.parse(text)).toEqual(definition);
  });
});
