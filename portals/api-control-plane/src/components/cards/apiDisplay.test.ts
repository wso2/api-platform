import { describe, expect, it } from 'vitest';

import type { Api } from '../../types/domain';
import {
  componentStatusColor,
  COMPONENT_KIND_LABEL,
  filterApis,
  groupApisByKind,
} from './apiDisplay';

const make = (over: Partial<Api>): Api => ({
  id: over.id || 'c',
  projectId: 'p',
  name: over.name || 'c',
  displayName: over.displayName || over.name || 'c',
  handler: over.handler || 'c',
  kind: over.kind || 'API_PROXY',
  status: over.status || 'ACTIVE',
  description: over.description,
  version: over.version,
});

describe('apiDisplay', () => {
  const components = [
    make({ id: '1', name: 'orders', kind: 'API_PROXY' }),
    make({ id: '3', name: 'web', kind: 'WEB_APP' }),
  ];

  it('groups by API Proxy / others', () => {
    const groups = groupApisByKind(components);
    expect(groups.apiProxies.map((c) => c.id)).toEqual(['1']);
    expect(groups.others.map((c) => c.id)).toEqual(['3']);
  });

  it('filters by name/displayName/description, case-insensitive', () => {
    expect(filterApis(components, 'ORDER').map((c) => c.id)).toEqual(['1']);
    expect(filterApis(components, '')).toHaveLength(2);
  });

  it('maps status to a chip color', () => {
    expect(componentStatusColor('ACTIVE')).toBe('success');
    expect(componentStatusColor('PENDING')).toBe('warning');
    expect(componentStatusColor('FAILED')).toBe('error');
    expect(componentStatusColor('DRAFT')).toBe('default');
  });

  it('labels every component kind', () => {
    expect(COMPONENT_KIND_LABEL.API_PROXY).toBe('API Proxy');
    expect(COMPONENT_KIND_LABEL.SERVICE).toBe('Service');
    expect(COMPONENT_KIND_LABEL.WEB_APP).toBe('Web App');
  });
});
