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

import React from 'react';
import { Outlet, useNavigate, useParams } from 'react-router-dom';
import { PageContent, Stack, Typography } from '@wso2/oxygen-ui';
import { useAppShell } from '../../../../contexts/AppShellContext';
import AILoader from '../../../../Components/AILoader';
import { buildOrgPath, getOrgSlug } from '../../../../utils/projectRouting';
import { FormattedMessage } from 'react-intl';

export default function OrgShell() {
  const navigate = useNavigate();
  const { orgSlug } = useParams<{ orgSlug: string }>();
  const { currentOrganization, isLoading } = useAppShell();

  if (isLoading) {
    return (
      <PageContent fullWidth>
        <Stack spacing={2} alignItems="center" sx={{ py: 6 }}>
          <AILoader />
          <Typography variant="body2" color="text.secondary">
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.projects.OrgShell.loading.organization"
              defaultMessage={'Loading organization...'}
            />
          </Typography>
        </Stack>
      </PageContent>
    );
  }

  const isMatch = currentOrganization != null && getOrgSlug(currentOrganization) === orgSlug;

  if (!isMatch) {
    const fallbackPath = buildOrgPath(currentOrganization, '/home');
    return (
      <PageContent fullWidth>
        <Stack spacing={1.5} alignItems="flex-start">
          <Typography variant="h6">
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.projects.OrgShell.organization.not.found"
              defaultMessage={'Organization not found'}
            />
          </Typography>
          <Typography variant="body2" color="text.secondary">
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.projects.OrgShell.the.selected.organization.is.unavailable.please.choose.another"
              defaultMessage={'The selected organization is unavailable.'}
            />
          </Typography>
          {fallbackPath ? (
            <Typography
              variant="body2"
              color="primary"
              sx={{ cursor: 'pointer' }}
              onClick={() => navigate(fallbackPath)}
            >
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.projects.OrgShell.go.to.projects"
                defaultMessage={'Go to projects'}
              />
            </Typography>
          ) : null}
        </Stack>
      </PageContent>
    );
  }

  return <Outlet key={currentOrganization?.id} />;
}
