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

import type { FC, ReactNode } from 'react';
import { Box, Button, Stack, Typography, useTheme } from '@wso2/oxygen-ui';

/**
 * The roadworks barricade from the console's own `ComingSoon`, redrawn.
 *
 * Core paints it through Oxygen's `ColorSchemeSVG`, which resolves names like
 * `surface` and `warning` against the theme. That component only exists from
 * Oxygen 0.13.1, and this package is typechecked against 0.5.0 as well — so the
 * colours are resolved from the theme here instead. Same geometry, so the two
 * pages look alike; importing core's component is what does not compile.
 */
const Barricade: FC = () => {
  const theme = useTheme();
  const border = theme.palette.divider;
  const warning = theme.palette.warning.main;
  return (
    <svg width={268} height={210} viewBox="0 0 268 210" aria-hidden>
      <path
        fill={theme.palette.action.hover}
        d="M46 92c-14-42 18-84 66-88 44-4 68 16 102 22 30 6 52 30 50 62-2 34-26 52-58 62-36 12-70 20-104 8C64 146 56 122 46 92Z"
      />
      <g stroke={border} strokeWidth="1.5" fill={theme.palette.background.paper}>
        <path d="M56 62h84l-14 46H42Z" />
        <path d="M146 62h84l-14 46h-84Z" />
      </g>
      <g fill={warning}>
        <path d="M74 64h26l-14 42H60Z" />
        <path d="M112 64h24l-14 42H98Z" />
        <path d="M158 64h26l-14 42h-26Z" />
        <path d="M198 64h22l-14 42h-22Z" />
      </g>
      <g stroke={border} strokeWidth="1.5" fill="none">
        <path d="M56 62h84l-14 46H42Z" />
        <path d="M146 62h84l-14 46h-84Z" />
      </g>
      <g stroke={border} strokeWidth="1.5" strokeLinecap="round">
        <path d="M92 108v56M186 108v56" />
        <path d="M42 164h96M158 164h72" />
      </g>
      <g fill={warning}>
        <circle cx="66" cy="48" r="5" />
        <circle cx="222" cy="48" r="5" />
      </g>
      <g stroke={warning} strokeWidth="1.5" strokeLinecap="round">
        <path d="M58 36l-4-6M66 34v-7M74 36l4-6" />
        <path d="M214 36l-4-6M222 34v-7M230 36l4-6" />
      </g>
    </svg>
  );
};

export type ComingSoonPanelProps = {
  /** The feature's own name, read as "{feature} will be available soon." */
  feature: string;
  /** An extra line under the first, e.g. where to go in the meantime. */
  detail?: ReactNode;
  /** Somewhere to go instead. The plugin holds no router, so it is a callback. */
  action?: { label: string; onClick: () => void };
};

/**
 * A page whose feature is not built yet, in the shape the console's built-in
 * pages use — so Observability reads the same at every scope.
 */
const ComingSoonPanel: FC<ComingSoonPanelProps> = ({ action, detail, feature }) => (
  <Box
    sx={{
      alignItems: 'center',
      display: 'flex',
      justifyContent: 'center',
      minHeight: '60vh',
      px: 3,
      py: 6,
    }}
  >
    <Stack alignItems="center" spacing={1} sx={{ maxWidth: 520 }}>
      <Barricade />
      <Typography sx={{ fontWeight: 700, pt: 1 }} variant="h4">
        Coming Soon
      </Typography>
      <Typography color="text.secondary" sx={{ textAlign: 'center' }}>
        {feature} will be available soon.
      </Typography>
      {detail ? (
        <Typography color="text.secondary" sx={{ textAlign: 'center' }}>
          {detail}
        </Typography>
      ) : null}
      {action ? (
        <Button onClick={action.onClick} sx={{ mt: 2 }} variant="contained">
          {action.label}
        </Button>
      ) : null}
    </Stack>
  </Box>
);

export default ComingSoonPanel;
