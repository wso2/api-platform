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

import { Box, Card, Divider, Stack, Typography } from '@wso2/oxygen-ui';
import type { ReactNode } from 'react';

type DocumentCardProps = {
  /** Buttons for the header's right-hand end. */
  actions?: ReactNode;
  /** The document's body, in the card's scrolling area. */
  children: ReactNode;
  title: string;
};

/** One document in a card: a header with its name and actions over its content, which alone scrolls. */
export function DocumentCard({ actions, children, title }: DocumentCardProps) {
  return (
    <Card component="article" sx={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <Stack
        direction="row"
        spacing={2}
        sx={{
          alignItems: 'center',
          bgcolor: 'action.hover',
          flexShrink: 0,
          justifyContent: 'space-between',
          px: 3,
          py: 1.5,
        }}
      >
        <Stack direction="row" spacing={1.5} sx={{ alignItems: 'center', minWidth: 0 }}>
          <Typography component="h2" noWrap sx={{ fontWeight: 600, minWidth: 0 }} title={title} variant="h6">
            {title}
          </Typography>
        </Stack>
        {actions}
      </Stack>
      <Divider />
      <Box sx={{ flex: 1, minHeight: 0, overflowY: 'auto', px: 3, py: 3 }}>
        <Box sx={{ maxWidth: 820 }}>{children}</Box>
      </Box>
    </Card>
  );
}
