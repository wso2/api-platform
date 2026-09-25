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

import { useState, type MouseEvent } from 'react';
import {
  alpha,
  Box,
  Card,
  Divider,
  IconButton,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { Boxes, Clock, Layers, MoreVertical, Rocket, Trash2 } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { useGraphQLApis } from '@/api/resources/graphqlApis';
import { useRestApis } from '@/api/resources/restApis';
import type { Project } from '@/api/resources/projects';
import { relativeTime } from '@/utils/relativeTime';
import { openableProps } from '@/components/openable';
import { focusRingSx, interactiveCardSx } from '@/theme';

/** Tint strength of the metadata strip per color scheme. */
const METADATA_TINT = { dark: 0.08, light: 0.06 } as const;

type ProjectCardProps = {
  project: Project;
  onOpen: (project: Project) => void;
  onDelete?: (project: Project) => void;
};

const messages = defineMessages({
  actionsLabel: {
    id: 'project.card.actionsLabel',
    defaultMessage: 'Project actions',
    description: 'Accessible label for the button opening the card overflow menu.',
  },
  apiCount: {
    id: 'project.card.apiCount',
    defaultMessage: '{count, plural, one {# API} other {# APIs}}',
  },
  apiCountLoading: {
    id: 'project.card.apiCountLoading',
    defaultMessage: '… APIs',
    description: 'Placeholder shown while the API count is still loading.',
  },
  defaultBadge: {
    id: 'project.card.defaultBadge',
    defaultMessage: 'DEFAULT',
    description: 'Badge marking the organization’s default project.',
  },
  delete: {
    id: 'project.card.delete',
    defaultMessage: 'Delete',
  },
  deployedCount: {
    id: 'project.card.deployedCount',
    defaultMessage: '{count} deployed',
  },
  deployedCountLoading: {
    id: 'project.card.deployedCountLoading',
    defaultMessage: '… deployed',
    description: 'Placeholder shown while the deployed count is still loading.',
  },
  fallbackDescription: {
    id: 'project.card.fallbackDescription',
    defaultMessage: 'No decsription',
    description: 'Shown in place of a description when the project has none.',
  },
  neverUpdated: {
    id: 'project.card.neverUpdated',
    defaultMessage: 'Not updated yet',
  },
  updatedAt: {
    id: 'project.card.updatedAt',
    defaultMessage: 'Updated {relative}',
    description: 'Footer timestamp; {relative} is a phrase such as "3 hours ago".',
  },
});

export function ProjectCard({ project, onOpen, onDelete }: ProjectCardProps) {
  const intl = useIntl();
  const stopCardClick = (event: MouseEvent) => event.stopPropagation();
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);

  const closeMenu = (event?: MouseEvent) => {
    event?.stopPropagation();
    setMenuAnchor(null);
  };

  // Scoped to this card's project rather than the route's, so the counts belong
  // to the card the user is looking at and not the project they are currently in.
  const apisQuery = useRestApis({}, { projectId: project.id });
  const graphqlApisQuery = useGraphQLApis({}, { projectId: project.id });
  const isLoading = apisQuery.isLoading || graphqlApisQuery.isLoading;
  const restTotal = apisQuery.data?.pagination?.total ?? apisQuery.data?.count;
  const graphqlTotal = graphqlApisQuery.data?.pagination?.total ?? graphqlApisQuery.data?.count;
  const apiCount = (restTotal ?? 0) + (graphqlTotal ?? 0);
  // GraphQL APIs have no lifecycle status (see GraphQLAPIListItem) — this
  // count is REST-only, same as `ProjectStatistics`'s published/created split.
  const deployedCount = apisQuery.data?.list?.filter(
    (api) => api.lifeCycleStatus === 'PUBLISHED',
  ).length;

  return (
    <Card
      // elevation={0}
      {...openableProps(intl, project.displayName, () => onOpen(project))}
      sx={(theme) => ({
        ...interactiveCardSx,
        ...focusRingSx(theme),
        display: 'flex',
        flexDirection: 'column',
        height: '100%',
        overflow: 'hidden',
      })}
    >
      <Box sx={{ flexGrow: 1, p: 2.5 }}>
        <Stack alignItems="center" direction="row" spacing={1.75}>
          <Box
            sx={() => ({
              alignItems: 'center',
              bgcolor: `primary.light`,
              color: `primary.contrastText`,
              borderRadius: 2,
              display: 'flex',
              flex: 'none',
              height: 46,
              justifyContent: 'center',
              width: 46,
            })}
          >
            <Layers size={22} />
          </Box>
          <Box sx={{ minWidth: 0 }}>
            <Stack alignItems="center" direction="row" spacing={1}>
              <Typography noWrap sx={{ fontWeight: 600 }} variant="subtitle1">
                {project.displayName}
              </Typography>
            </Stack>
            <Typography
              color="text.secondary"
              noWrap
              sx={{ mt: 0.25, fontSize: '0.7rem' }}
              variant="body2"
            >
              {project.description || <FormattedMessage {...messages.fallbackDescription} />}
            </Typography>
          </Box>
        </Stack>

        {/* info strip — real project metadata */}
        <Stack
          direction="row"
          spacing={2}
          sx={[
            (theme) => ({
              alignItems: 'center',
              bgcolor: alpha(theme.palette.common.black, METADATA_TINT.light),
              borderRadius: 1,
              color: 'text.secondary',
              mt: 2.25,
              px: 1.75,
              py: 1.25,
            }),
            // Emitted under the dark color-scheme selector, so it follows the
            // theme the user is actually on. Must come last in the array —
            // `applyStyles` returns a nested selector, not a flat value.
            (theme) =>
              theme.applyStyles('dark', {
                bgcolor: alpha(theme.palette.common.white, METADATA_TINT.dark),
              }),
          ]}
        >
          <Stack alignItems="center" direction="row" spacing={0.75} sx={{ minWidth: 0 }}>
            <Boxes size={16} />
            <Typography noWrap variant="body2">
              {isLoading ? (
                <FormattedMessage {...messages.apiCountLoading} />
              ) : (
                <FormattedMessage {...messages.apiCount} values={{ count: apiCount }} />
              )}
            </Typography>
          </Stack>
          <Stack alignItems="center" direction="row" spacing={0.75} sx={{ minWidth: 0 }}>
            <Rocket size={16} />
            <Typography noWrap variant="body2">
              {apisQuery.isLoading ? (
                <FormattedMessage {...messages.deployedCountLoading} />
              ) : (
                <FormattedMessage
                  {...messages.deployedCount}
                  values={{ count: deployedCount ?? 0 }}
                />
              )}
            </Typography>
          </Stack>
        </Stack>
      </Box>

      <Divider />

      {/* footer */}
      <Box
        sx={{
          alignItems: 'center',
          display: 'flex',
          gap: 1,
          px: 2,
          py: 1.25,
        }}
      >
        <Clock size={14} />
        <Typography color="text.secondary" variant="caption">
          {project.updatedAt ? (
            <FormattedMessage
              {...messages.updatedAt}
              values={{ relative: relativeTime(project.updatedAt) }}
            />
          ) : (
            <FormattedMessage {...messages.neverUpdated} />
          )}
        </Typography>
        <Box sx={{ flex: 1 }} />
        {onDelete && (
          <>
            <Tooltip title={intl.formatMessage(messages.actionsLabel)}>
              <IconButton
                aria-label={intl.formatMessage(messages.actionsLabel)}
                onClick={(event) => {
                  event.stopPropagation();
                  setMenuAnchor(event.currentTarget);
                }}
                size="small"
              >
                <MoreVertical size={16} />
              </IconButton>
            </Tooltip>
            <Menu
              anchorEl={menuAnchor}
              onClick={stopCardClick}
              onClose={() => closeMenu()}
              open={Boolean(menuAnchor)}
            >
              <MenuItem
                onClick={(event) => {
                  closeMenu(event);
                  onDelete(project);
                }}
                sx={{ color: 'error.main' }}
              >
                <ListItemIcon sx={{ color: 'inherit' }}>
                  <Trash2 size={16} />
                </ListItemIcon>
                <ListItemText>
                  <FormattedMessage {...messages.delete} />
                </ListItemText>
              </MenuItem>
            </Menu>
          </>
        )}
      </Box>
    </Card>
  );
}
