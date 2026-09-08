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

import { Avatar, Box, IconButton, ListingTable, Stack, Tooltip, Typography } from '@wso2/oxygen-ui';
import { Clock, Trash2 } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import type { Project } from '@/api/resources/projects';
import { useRestApis } from '@/api/resources/restApis';
import { relativeTime } from '@/utils/relativeTime';

type ProjectsGridProps = {
  projects: Project[];
  onOpen: (project: Project) => void;
  onDelete?: (project: Project) => void;
};

const messages = defineMessages({
  apiCount: {
    id: 'project.list.table.apiCount',
    defaultMessage: '{count, plural, one {# API} other {# APIs}}',
  },
  apiCountLoading: {
    id: 'project.list.table.apiCountLoading',
    defaultMessage: '… APIs',
  },
  apiCountUnavailable: {
    id: 'project.list.table.apiCountUnavailable',
    defaultMessage: 'API count unavailable',
  },
  delete: {
    id: 'project.list.table.delete',
    defaultMessage: 'Delete {name}',
  },
  neverUpdated: {
    id: 'project.list.table.neverUpdated',
    defaultMessage: 'Not updated yet',
  },
  updatedAt: {
    id: 'project.list.table.updatedAt',
    defaultMessage: 'Updated {relative}',
  },
});

function ProjectRow({
  onDelete,
  onOpen,
  project,
}: Pick<ProjectsGridProps, 'onDelete' | 'onOpen'> & { project: Project }) {
  const intl = useIntl();
  const apisQuery = useRestApis({}, { projectId: project.id });
  const apiCount = apisQuery.data?.pagination?.total ?? apisQuery.data?.count;
  const initial = project.displayName.trim().charAt(0).toUpperCase() || '?';

  return (
    <ListingTable.Row
      hover
      onClick={() => onOpen(project)}
      sx={{ cursor: 'pointer', '& > td': { py: 1.5 } }}
    >
      <ListingTable.Cell>
        <Stack alignItems="center" direction="row" spacing={1.5} sx={{ minWidth: 0 }}>
          <Avatar sx={{ bgcolor: 'primary.main', color: 'primary.contrastText' }}>{initial}</Avatar>
          <Box sx={{ minWidth: 0 }}>
            <Typography noWrap sx={{ fontWeight: 600 }} variant="body1">
              {project.displayName}
            </Typography>
            {project.description && (
              <Typography color="text.secondary" noWrap variant="body2">
                {project.description}
              </Typography>
            )}
          </Box>
        </Stack>
      </ListingTable.Cell>
      <ListingTable.Cell sx={{ whiteSpace: 'nowrap', width: 120 }}>
        <Typography color="text.secondary" variant="body2">
          {apisQuery.isLoading ? (
            <FormattedMessage {...messages.apiCountLoading} />
          ) : apiCount === undefined ? (
            <FormattedMessage {...messages.apiCountUnavailable} />
          ) : (
            <FormattedMessage {...messages.apiCount} values={{ count: apiCount }} />
          )}
        </Typography>
      </ListingTable.Cell>
      <ListingTable.Cell sx={{ whiteSpace: 'nowrap', width: 180 }}>
        <Stack alignItems="center" direction="row" spacing={0.75}>
          <Clock size={16} />
          <Typography color="text.secondary" variant="body2">
            {project.updatedAt ? (
              <FormattedMessage
                {...messages.updatedAt}
                values={{ relative: relativeTime(project.updatedAt) }}
              />
            ) : (
              <FormattedMessage {...messages.neverUpdated} />
            )}
          </Typography>
        </Stack>
      </ListingTable.Cell>
      <ListingTable.Cell align="right" sx={{ width: 56 }}>
        {onDelete && (
          <Tooltip title={intl.formatMessage(messages.delete, { name: project.displayName })}>
            <IconButton
              aria-label={intl.formatMessage(messages.delete, { name: project.displayName })}
              color="error"
              onClick={(event) => {
                event.stopPropagation();
                onDelete(project);
              }}
              size="small"
            >
              <Trash2 size={18} />
            </IconButton>
          </Tooltip>
        )}
      </ListingTable.Cell>
    </ListingTable.Row>
  );
}

/** Renders only the projects provided; paging is handled by the page. */
export function ProjectsGrid({ projects, onOpen, onDelete }: ProjectsGridProps) {
  return (
    <ListingTable.Provider>
      <ListingTable.Container sx={{ border: 0, borderRadius: 0 }}>
        <ListingTable>
          <ListingTable.Body>
            {projects.map((project) => (
              <ProjectRow key={project.id} onDelete={onDelete} onOpen={onOpen} project={project} />
            ))}
          </ListingTable.Body>
        </ListingTable>
      </ListingTable.Container>
    </ListingTable.Provider>
  );
}
