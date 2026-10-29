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

import { DISABLED_ACTION_SX } from '../auth/permissions';

/** The monogram or logo tile on an Overview header. */
export const OVERVIEW_AVATAR_SX = {
  bgcolor: 'primary.light',
  borderRadius: 2.5,
  color: 'primary.contrastText',
  flexShrink: 0,
  fontSize: 32,
  fontWeight: 700,
  height: 80,
  width: 80,
} as const;

/** Lines of an Overview header's description shown; the full text is in the tooltip. */
export const OVERVIEW_DESCRIPTION_LINES = 2;

/** Wider than the default tooltip, so a long description reads as a short paragraph, not a tall column. */
export const OVERVIEW_DESCRIPTION_TOOLTIP_WIDTH = 480;

export const OVERVIEW_DESCRIPTION_SX = {
  display: '-webkit-box',
  maxWidth: 560,
  overflow: 'hidden',
  WebkitBoxOrient: 'vertical',
  WebkitLineClamp: OVERVIEW_DESCRIPTION_LINES,
} as const;

/** The pill shape an Overview header gives its action buttons; `width` fits the longest label on one line. */
export const overviewButtonSx = (width = 190) => ({
  ...DISABLED_ACTION_SX,
  borderRadius: 5,
  whiteSpace: 'nowrap',
  width,
});
