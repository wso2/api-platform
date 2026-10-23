/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import type { JSX, ReactNode } from 'react';
import { Box, Tooltip } from '@wso2/oxygen-ui';
import { useResourceLimits } from '../../hooks/useResourceLimits';

/**
 * Whether the organization may not write at all, for composing into a control's
 * `disabled`.
 *
 * Most write controls need nothing of their own: they are gated on a write
 * scope, and `useAppAuth` already answers `false` for those while the
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
 * A disabled `<button>` fires no events of its own, so the tooltip needs a
 * wrapping element to hang off — the same `<Box component="span">` the
 * quota-disabled create buttons already use.
 */
export default function ReadOnlyTooltip({
  children,
}: ReadOnlyTooltipProps): JSX.Element {
  const { readOnly, readOnlyMessage } = useResourceLimits();

  if (!readOnly) return <>{children}</>;

  return (
    <Tooltip title={readOnlyMessage}>
      <Box component="span">{children}</Box>
    </Tooltip>
  );
}
