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
import { Button, Card, CardContent, Stack, Typography } from '@wso2/oxygen-ui';

export type ExternalToolPanelProps = {
  /** Headline naming where the feature actually lives. */
  title: ReactNode;
  /** One sentence on what the user will find once they get there. */
  description: ReactNode;
  /** Label of the hand-off button. */
  actionLabel: ReactNode;
  /** Absolute URL of the external console. Opened in a new tab. */
  href: string;
};

/**
 * Full-width panel for a page whose feature is served by a product outside this
 * console; the Insights pages, which live in Moesif.
 *
 * Distinct from `ComingSoon`: nothing is pending here, the capability exists,
 * just not on this screen. So it states where the feature is and hands the user
 * straight to it, rather than asking them to wait.
 */
export function ExternalToolPanel({
  actionLabel,
  description,
  href,
  title,
}: ExternalToolPanelProps) {
  return (
    <Card variant="outlined">
      <CardContent
        sx={{
          alignItems: 'center',
          display: 'flex',
          justifyContent: 'center',
          minHeight: 300,
          px: 3,
          py: 6,
        }}
      >
        <Stack alignItems="center" spacing={2} sx={{ maxWidth: 560, textAlign: 'center' }}>
          <Typography variant="h5">{title}</Typography>
          <Typography color="text.secondary" variant="body2">
            {description}
          </Typography>
          <Button
            component="a"
            href={href}
            // `noopener` keeps the opened tab from reaching back through
            // `window.opener`; `noreferrer` withholds the console URL from it.
            rel="noopener noreferrer"
            target="_blank"
            variant="contained"
          >
            {actionLabel}
          </Button>
        </Stack>
      </CardContent>
    </Card>
  );
}
