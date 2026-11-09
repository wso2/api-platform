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

import { buildLandingPageStarter } from './landingPageStarter';
import type { StoredDefinition } from './storedDefinition';

const values = { description: 'Manage loans.', productionUrl: 'https://api.example.com/loans', sandboxUrl: '' };

const openApi = (paths: Record<string, unknown>): StoredDefinition => ({
  format: 'json',
  text: JSON.stringify({ openapi: '3.0.0', paths }),
});

describe('buildLandingPageStarter', () => {
  it('writes the overview first, then the endpoints and the resources', () => {
    const definition = openApi({
      '/loans': { get: { summary: 'List loans' }, post: { summary: 'Open a loan' } },
      '/loans/{id}': { delete: {} },
    });

    expect(buildLandingPageStarter({ ...values, sandboxUrl: 'https://sandbox.example.com/loans' }, definition)).toBe(
      [
        '## Overview',
        '',
        'Manage loans.',
        '',
        '## Endpoints',
        '',
        '- **Production:** https://api.example.com/loans',
        '- **Sandbox:** https://sandbox.example.com/loans',
        '',
        '## Resources',
        '',
        '- **GET** `/loans` — List loans',
        '- **POST** `/loans` — Open a loan',
        '- **DELETE** `/loans/{id}`',
        '',
      ].join('\n'),
    );
  });

  it('lists only the methods the portal lists, and keeps a summary on one line', () => {
    const definition = openApi({
      '/loans': { head: {}, options: {}, parameters: [], put: { summary: 'Replace\n  all   loans' } },
    });

    const starter = buildLandingPageStarter(values, definition);

    expect(starter).toContain('- **PUT** `/loans` — Replace all loans');
    expect(starter).not.toMatch(/HEAD|OPTIONS/);
  });

  it('leaves the overview to be filled in when the API has no description', () => {
    expect(buildLandingPageStarter({ ...values, description: '' }, undefined)).toContain('## Overview\n\n\n\n## Endpoints');
  });

  it('leaves out the endpoints and resources it has nothing for', () => {
    expect(buildLandingPageStarter({ description: 'Hi', productionUrl: '', sandboxUrl: '' }, openApi({}))).toBe(
      '## Overview\n\nHi\n',
    );
  });

  it('lists no resources for a GraphQL schema or a definition that does not parse', () => {
    const sdl: StoredDefinition = { format: 'graphql', text: 'type Query { loans: [String] }' };
    const broken: StoredDefinition = { format: 'json', text: '{ not json' };

    expect(buildLandingPageStarter(values, sdl)).not.toContain('## Resources');
    expect(buildLandingPageStarter(values, broken)).not.toContain('## Resources');
  });
});
