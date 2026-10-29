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

import type { ReactNode } from 'react';
import { defineMessages, useIntl } from 'react-intl';
import { Alert, Box, Stack, Typography } from '@wso2/oxygen-ui';

import AILoader from '../../../../Components/AILoader';

const messages = defineMessages({
  loading: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.states.loading',
    defaultMessage: 'Loading',
  },
  somethingWentWrong: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.states.somethingWentWrong',
    defaultMessage: 'Something went wrong',
  },
});

/** The workspace's branded loader, centered in its container with the label beneath. */
export function LoadingState({ label }: { label?: string }) {
  const intl = useIntl();
  return (
    <Stack alignItems="center" spacing={2} sx={{ py: 6 }}>
      <AILoader />
      <Typography color="text.secondary" variant="body2">
        {label ?? intl.formatMessage(messages.loading)}
      </Typography>
    </Stack>
  );
}

/** A compact dashed panel for a list with nothing to show, with the action that fills it, if there is one. */
export function EmptyState({
  action,
  description,
  title,
}: {
  action?: ReactNode;
  description?: string;
  title: string;
}) {
  return (
    <Box
      sx={{
        border: '1px dashed',
        borderColor: 'divider',
        borderRadius: 3,
        p: 4,
        textAlign: 'center',
      }}
    >
      <Typography variant="h6">{title}</Typography>
      {description && (
        <Typography color="text.secondary" sx={{ mt: 1 }}>
          {description}
        </Typography>
      )}
      {action && <Box sx={{ mt: 3 }}>{action}</Box>}
    </Box>
  );
}

export function ErrorState({ title, message }: { title?: string; message?: string }) {
  const intl = useIntl();
  return (
    <Alert severity="error">
      <Typography fontWeight={600}>{title ?? intl.formatMessage(messages.somethingWentWrong)}</Typography>
      {message && <Typography>{message}</Typography>}
    </Alert>
  );
}
