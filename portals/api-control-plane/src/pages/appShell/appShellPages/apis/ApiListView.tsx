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

import { useState } from 'react';
import {
  Avatar,
  Box,
  Card,
  IconButton,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { Clock, MoreVertical, Trash2 } from '@wso2/oxygen-ui-icons-react';

import type { RestApi } from '../../../../api/resources/restApis';
import { relativeTime } from '../../../../utils/relativeTime';
import { KindIconTile } from '../../../../components/cards/KindIconTile';
import {
  DeploymentStateLabel,
  GatewayChips,
  LifecycleChip,
} from './components/RestApiChips';
import { apiKindLabel, useApiDeploymentState } from './restApiDisplay';

type ApiRowProps = {
  api: RestApi;
  onOpen: (api: RestApi) => void;
  onDelete?: (api: RestApi) => void;
};

function ApiRow({ api, onOpen, onDelete }: ApiRowProps) {
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);
  const { gatewayIds, state } = useApiDeploymentState(api.id);

  const updated = api.updatedAt || api.createdAt;

  const closeMenu = (event?: React.MouseEvent) => {
    event?.stopPropagation();
    setMenuAnchor(null);
  };

  return (
    <Box
      onClick={() => onOpen(api)}
      sx={(theme) => ({
        alignItems: 'center',
        borderBottom: `${theme.border.width} ${theme.border.style}`,
        borderColor: 'divider',
        cursor: 'pointer',
        display: 'flex',
        gap: 2,
        px: 2.5,
        py: 1.75,
        transition: 'background-color 250ms',
        '&:hover': { bgcolor: 'action.hover' },
        '&:last-of-type': { borderBottom: 0 },
      })}
    >
      <Tooltip placement="left" title={apiKindLabel(api.kind)}>
        <Box sx={{ display: 'inline-flex' }}>
          <KindIconTile />
        </Box>
      </Tooltip>
      <Box sx={{ flex: 1, minWidth: 140 }}>
        <Typography noWrap sx={{ fontWeight: 500 }} variant="subtitle2">
          {api.displayName}
        </Typography>
        <Typography
          color="text.secondary"
          component="div"
          noWrap
          sx={{ fontFamily: 'monospace' }}
          variant="caption"
        >
          {api.context}
          {api.version ? ` · v${api.version}` : ''}
        </Typography>
      </Box>
      <LifecycleChip status={api.lifeCycleStatus} />
      <Box
        sx={{
          alignItems: 'center',
          display: { sm: 'flex', xs: 'none' },
          flexShrink: 0,
          gap: 1,
        }}
      >
        <DeploymentStateLabel state={state} />
        <GatewayChips gatewayIds={gatewayIds} />
      </Box>
      <Box
        sx={{
          alignItems: 'center',
          color: 'text.secondary',
          display: { md: 'flex', xs: 'none' },
          flexShrink: 0,
          gap: 2.5,
          ml: 'auto',
        }}
      >
        {api.createdBy && (
          <Box sx={{ alignItems: 'center', display: 'flex', gap: 1 }}>
            <Avatar
              sx={{ fontSize: 10, fontWeight: 600, height: 20, width: 20 }}
            >
              {api.createdBy.charAt(0).toUpperCase()}
            </Avatar>
            <Typography color="text.secondary" noWrap variant="caption">
              {api.createdBy}
            </Typography>
          </Box>
        )}
        <Box sx={{ alignItems: 'center', display: 'flex', gap: 0.75 }}>
          <Clock size={12} />
          <Typography color="text.secondary" noWrap variant="caption">
            {updated ? relativeTime(updated) : '—'}
          </Typography>
        </Box>
      </Box>
      {onDelete && (
        <>
          <IconButton
            aria-label="API actions"
            onClick={(event) => {
              event.stopPropagation();
              setMenuAnchor(event.currentTarget);
            }}
            size="small"
            sx={{ ml: { md: 0, xs: 'auto' } }}
          >
            <MoreVertical size={18} />
          </IconButton>
          <Menu
            anchorEl={menuAnchor}
            onClose={() => closeMenu()}
            open={Boolean(menuAnchor)}
          >
            <MenuItem
              onClick={(event) => {
                closeMenu(event);
                onDelete(api);
              }}
              sx={{ color: 'error.main' }}
            >
              <ListItemIcon sx={{ color: 'inherit' }}>
                <Trash2 size={16} />
              </ListItemIcon>
              <ListItemText>Delete</ListItemText>
            </MenuItem>
          </Menu>
        </>
      )}
    </Box>
  );
}

type ApiListViewProps = {
  apis: RestApi[];
  onOpen: (api: RestApi) => void;
  onDelete?: (api: RestApi) => void;
};

/** Compact row layout for APIs — the list-view counterpart of ApiCardGrid. */
export function ApiListView({ apis, onOpen, onDelete }: ApiListViewProps) {
  return (
    <Card data-testid="api-list-view" variant="outlined">
      {apis.map((api) => (
        <ApiRow
          api={api}
          key={api.id}
          onDelete={onDelete}
          onOpen={onOpen}
        />
      ))}
    </Card>
  );
}
