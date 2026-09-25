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
import { Alert, Box, Button, Stack, Typography } from '@wso2/oxygen-ui';
import { Lock } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, useIntl } from 'react-intl';

import { AppLoader } from './AppLoader';

const messages = defineMessages({
  forbiddenTitle: {
    id: 'apiControlPlane.components.StateViews.forbiddenTitle',
    defaultMessage: "You don't have permission to view this",
    description: 'Heading shown in place of a page the signed-in user lacks the scope to read.',
  },
  forbiddenDescription: {
    id: 'apiControlPlane.components.StateViews.forbiddenDescription',
    defaultMessage:
      'Your account does not include access to this page. An administrator in your organization can grant it.',
    description:
      'Explanation under the forbidden heading. Names who can fix it, and deliberately not which scope is missing — that is meaningless to the reader and tells a probing caller what to look for.',
  },
});

export function LoadingState({
  label = 'Loading',
  fullScreen = false,
}: {
  label?: string;
  fullScreen?: boolean;
}) {
  return <AppLoader fullScreen={fullScreen} label={label} />;
}

type EmptyStateProps = {
  title: string;
  description?: string;
  /** Rendered only alongside `onAction`; one empty state offers one way out. */
  actionLabel?: string;
  /** Leading mark on the action button, e.g. `<Plus />` for a create prompt. */
  actionIcon?: ReactNode;
  /** Artwork for first-run case; triggers centred full-height layout. */
  illustration?: ReactNode;
  onAction?: () => void;
};

/**
 * Empty state with optional action.
 * Uses an illustration for first-run content and a compact panel for lists.
 */
export function EmptyState({
  title,
  description,
  actionLabel,
  actionIcon,
  illustration,
  onAction,
}: EmptyStateProps) {
  const action =
    actionLabel && onAction ? (
      <Button onClick={onAction} startIcon={actionIcon} variant="contained">
        {actionLabel}
      </Button>
    ) : null;

  if (illustration) {
    return (
      <Box
        sx={{
          alignItems: 'center',
          display: 'flex',
          justifyContent: 'center',
          // Claims the rest of the content area so the block sits optically
          // centred instead of hugging whatever heading is above it.
          flexGrow: 1,
          minHeight: '50vh',
          px: 3,
          py: 6,
        }}
      >
        <Stack alignItems="center" spacing={1} sx={{ maxWidth: 440 }}>
          {illustration}
          <Typography sx={{ fontWeight: 700, pt: 2 }} variant="h5">
            {title}
          </Typography>
          {description && (
            <Typography color="text.secondary" sx={{ textAlign: 'center' }}>
              {description}
            </Typography>
          )}
          {action && <Box sx={{ pt: 2 }}>{action}</Box>}
        </Stack>
      </Box>
    );
  }

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

export function ErrorState({
  title = 'Something went wrong',
  message,
}: {
  title?: string;
  message?: string;
}) {
  return (
    <Alert severity="error">
      <Typography fontWeight={600}>{title}</Typography>
      {message && <Typography>{message}</Typography>}
    </Alert>
  );
}

type ForbiddenStateProps = {
  /** Overrides the default heading, e.g. to name the thing being withheld. */
  title?: string;
  description?: string;
  /** Artwork, matching `EmptyState`'s option. The lock mark is used without it. */
  illustration?: ReactNode;
};

/**
 * Shown in place of content the signed-in user's scopes do not allow them to
 * read — pass it as `fallback` to `Can`.
 *
 * Deliberately *not* an `EmptyState`. "This organization has no gateways" and
 * "you may not see this organization's gateways" are different facts, and a
 * console that renders them identically teaches users to distrust every empty
 * table it shows them. It is also not an `ErrorState`: nothing failed, so the
 * red alert styling would be a lie.
 */
export function ForbiddenState({ title, description, illustration }: ForbiddenStateProps) {
  const intl = useIntl();

  return (
    <Box
      sx={{
        alignItems: 'center',
        display: 'flex',
        justifyContent: 'center',
        flexGrow: 1,
        minHeight: '40vh',
        px: 3,
        py: 6,
      }}
    >
      <Stack alignItems="center" spacing={1} sx={{ maxWidth: 440 }}>
        {illustration ?? <Lock aria-hidden size={40} strokeWidth={1.5} color="currentColor" />}
        <Typography sx={{ fontWeight: 700, pt: 2 }} variant="h5">
          {title ?? intl.formatMessage(messages.forbiddenTitle)}
        </Typography>
        <Typography color="text.secondary" sx={{ textAlign: 'center' }}>
          {description ?? intl.formatMessage(messages.forbiddenDescription)}
        </Typography>
      </Stack>
    </Box>
  );
}
