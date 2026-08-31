/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import type { FC } from 'react';
import { Box } from '@wso2/oxygen-ui';
import type { Tone } from '../utils/status';

export type StatusDotProps = {
  tone: Tone;
  size?: number;
};

/** A small colored dot, themed via the same tone tokens as `Chip`'s `color` prop — never raw hex, so it follows light/dark and theme swaps automatically. */
const StatusDot: FC<StatusDotProps> = ({ tone, size = 8 }) => (
  <Box
    sx={{
      width: size,
      height: size,
      borderRadius: '50%',
      bgcolor: tone === 'default' ? 'text.disabled' : `${tone}.main`,
      flexShrink: 0,
    }}
  />
);

export default StatusDot;
