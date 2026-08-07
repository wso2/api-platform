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
import { Navigate, useNavigate } from 'react-router-dom';

import { useOrganizations } from '../../api/hooks/useMvpQueries';
import { EmptyState, ErrorState, LoadingState } from '../../components/StateViews';
import { routes } from '../../routes/paths';
import { useAuth } from '../auth/AuthProvider';

export function OrganizationRedirectPage() {
  const organizationsQuery = useOrganizations();

  if (organizationsQuery.isLoading) {
    return <LoadingState label="Loading organization" />;
  }

  if (organizationsQuery.error) {
    return (
      <ErrorState
        title="Unable to load organizations"
        message="Refresh the page or contact support if this continues."
      />
    );
  }

  const organization = organizationsQuery.data?.[0];

  if (!organization) {
    return (
      <EmptyState
        title="No organizations found"
        description="Your account is authenticated, but it is not associated with an organization."
      />
    );
  }

  return <Navigate to={routes.organizationHome(organization.handle)} replace />;
}

export function UnauthorizedPage() {
  const navigate = useNavigate();
  const auth = useAuth();

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.Header>Unauthorized</PageTitle.Header>
        <PageTitle.SubHeader>
          You do not have permission to access this console.
        </PageTitle.SubHeader>
      </PageTitle>
      <Stack direction="row" spacing={2}>
        <Button variant="contained" onClick={() => navigate(routes.login)}>
          Sign in again
        </Button>
        <Button onClick={auth.logout}>Clear session</Button>
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
        <PageTitle.Header>Session expired</PageTitle.Header>
        <PageTitle.SubHeader>
          Your session has expired. Sign in again to continue.
        </PageTitle.SubHeader>
      </PageTitle>
      <Button
        variant="contained"
        onClick={() => {
          auth.logout();
          navigate(routes.login);
        }}
      >
        Sign in
      </Button>
    </PageContent>
  );
}

export function ServerErrorPage() {
  return (
    <PageContent>
      <ErrorState
        title="Server error"
        message="The console could not complete the request. Try again shortly."
      />
    </PageContent>
  );
}

export function NotFoundPage() {
  const navigate = useNavigate();

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.Header>Page not found</PageTitle.Header>
        <PageTitle.SubHeader>
          This route is not part of the API Platform MVP.
        </PageTitle.SubHeader>
      </PageTitle>
      <Typography color="text.secondary">
        Legacy routes that are outside the MVP are intentionally not exposed.
      </Typography>
      <Button sx={{ mt: 3 }} variant="contained" onClick={() => navigate('/')}>
        Go to console home
      </Button>
    </PageContent>
  );
}
