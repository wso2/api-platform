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

import { redeclaredSections, tomlSections } from './toml';

describe('tomlSections', () => {
  it('finds table headers at the start of a line', () => {
    expect(tomlSections('[mcp]\nenabled = true\n\n[other]\nx = 1')).toEqual([
      'mcp',
      'other',
    ]);
  });

  it('tolerates indentation and trailing spaces around a header', () => {
    expect(tomlSections('  [mcp]  \n')).toEqual(['mcp']);
    expect(tomlSections('\t[mcp]\t\n')).toEqual(['mcp']);
  });

  it('ignores a bracketed value, which is not a table declaration', () => {
    // Only a header occupies a line on its own, so an array value must not read
    // as a section — otherwise every list would produce a false warning.
    expect(tomlSections('hosts = ["a", "b"]')).toEqual([]);
    expect(tomlSections('key = [1]')).toEqual([]);
  });
});

describe('redeclaredSections', () => {
  it('warns about the two sections the bootstrap seed already emits', () => {
    // TOML forbids declaring a table twice and the chart APPENDS this text to a
    // config.toml it has already written, so either of these is a parse error
    // at gateway startup rather than an override.
    expect(
      redeclaredSections('[policy_configurations.ratelimit_v1]\nx = 1')
    ).toEqual(['policy_configurations.ratelimit_v1']);
    expect(
      redeclaredSections(
        '[policy_configurations.llm_cost_ratelimit_v1]\nx = 1'
      )
    ).toEqual(['policy_configurations.llm_cost_ratelimit_v1']);
  });

  it('reports each seeded section once, however often it appears', () => {
    expect(
      redeclaredSections(
        '[policy_configurations.ratelimit_v1]\na = 1\n[policy_configurations.ratelimit_v1]\nb = 2'
      )
    ).toEqual(['policy_configurations.ratelimit_v1']);
  });

  it('says nothing about a section the platform does not emit', () => {
    // A new section is exactly what this field is for; only a REPEAT is fatal.
    expect(redeclaredSections('[mcp]\nenabled = true')).toEqual([]);
    expect(redeclaredSections('')).toEqual([]);
  });

  it('does not match a section whose name merely starts the same way', () => {
    expect(
      redeclaredSections('[policy_configurations.ratelimit_v2]\nx = 1')
    ).toEqual([]);
  });

  it('sees a seeded header that carries a trailing comment', () => {
    // TOML comments run to end of line, so this declares the table just as
    // surely as the bare form -- and it is the form that kills a gateway
    // silently if the warning misses it.
    expect(
      redeclaredSections('[policy_configurations.ratelimit_v1] # note')
    ).toEqual(['policy_configurations.ratelimit_v1']);
    expect(
      tomlSections('[a.b]\t#  trailing\n[c]#tight\n')
    ).toEqual(['a.b', 'c']);
    // A commented-OUT header is still a comment, not a declaration.
    expect(tomlSections('# [policy_configurations.ratelimit_v1]')).toEqual([]);
  });
});
