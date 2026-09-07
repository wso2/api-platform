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

import { Button, PageContent, PageTitle, Stack, Typography } from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { Navigate, useNavigate } from 'react-router-dom';

import { useOrganizations } from '@/api/resources/organizations';
import { EmptyState, ErrorState, LoadingState } from '@/components/StateViews';
import { routes } from '@/routes/paths';
import { useAuth } from '@/contexts/auth/AuthProvider';

/**
 * Message ids are keyed by the *component*, not by this file: five unrelated
 * pages share one module, so a single `SystemPages` namespace would collide
 * five `title` slugs against each other and give translators no idea which
 * screen they were looking at.
 */
const organizationRedirectMessages = defineMessages({
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.system.OrganizationRedirectPage.loading',
    defaultMessage: 'Loading organization',
    description: "Shown while the signed-in user's organizations are being fetched.",
  },
  errorTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.system.OrganizationRedirectPage.errorTitle',
    defaultMessage: 'Unable to load organizations',
  },
  errorMessage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.system.OrganizationRedirectPage.errorMessage',
    defaultMessage: 'Refresh the page or contact support if this continues.',
  },
  emptyTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.system.OrganizationRedirectPage.emptyTitle',
    defaultMessage: 'No organizations found',
  },
  emptyDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.system.OrganizationRedirectPage.emptyDescription',
    defaultMessage: 'Your account is authenticated, but it is not associated with an organization.',
  },
});

const unauthorizedMessages = defineMessages({
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.system.UnauthorizedPage.title',
    defaultMessage: 'Unauthorized',
    description: 'Page heading shown when the signed-in account may not use this console.',
  },
  subtitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.system.UnauthorizedPage.subtitle',
    defaultMessage: 'You do not have permission to access this console.',
  },
  signInAgain: {
    id: 'apiControlPlane.pages.appShell.appShellPages.system.UnauthorizedPage.signInAgain',
    defaultMessage: 'Sign in again',
    description: 'Button returning to the login page. Verb phrase.',
  },
  clearSession: {
    id: 'apiControlPlane.pages.appShell.appShellPages.system.UnauthorizedPage.clearSession',
    defaultMessage: 'Clear session',
    description: 'Button that logs the user out, discarding the current session. Verb phrase.',
  },
});

const sessionExpiredMessages = defineMessages({
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.system.SessionExpiredPage.title',
    defaultMessage: 'Session expired',
  },
  subtitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.system.SessionExpiredPage.subtitle',
    defaultMessage: 'Your session has expired. Sign in again to continue.',
  },
  signIn: {
    id: 'apiControlPlane.pages.appShell.appShellPages.system.SessionExpiredPage.signIn',
    defaultMessage: 'Sign in',
    description: 'Button that logs out and returns to the login page. Verb phrase.',
  },
});

const serverErrorMessages = defineMessages({
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.system.ServerErrorPage.title',
    defaultMessage: 'Server error',
  },
  message: {
    id: 'apiControlPlane.pages.appShell.appShellPages.system.ServerErrorPage.message',
    defaultMessage: 'The console could not complete the request. Try again shortly.',
  },
});

const notFoundMessages = defineMessages({
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.system.NotFoundPage.title',
    defaultMessage: 'Page not found',
  },
  subtitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.system.NotFoundPage.subtitle',
    defaultMessage: 'This route is not part of the API Platform MVP.',
    description: '"API Platform" is a product name — leave it untranslated.',
  },
  body: {
    id: 'apiControlPlane.pages.appShell.appShellPages.system.NotFoundPage.body',
    defaultMessage: 'Legacy routes that are outside the MVP are intentionally not exposed.',
  },
  goHome: {
    id: 'apiControlPlane.pages.appShell.appShellPages.system.NotFoundPage.goHome',
    defaultMessage: 'Go to console home',
    description: 'Button returning to the root of the console. Verb phrase.',
  },
});

export function OrganizationRedirectPage() {
  const intl = useIntl();
  const organizationsQuery = useOrganizations();

  if (organizationsQuery.isPending) {
    return <LoadingState label={intl.formatMessage(organizationRedirectMessages.loading)} />;
  }

  if (organizationsQuery.error) {
    return (
      <ErrorState
        title={intl.formatMessage(organizationRedirectMessages.errorTitle)}
        message={intl.formatMessage(organizationRedirectMessages.errorMessage)}
      />
    );
  }

  const organization = organizationsQuery.data?.list?.[0];

  if (!organization) {
    return (
      <EmptyState
        title={intl.formatMessage(organizationRedirectMessages.emptyTitle)}
        description={intl.formatMessage(organizationRedirectMessages.emptyDescription)}
      />
    );
  }

  return <Navigate to={routes.organizationHome(organization.id)} replace />;
}

export function UnauthorizedPage() {
  const navigate = useNavigate();
  const auth = useAuth();

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.Header>
          <FormattedMessage {...unauthorizedMessages.title} />
        </PageTitle.Header>
        <PageTitle.SubHeader>
          <FormattedMessage {...unauthorizedMessages.subtitle} />
        </PageTitle.SubHeader>
      </PageTitle>
      <Stack direction="row" spacing={2}>
        <Button variant="contained" onClick={() => navigate(routes.login)}>
          <FormattedMessage {...unauthorizedMessages.signInAgain} />
        </Button>
        <Button onClick={auth.logout}>
          <FormattedMessage {...unauthorizedMessages.clearSession} />
        </Button>
      </Stack>
    </PageContent>
  );
}

export function SessionExpiredPage() {
  const navigate = useNavigate();
  const auth = useAuth();

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.Header>
          <FormattedMessage {...sessionExpiredMessages.title} />
        </PageTitle.Header>
        <PageTitle.SubHeader>
          <FormattedMessage {...sessionExpiredMessages.subtitle} />
        </PageTitle.SubHeader>
      </PageTitle>
      <Button
        variant="contained"
        onClick={() => {
          auth.logout();
          navigate(routes.login);
        }}
      >
        <FormattedMessage {...sessionExpiredMessages.signIn} />
      </Button>
    </PageContent>
  );
}

export function ServerErrorPage() {
  const intl = useIntl();

  return (
    <PageContent>
      <ErrorState
        title={intl.formatMessage(serverErrorMessages.title)}
        message={intl.formatMessage(serverErrorMessages.message)}
      />
    </PageContent>
  );
}

export function NotFoundPage() {
  const navigate = useNavigate();

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.Header>
          <FormattedMessage {...notFoundMessages.title} />
        </PageTitle.Header>
        <PageTitle.SubHeader>
          <FormattedMessage {...notFoundMessages.subtitle} />
        </PageTitle.SubHeader>
      </PageTitle>
      <Typography color="text.secondary">
        <FormattedMessage {...notFoundMessages.body} />
      </Typography>
      <Button sx={{ mt: 3 }} variant="contained" onClick={() => navigate('/')}>
        <FormattedMessage {...notFoundMessages.goHome} />
      </Button>
    </PageContent>
  );
}
