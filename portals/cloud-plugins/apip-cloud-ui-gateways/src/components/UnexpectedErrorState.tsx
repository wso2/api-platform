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

import type { FC } from 'react';
import { Box, Button, Stack, Typography, useTheme } from '@wso2/oxygen-ui';
import { RotateCcw } from '@wso2/oxygen-ui-icons-react';

/**
 * A browser window with a broken face, drawn with plain `<svg>` and colours
 * resolved from `useTheme()` rather than Oxygen's `ColorSchemeSVG` (that
 * helper only exists from Oxygen 0.13.1; this package is typechecked against
 * 0.5.0 — see `apip-cloud-ui-logs/ComingSoonPanel.tsx` for the same
 * workaround applied to the console's "coming soon" barricade).
 */
const BrokenPageIllustration: FC = () => {
  const theme = useTheme();
  const border = theme.palette.divider;
  const warning = theme.palette.warning.main;
  const muted = theme.palette.text.secondary;
  const surface = theme.palette.background.paper;
  const blob = theme.palette.action.hover;

  return (
    <svg width={200} height={160} viewBox="0 0 200 160" aria-hidden>
      <ellipse cx="100" cy="90" rx="78" ry="58" fill={blob} />

      <rect x="30" y="35" width="140" height="95" rx="10" fill={surface} stroke={border} strokeWidth="1.5" />
      <path d="M30 59a10 10 0 0 1 10-10h120a10 10 0 0 1 10 10v4H30z" fill={blob} />
      <circle cx="46" cy="49" r="4" fill={warning} />
      <circle cx="62" cy="49" r="4" fill={border} />
      <circle cx="78" cy="49" r="4" fill={border} />
      <line x1="30" y1="63" x2="170" y2="63" stroke={border} strokeWidth="1.5" />

      <g stroke={muted} strokeWidth="3" strokeLinecap="round">
        <path d="M76 86l12 12M88 86l-12 12" />
        <path d="M112 86l12 12M124 86l-12 12" />
      </g>
      <path d="M80 114q20 -16 40 0" stroke={muted} strokeWidth="3" fill="none" strokeLinecap="round" />

      <path d="M150 8l22 38h-44z" fill={warning} stroke={surface} strokeWidth="3" strokeLinejoin="round" />
      <rect x="147.5" y="20" width="5" height="14" rx="2.5" fill={surface} />
      <circle cx="150" cy="39" r="2.5" fill={surface} />

      <g stroke={border} strokeWidth="1.5" strokeLinecap="round">
        <path d="M14 30l6 6M20 30l-6 6M17 24v16" />
        <path d="M176 118l5 5M181 118l-5 5M178.5 113v15" />
      </g>
    </svg>
  );
};

export type UnexpectedErrorStateProps = {
  /** What specifically failed to load, e.g. "your gateways". */
  description: string;
  /**
   * The actual error message, when there is one worth showing. Covers the
   * failure a plain Retry can't fix (a permissions error, a validation
   * rejection surfaced as a load failure) — without it the page says only
   * "try again", which is actively unhelpful when trying again can't work.
   */
  detail?: string;
  onRetry: () => void;
  retryLabel?: string;
};

/**
 * Full-page error state for a cloud page whose initial load failed (a 500,
 * a network drop, anything the user can plausibly fix by trying again).
 *
 * Deliberately distinct from the inline `Alert` + Retry banner some of these
 * pages still use for a *refetch* failure with existing data on screen —
 * this one replaces the whole page, so it is only for the case where there
 * is nothing underneath it worth keeping visible.
 */
const UnexpectedErrorState: FC<UnexpectedErrorStateProps> = ({
  description,
  detail,
  onRetry,
  retryLabel = 'Retry',
}) => (
  <Box
    sx={{
      alignItems: 'center',
      display: 'flex',
      justifyContent: 'center',
      minHeight: '50vh',
      px: 3,
      py: 6,
    }}
  >
    <Stack alignItems="center" spacing={1} sx={{ maxWidth: 420 }}>
      <BrokenPageIllustration />
      <Typography sx={{ fontWeight: 700, pt: 2 }} variant="h5">
        An unexpected error occurred.
      </Typography>
      <Typography color="text.secondary" sx={{ textAlign: 'center' }}>
        {description}
      </Typography>
      {detail && (
        <Typography
          color="text.disabled"
          sx={{ fontFamily: 'monospace', textAlign: 'center', wordBreak: 'break-word' }}
          variant="caption"
        >
          {detail}
        </Typography>
      )}
      <Button onClick={onRetry} startIcon={<RotateCcw size={16} />} sx={{ mt: 2 }} variant="contained">
        {retryLabel}
      </Button>
    </Stack>
  </Box>
);

export default UnexpectedErrorState;
