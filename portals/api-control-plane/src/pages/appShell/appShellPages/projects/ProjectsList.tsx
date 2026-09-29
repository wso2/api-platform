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

import { Avatar, Box, Card, IconButton, Stack, Tooltip, Typography } from '@wso2/oxygen-ui';
import { Clock, Layers, Trash2 } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import type { Project } from '@/api/resources/projects';
import { openableProps } from '@/components/openable';
import { focusRingSx } from '@/theme';
import { useFormatters } from '@/i18n/useFormatters';
import { useCan } from '@/permissions/useCan';

const AVATAR_SIZE = 40;
const AVATAR_ICON_SIZE = 22;

const messages = defineMessages({
  deleteLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectsList.deleteLabel',
    defaultMessage: 'Delete {name}',
    description: 'Accessible label for the delete button on a project row.',
  },
  deleteTooltip: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectsList.deleteTooltip',
    defaultMessage: 'Delete project',
  },
  fallbackDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectsList.fallbackDescription',
    defaultMessage: 'No description',
    description: 'Shown in place of a description when the project has none.',
  },
  neverUpdated: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectsList.neverUpdated',
    defaultMessage: 'Not updated yet',
  },
  projectColumn: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectsList.projectColumn',
    defaultMessage: 'Project',
    description: 'Column header over the project name and description.',
  },
  updatedAt: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectsList.updatedAt',
    defaultMessage: 'Updated {relative}',
    description: 'Row timestamp; {relative} is a phrase such as "3 hours ago".',
  },
  updatedColumn: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectsList.updatedColumn',
    defaultMessage: 'Updated',
    description: 'Column header over the last-updated timestamp.',
  },
});

/**
 * Shared by the header and every row, so a column can never drift between the
 * label and the cells under it. The timestamp column is fixed-width to keep the
 * delete button in a straight line down the page.
 */
const rowGridSx = {
  alignItems: 'center',
  display: 'grid',
  gap: 2,
  gridTemplateColumns: {
    xs: 'minmax(0, 1fr) auto',
    md: 'minmax(0, 1fr) 260px',
  },
} as const;

type ProjectRowProps = {
  project: Project;
  onOpen: (project: Project) => void;
  onDelete?: (project: Project) => void;
};

/** One project as a row: identity on the left, when it last changed on the right. */
function ProjectRow({ project, onOpen, onDelete }: ProjectRowProps) {
  const intl = useIntl();
  const canDelete = useCan('DeleteProject');
  const { relativeTime } = useFormatters();

  return (
    <Box
      {...openableProps(intl, project.displayName, () => onOpen(project))}
      sx={(theme) => ({
        borderBottom: `${theme.border.width} ${theme.border.style}`,
        borderColor: 'divider',
        cursor: 'pointer',
        px: 2.5,
        py: 1.75,
        transition: theme.transitions.create('background-color'),
        ...rowGridSx,
        ...focusRingSx(theme),
        // Keyboard users get the action the same way pointer users do.
        '&:focus-within .project-delete-action, &:hover .project-delete-action': { opacity: 1 },
        '&:hover': { bgcolor: 'action.hover' },
        '&:last-of-type': { borderBottom: 0 },
      })}
    >
      {/* `minWidth: 0` is what lets a long name truncate instead of widening the grid. */}
      <Stack alignItems="center" direction="row" spacing={1.75} sx={{ minWidth: 0 }}>
        <Avatar
          sx={{
            bgcolor: 'primary.light',
            color: 'primary.contrastText',
            flexShrink: 0,
            height: AVATAR_SIZE,
            width: AVATAR_SIZE,
          }}
          variant="rounded"
        >
          <Layers size={AVATAR_ICON_SIZE} />
        </Avatar>
        <Box sx={{ minWidth: 0 }}>
          <Typography component="div" noWrap sx={{ fontWeight: 600 }} variant="subtitle2">
            {project.displayName}
          </Typography>
          <Typography color="text.secondary" noWrap sx={{ fontSize: '0.7rem' }} variant="caption">
            {project.description || <FormattedMessage {...messages.fallbackDescription} />}
          </Typography>
        </Box>
      </Stack>

      <Stack alignItems="center" direction="row" justifyContent="space-between" spacing={1}>
        <Stack
          alignItems="center"
          direction="row"
          spacing={1}
          sx={{ color: 'text.secondary', flexShrink: 0 }}
        >
          <Clock size={14} />
          <Typography color="text.secondary" noWrap variant="caption">
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
        {onDelete && canDelete && (
          <Tooltip title={intl.formatMessage(messages.deleteTooltip)}>
            <IconButton
              aria-label={intl.formatMessage(messages.deleteLabel, { name: project.displayName })}
              className="project-delete-action"
              color="error"
              onClick={(event) => {
                event.stopPropagation();
                onDelete(project);
              }}
              size="small"
              sx={{
                flexShrink: 0,
                mr: -1,
                // Hidden until hover on a pointer-sized viewport; always shown
                // where there is no hover to reveal it.
                opacity: { md: 0, xs: 1 },
                transition: 'opacity 150ms ease',
              }}
            >
              <Trash2 size={18} />
            </IconButton>
          </Tooltip>
        )}
      </Stack>
    </Box>
  );
}

type ProjectsListProps = {
  projects: Project[];
  onOpen: (project: Project) => void;
  onDelete?: (project: Project) => void;
};

/** Table layout for projects, the list-view counterpart of `ProjectsGrid`. */
export function ProjectsList({ projects, onOpen, onDelete }: ProjectsListProps) {
  return (
    <Card data-testid="project-list-view" variant="outlined">
      <Box sx={{ ...rowGridSx, bgcolor: 'action.hover', px: 2.5, py: 1.25 }}>
        <Typography color="text.secondary" sx={{ fontWeight: 700 }} variant="caption">
          <FormattedMessage {...messages.projectColumn} />
        </Typography>
        <Typography color="text.secondary" sx={{ fontWeight: 700 }} variant="caption">
          <FormattedMessage {...messages.updatedColumn} />
        </Typography>
      </Box>
      {projects.map((project) => (
        <ProjectRow key={project.id} onDelete={onDelete} onOpen={onOpen} project={project} />
      ))}
    </Card>
  );
}
