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
  Typography,
} from '@wso2/oxygen-ui';
import { Clock, MoreVertical, Trash2 } from '@wso2/oxygen-ui-icons-react';

import type { Api } from '../../types/domain';
import { relativeTime } from '../../utils/relativeTime';
import { COMPONENT_KIND_LABEL } from './apiDisplay';
import { EnvStatusChips } from './EnvStatusChips';
import { KindIconTile } from './KindIconTile';
import { StatusPill } from './StatusPill';

type ApiRowProps = {
  component: Api;
  onOpen: (component: Api) => void;
  onDelete?: (component: Api) => void;
};

function ApiRow({ component, onOpen, onDelete }: ApiRowProps) {
  const updated = component.updatedAt || component.createdAt;
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);

  const closeMenu = (event?: React.MouseEvent) => {
    event?.stopPropagation();
    setMenuAnchor(null);
  };

  return (
    <Box
      onClick={() => onOpen(component)}
      sx={{
        alignItems: 'center',
        borderBottom: '1px solid',
        borderColor: 'divider',
        cursor: 'pointer',
        display: 'flex',
        gap: 2,
        px: 2.5,
        py: 1.75,
        transition: 'background-color 250ms',
        '&:hover': { bgcolor: 'action.hover' },
        '&:last-of-type': { borderBottom: 0 },
      }}
    >
      <KindIconTile />
      <Box sx={{ flex: 1, minWidth: 140 }}>
        <Typography noWrap sx={{ fontWeight: 500 }} variant="subtitle2">
          {component.displayName}
        </Typography>
        <Typography
          color="text.secondary"
          component="div"
          noWrap
          sx={{ fontFamily: 'monospace' }}
          variant="caption"
        >
          {COMPONENT_KIND_LABEL[component.kind]}
          {component.version ? ` · v${component.version}` : ''}
        </Typography>
      </Box>
      <StatusPill status={component.status} />
      <Box
        sx={{
          alignItems: 'center',
          display: { sm: 'flex', xs: 'none' },
          flexShrink: 0,
          gap: 1,
        }}
      >
        <EnvStatusChips componentId={component.id} />
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
        {component.owner && (
          <Box sx={{ alignItems: 'center', display: 'flex', gap: 1 }}>
            <Avatar
              sx={{ fontSize: 10, fontWeight: 600, height: 20, width: 20 }}
            >
              {component.owner.charAt(0).toUpperCase()}
            </Avatar>
            <Typography color="text.secondary" noWrap variant="caption">
              {component.owner}
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
                onDelete(component);
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
  components: Api[];
  onOpen: (component: Api) => void;
  onDelete?: (component: Api) => void;
};

/** Compact row layout for APIs — the list-view counterpart of ApiCardGrid. */
export function ApiListView({
  components,
  onOpen,
  onDelete,
}: ApiListViewProps) {
  return (
    <Card data-testid="api-list-view" variant="outlined">
      {components.map((component) => (
        <ApiRow
          component={component}
          key={component.id}
          onDelete={onDelete}
          onOpen={onOpen}
        />
      ))}
    </Card>
  );
}
