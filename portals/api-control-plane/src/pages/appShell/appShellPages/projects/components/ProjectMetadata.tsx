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
import { Avatar, Box, Divider, IconButton, Stack, Tooltip, Typography } from '@wso2/oxygen-ui';
import { Layers, Settings } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { useNavigate } from 'react-router-dom';

import type { Project } from '@/api/resources/projects';
import { routes } from '@/routes/paths';
import { relativeTime } from '@/utils/relativeTime';

const messages = defineMessages({
  created: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.components.ProjectMetadata.created',
    defaultMessage: '<muted>Created</muted> {date}',
  },
  descriptionFallback: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.components.ProjectMetadata.descriptionFallback',
    defaultMessage: 'Add an API to this project.',
  },
  memberCount: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.components.ProjectMetadata.memberCount',
    defaultMessage: '{count} <muted>{count, plural, one {member} other {members}}</muted>',
  },
  owner: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.components.ProjectMetadata.owner',
    defaultMessage: '<muted>Owned by</muted> {owner}',
  },
  settings: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.components.ProjectMetadata.settings',
    defaultMessage: 'Project settings',
  },
  unknown: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.components.ProjectMetadata.unknown',
    defaultMessage: 'Unknown',
  },
});

type ProjectMetadataProps = {
  orgHandle: string;
  project: Project;
};

export function ProjectMetadata({ orgHandle, project }: ProjectMetadataProps) {
  const intl = useIntl();
  const navigate = useNavigate();
  const memberName = project.updatedBy;
  const memberCount = memberName ? 1 : 0;
  const owner = project.createdBy || intl.formatMessage(messages.unknown);
  const created = project.createdAt
    ? relativeTime(project.createdAt)
    : intl.formatMessage(messages.unknown);
  const muted = (chunks: ReactNode) => (
    <Box component="span" sx={{ opacity: 0.7 }}>
      {chunks}
    </Box>
  );

  return (
    <Box>
      <Stack
        alignItems={{ md: 'flex-start' }}
        direction={{ md: 'row', xs: 'column' }}
        justifyContent="space-between"
        spacing={2}
        marginTop={2}
      >
        <Stack alignItems="flex-start" direction="row" spacing={2.5} sx={{ minWidth: 0 }}>
          <Avatar
            sx={{
              bgcolor: 'primary.main',
              color: 'primary.contrastText',
              height: 64,
              width: 64,
            }}
            variant="rounded"
          >
            <Layers size={28} />
          </Avatar>
          <Stack spacing={1.5} sx={{ minWidth: 0 }}>
            <Box sx={{ minWidth: 0 }}>
              <Typography noWrap sx={{ fontWeight: 800 }} variant="h2">
                {project.displayName}
              </Typography>
              <Typography color="text.secondary" variant="body1">
                {project.description || <FormattedMessage {...messages.descriptionFallback} />}
              </Typography>
            </Box>

            <Stack
              alignItems="center"
              direction="row"
              divider={<Divider flexItem orientation="vertical" />}
              flexWrap="wrap"
              spacing={2}
            >
              <Typography color="text.secondary" variant="body2">
                <FormattedMessage {...messages.owner} values={{ muted, owner }} />
              </Typography>
              <Typography color="text.secondary" variant="body2">
                <FormattedMessage {...messages.created} values={{ date: created, muted }} />
              </Typography>
              <Stack alignItems="center" direction="row" spacing={1}>
                {memberName && (
                  <Tooltip title={memberName}>
                    <Stack direction="row" sx={{ '& > * + *': { ml: -1 } }}>
                      <Avatar
                        sx={{
                          bgcolor: 'primary.main',
                          color: 'primary.contrastText',
                          height: 28,
                          width: 28,
                        }}
                      >
                        {memberName.charAt(0).toUpperCase()}
                      </Avatar>
                      <Avatar
                        aria-hidden="true"
                        sx={{
                          bgcolor: 'action.hover',
                          height: 28,
                          width: 28,
                        }}
                      />
                    </Stack>
                  </Tooltip>
                )}
                <Typography color="text.secondary" variant="body2">
                  <FormattedMessage
                    {...messages.memberCount}
                    values={{ count: memberCount, muted }}
                  />
                </Typography>
              </Stack>
            </Stack>
          </Stack>
        </Stack>

        <Tooltip title={intl.formatMessage(messages.settings)}>
          <IconButton
            aria-label={intl.formatMessage(messages.settings)}
            onClick={() => navigate(routes.projectSettings(orgHandle, project.id))}
            sx={{ flexShrink: 0 }}
          >
            <Settings size={20} />
          </IconButton>
        </Tooltip>
      </Stack>
    </Box>
  );
}
