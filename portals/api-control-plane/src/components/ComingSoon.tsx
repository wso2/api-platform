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

import type { ReactNode } from 'react';
import { Box, Button, ColorSchemeSVG, Stack, Typography } from '@wso2/oxygen-ui';
import { FormattedMessage } from 'react-intl';
import { Link } from 'react-router-dom';

/**
 * The roadworks barricade, drawn through Oxygen's `ColorSchemeSVG` so its fills
 * and strokes resolve to theme colours rather than hard-coded hex — the console
 * has a light/dark toggle, and a fixed-colour illustration would go muddy in
 * dark mode.
 */
function BarricadeIllustration() {
  return (
    <ColorSchemeSVG width={268} height={210} viewBox="0 0 268 210">
      {/* Soft blob behind the barricade. */}
      <path
        fill="surface"
        d="M46 92c-14-42 18-84 66-88 44-4 68 16 102 22 30 6 52 30 50 62-2 34-26 52-58 62-36 12-70 20-104 8C64 146 56 122 46 92Z"
      />

      {/* Two angled boards, each with its own diagonal stripes. */}
      <g stroke="border" strokeWidth="1.5" fill="background">
        <path d="M56 62h84l-14 46H42Z" />
        <path d="M146 62h84l-14 46h-84Z" />
      </g>
      <g fill="warning">
        <path d="M74 64h26l-14 42H60Z" />
        <path d="M112 64h24l-14 42H98Z" />
        <path d="M158 64h26l-14 42h-26Z" />
        <path d="M198 64h22l-14 42h-22Z" />
      </g>
      <g stroke="border" strokeWidth="1.5" fill="none">
        <path d="M56 62h84l-14 46H42Z" />
        <path d="M146 62h84l-14 46h-84Z" />
      </g>

      {/* Legs and the ground line they stand on. */}
      <g stroke="border" strokeWidth="1.5" strokeLinecap="round">
        <path d="M92 108v56M186 108v56" />
        <path d="M42 164h96M158 164h72" />
      </g>

      {/* Warning lamps, one at each end. */}
      <g fill="warning">
        <circle cx="66" cy="48" r="5" />
        <circle cx="222" cy="48" r="5" />
      </g>
      <g stroke="warning" strokeWidth="1.5" strokeLinecap="round">
        <path d="M58 36l-4-6M66 34v-7M74 36l4-6" />
        <path d="M214 36l-4-6M222 34v-7M230 36l4-6" />
      </g>
    </ColorSchemeSVG>
  );
}

export type ComingSoonProps = {
  /**
   * The feature's own name, interpolated into the message — pass a
   * `<FormattedMessage>` so the page keeps owning its own translation.
   */
  feature: ReactNode;
  /** An extra line under the first, e.g. where to go in the meantime. */
  detail?: ReactNode;
  /** Somewhere to go instead. Omitted when there is no alternative yet. */
  action?: { label: ReactNode; to: string };
};

/**
 * Placeholder for a page whose feature is not built yet.
 *
 * Distinct from `EmptyState` in `StateViews`: that one says "nothing here yet,
 * add something", which invites an action the user *can* take. This says "not
 * here yet at all", so it never implies the page will fill up on its own.
 */
export function ComingSoon({ action, detail, feature }: ComingSoonProps) {
  return (
    <Box
      sx={{
        alignItems: 'center',
        display: 'flex',
        justifyContent: 'center',
        // Fills the content area so the block sits optically centred rather than
        // hugging the top of the page.
        minHeight: '60vh',
        px: 3,
        py: 6,
      }}
    >
      <Stack alignItems="center" spacing={1} sx={{ maxWidth: 520 }}>
        <BarricadeIllustration />
        <Typography sx={{ fontWeight: 700, pt: 1 }} variant="h4">
          <FormattedMessage id="components.comingSoon.title" defaultMessage="Coming Soon" />
        </Typography>
        <Typography color="text.secondary" sx={{ textAlign: 'center' }}>
          <FormattedMessage
            id="components.comingSoon.body"
            defaultMessage="{feature} will be available soon."
            values={{ feature }}
          />
        </Typography>
        {detail && (
          <Typography color="text.secondary" sx={{ textAlign: 'center' }}>
            {detail}
          </Typography>
        )}
        {action && (
          <Button component={Link} sx={{ mt: 2 }} to={action.to} variant="contained">
            {action.label}
          </Button>
        )}
      </Stack>
    </Box>
  );
}
