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

import { Button, Divider, Stack, Typography } from '@wso2/oxygen-ui';
import { FileUp, UserPlus } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage } from 'react-intl';
import { useNavigate } from 'react-router-dom';

import { routes } from '@/routes/paths';

const messages = defineMessages({
  importApi: {
    id: 'apiControlPlane.projects.ProjectQuickActions.importApi',
    defaultMessage: 'Import an OpenAPI definition',
  },
  inviteMember: {
    id: 'apiControlPlane.projects.ProjectQuickActions.inviteMember',
    defaultMessage: 'Invite a team member',
  },
  title: {
    id: 'apiControlPlane.projects.ProjectQuickActions.title',
    defaultMessage: 'Quick actions',
  },
});

type ProjectQuickActionsProps = { orgHandle: string; projectId: string };

export function ProjectQuickActions({ orgHandle, projectId }: ProjectQuickActionsProps) {
  const navigate = useNavigate();

  return (
    <Stack spacing={0.5}>
      <Divider />
      <Stack alignItems="center" direction="row" flexWrap="wrap" spacing={2}>
        <Typography color="text.secondary" sx={{ textTransform: 'uppercase' }} variant="caption">
          <FormattedMessage {...messages.title} />
        </Typography>
        <Stack
          alignItems="center"
          direction="row"
          divider={<Divider flexItem orientation="vertical" />}
          flexWrap="wrap"
          spacing={1}
        >
          <Button
            onClick={() => navigate(routes.newApi(orgHandle, projectId))}
            startIcon={<FileUp size={16} />}
            variant="text"
          >
            <FormattedMessage {...messages.importApi} />
          </Button>
          <Button
            onClick={() => navigate(routes.projectSettings(orgHandle, projectId))}
            startIcon={<UserPlus size={16} />}
            variant="text"
          >
            <FormattedMessage {...messages.inviteMember} />
          </Button>
        </Stack>
      </Stack>
    </Stack>
  );
}
