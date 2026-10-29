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

const values = { description: 'Plan trips.', productionUrl: 'https://gw/travel/mcp', sandboxUrl: '' };

describe('buildLandingPageStarter', () => {
  it('starts from the overview, then the endpoints that are set', () => {
    expect(buildLandingPageStarter(values, '')).toBe(
      '## Overview\n\nPlan trips.\n\n## Endpoints\n\n- **Production:** https://gw/travel/mcp\n'
    );
    expect(
      buildLandingPageStarter({ ...values, productionUrl: '', sandboxUrl: 'https://sb/mcp' }, '')
    ).toContain('- **Sandbox:** https://sb/mcp');
  });

  it('leaves out the overview when there is no description', () => {
    expect(buildLandingPageStarter({ ...values, description: '  ' }, '')).toBe(
      '## Endpoints\n\n- **Production:** https://gw/travel/mcp\n'
    );
    expect(buildLandingPageStarter({ description: '', productionUrl: '', sandboxUrl: '' }, '')).toBe('\n');
  });

  it('lists the tools, resources and prompts the definition holds, with their descriptions', () => {
    const definition = JSON.stringify({
      tools: [{ name: 'search', description: 'Find\n  trips' }, { name: 'book' }],
      resources: [{ name: 'itinerary', uri: 'trip://1' }],
      prompts: [{ name: 'plan', description: 'Plan a trip' }],
    });
    const starter = buildLandingPageStarter(values, definition);
    expect(starter).toContain('## Tools\n\n- **search** — Find trips\n- **book**');
    expect(starter).toContain('## Resources\n\n- **itinerary**');
    expect(starter).toContain('## Prompts\n\n- **plan** — Plan a trip');
    expect(starter.indexOf('## Endpoints')).toBeLessThan(starter.indexOf('## Tools'));
  });

  it('leaves out a heading with nothing under it, and entries that have no name', () => {
    const starter = buildLandingPageStarter(
      { ...values, productionUrl: '' },
      JSON.stringify({ tools: [{ description: 'nameless' }, 'echo', null], resources: [] })
    );
    expect(starter).toBe('## Overview\n\nPlan trips.\n');
  });
});
