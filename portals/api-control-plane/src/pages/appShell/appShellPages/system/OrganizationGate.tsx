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

import { type ReactNode, useCallback, useEffect, useState } from 'react';
import { Box, Button, Stack, Typography } from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage } from 'react-intl';

import { useOrganizations } from '@/api/resources/organizations';
import { AppLoader } from '@/components/AppLoader';
import { useAuth } from '@/contexts/auth/AuthProvider';

const messages = defineMessages({
  loadingTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.system.OrganizationGate.loadingTitle',
    defaultMessage: 'Loading your organization',
    description: "Shown while the signed-in user's organizations are being fetched.",
  },
  provisioningTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.system.OrganizationGate.provisioningTitle',
    defaultMessage: 'Setting up your organization',
  },
  provisioningBody: {
    id: 'apiControlPlane.pages.appShell.appShellPages.system.OrganizationGate.provisioningBody',
    defaultMessage: 'This only happens once, and it will only take a moment.',
  },
  timeoutTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.system.OrganizationGate.timeoutTitle',
    defaultMessage: 'This is taking longer than expected',
  },
  timeoutBody: {
    id: 'apiControlPlane.pages.appShell.appShellPages.system.OrganizationGate.timeoutBody',
    defaultMessage:
      'Your organization is still being set up. Try again in a moment, or sign out and back in.',
  },
  retry: {
    id: 'apiControlPlane.pages.appShell.appShellPages.system.OrganizationGate.retry',
    defaultMessage: 'Try again',
    description: 'Button that re-checks whether the organization is ready. Verb phrase.',
  },
  logout: {
    id: 'apiControlPlane.pages.appShell.appShellPages.system.OrganizationGate.logout',
    defaultMessage: 'Log out',
    description: 'Button that ends the session. Verb phrase.',
  },
});

const FAST_POLL_MS = 2_000;
const SLOW_POLL_MS = 10_000;
const SLOW_DOWN_AFTER_MS = 30_000;
const GIVE_UP_AFTER_MS = 180_000;

function GateScreen({ actions, body, title }: { actions?: ReactNode; body: ReactNode; title: ReactNode }) {
  return (
    <Box
      sx={{
        alignItems: 'center',
        display: 'flex',
        justifyContent: 'center',
        minHeight: '100vh',
        px: 2,
        width: '100%',
      }}
    >
      <Stack alignItems="center" spacing={3} sx={{ maxWidth: 480, textAlign: 'center' }}>
        {!actions && <AppLoader />}
        <Stack aria-live="polite" role="status" spacing={1}>
          <Typography fontWeight={700} variant="h5">
            {title}
          </Typography>
          <Typography color="text.secondary" variant="body2">
            {body}
          </Typography>
        </Stack>
        {actions}
      </Stack>
    </Box>
  );
}

export function OrganizationGate({ children }: { children: ReactNode }) {
  const { logout, user } = useAuth();
  const [startedAt, setStartedAt] = useState(() => Date.now());
  const [isSlow, setIsSlow] = useState(false);
  const [hasGivenUp, setHasGivenUp] = useState(false);

  const hasOrganizationClaim = Boolean(user?.org);

  const organizationsQuery = useOrganizations(
    {},
    {
      pollWhileEmptyMs:
        hasOrganizationClaim && !hasGivenUp && (isSlow ? SLOW_POLL_MS : FAST_POLL_MS),
    }
  );

  const { isPending, refetch } = organizationsQuery;
  const hasOrganization = (organizationsQuery.data?.list?.length ?? 0) > 0;
  const isWaiting = hasOrganizationClaim && !isPending && !hasOrganization;

  useEffect(() => {
    if (!isWaiting || hasGivenUp) {
      return;
    }
    const now = Date.now();
    const timers = [
      window.setTimeout(() => setIsSlow(true), Math.max(0, startedAt + SLOW_DOWN_AFTER_MS - now)),
      window.setTimeout(() => setHasGivenUp(true), Math.max(0, startedAt + GIVE_UP_AFTER_MS - now)),
    ];
    return () => timers.forEach((timer) => window.clearTimeout(timer));
  }, [hasGivenUp, isWaiting, startedAt]);

  const handleRetry = useCallback(() => {
    setStartedAt(Date.now());
    setIsSlow(false);
    setHasGivenUp(false);
    void refetch();
  }, [refetch]);

  if (isPending) {
    return (
      <GateScreen
        body={<FormattedMessage {...messages.provisioningBody} />}
        title={<FormattedMessage {...messages.loadingTitle} />}
      />
    );
  }

  if (!isWaiting) {
    return <>{children}</>;
  }

  if (hasGivenUp) {
    return (
      <GateScreen
        actions={
          <Stack direction="row" spacing={2}>
            <Button onClick={handleRetry} variant="contained">
              <FormattedMessage {...messages.retry} />
            </Button>
            <Button onClick={logout}>
              <FormattedMessage {...messages.logout} />
            </Button>
          </Stack>
        }
        body={<FormattedMessage {...messages.timeoutBody} />}
        title={<FormattedMessage {...messages.timeoutTitle} />}
      />
    );
  }

  return (
    <GateScreen
      body={<FormattedMessage {...messages.provisioningBody} />}
      title={<FormattedMessage {...messages.provisioningTitle} />}
    />
  );
}
