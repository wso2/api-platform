import { afterEach, describe, expect, it } from 'vitest';

import {
  getDraggedPolicy,
  scopeId,
  setDraggedPolicy,
  type DraggedPolicy,
} from './policyDnd';

const policy: DraggedPolicy = {
  name: 'mediation.rewrite_resource_path',
  version: '2.0.0',
  displayName: 'Rewrite Resource Path',
};

describe('policyDnd', () => {
  afterEach(() => setDraggedPolicy(null));

  it('round-trips the dragged policy through the module holder', () => {
    expect(getDraggedPolicy()).toBeNull();
    setDraggedPolicy(policy);
    expect(getDraggedPolicy()).toEqual(policy);
  });

  it('clears the dragged policy when set to null', () => {
    setDraggedPolicy(policy);
    setDraggedPolicy(null);
    expect(getDraggedPolicy()).toBeNull();
  });

  it('builds a stable id for the api scope', () => {
    expect(scopeId({ kind: 'api' })).toBe('api');
  });

  it('builds a per-operation id from the index', () => {
    expect(scopeId({ kind: 'operation', index: 0 })).toBe('op-0');
    expect(scopeId({ kind: 'operation', index: 3 })).toBe('op-3');
  });
});
