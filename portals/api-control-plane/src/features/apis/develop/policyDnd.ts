import type { PolicySummary } from '../../../api/policyHub/policyHubClient';

/**
 * Native HTML5 drag-and-drop for the policy workspace. The catalog (right panel)
 * is the drag source; the Policies panel zones (left) are drop targets. We carry
 * a lightweight policy ref both on `dataTransfer` (for native semantics) and in a
 * module-level holder (because `dataTransfer.getData` is empty during dragover,
 * where we need the payload to validate/highlight).
 */
export const POLICY_DND_MIME = 'application/x-policy-ref';

export type DraggedPolicy = Pick<
  PolicySummary,
  'name' | 'version' | 'displayName'
>;

let dragged: DraggedPolicy | null = null;

export const setDraggedPolicy = (policy: DraggedPolicy | null) => {
  dragged = policy;
};

export const getDraggedPolicy = (): DraggedPolicy | null => dragged;

/** Scope a dropped policy targets. */
export type PolicyScope = { kind: 'api' } | { kind: 'operation'; index: number };

export const scopeId = (scope: PolicyScope): string =>
  scope.kind === 'api' ? 'api' : `op-${scope.index}`;
