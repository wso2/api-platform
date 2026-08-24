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

import { alpha, Box, useTheme } from '@wso2/oxygen-ui';

import type { ApiStatus } from '../../types/domain';
import { componentStatusColor } from './apiDisplay';

/**
 * Uppercase tinted status badge from the APIs page design: soft background,
 * matching border and text color, instead of the solid default Chip.
 */
export function StatusPill({ status }: { status: ApiStatus }) {
  const theme = useTheme();
  const color = componentStatusColor(status);
  const main =
    color === 'default'
      ? theme.palette.text.secondary
      : theme.palette[color].main;

  return (
    <Box
      component="span"
      sx={{
        alignItems: 'center',
        bgcolor: alpha(main, 0.14),
        border: '1px solid',
        borderColor: alpha(main, 0.35),
        borderRadius: '10px',
        color: main,
        display: 'inline-flex',
        flexShrink: 0,
        fontSize: 10,
        fontWeight: 600,
        letterSpacing: '0.04em',
        lineHeight: 1.6,
        px: 1.1,
        py: 0.25,
        textTransform: 'uppercase',
      }}
    >
      {status}
    </Box>
  );
}
