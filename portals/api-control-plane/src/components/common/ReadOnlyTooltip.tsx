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

import { Box, Tooltip } from '@wso2/oxygen-ui';
import type { JSX, ReactNode } from 'react';

import { useResourceLimits } from '../../hooks/useResourceLimits';

/**
 * Whether the organization may not write at all, for composing into a control's
 * `disabled`.
 *
 * Most write controls need nothing of their own: they are gated on a write
 * scope, and the permission layer already answers `false` for those while the
 * organization is read-only. This is for the rest — a form's submit, a drawer's
 * Apply — which no scope check covers.
 */
export function useOrgReadOnly(): boolean {
  return useResourceLimits().readOnly;
}

export type ReadOnlyTooltipProps = {
  /** The control being disabled. */
  children: ReactNode;
};

/**
 * Explains, on hover, a control that is disabled because the organization is
 * read-only. Renders its child untouched when it is not.
 *
 * A disabled control fires no pointer events of its own — MUI sets
 * `pointer-events: none` on it — so the tooltip needs a wrapping element to
 * hang off, which is what the `<Box component="span">` is for. The same
 * workaround already appears in `permissions/Can.tsx`.
 */
export default function ReadOnlyTooltip({ children }: ReadOnlyTooltipProps): JSX.Element {
  const { readOnly, readOnlyMessage } = useResourceLimits();

  if (!readOnly) return <>{children}</>;

  return (
    <Tooltip title={readOnlyMessage}>
      <Box component="span">{children}</Box>
    </Tooltip>
  );
}
