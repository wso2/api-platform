import { useState } from 'react';
import {
  alpha,
  Avatar,
  Box,
  IconButton,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import {
  Boxes,
  Clock,
  MoreVertical,
  Trash2,
} from '@wso2/oxygen-ui-icons-react';

import type { Api, ApiStatus } from '../../types/domain';
import { relativeTime } from '../../utils/relativeTime';
import { COMPONENT_KIND_LABEL, componentStatusColor } from './apiDisplay';
import { EnvStatusChips } from './EnvStatusChips';
import { KindIconTile } from './KindIconTile';

const STATUS_LABEL: Record<ApiStatus, string> = {
  ACTIVE: 'Active',
  PENDING: 'Pending',
  FAILED: 'Failed',
  DRAFT: 'Draft',
};

/** Shared square-chip styling from the gateway card. */
const chipSx = {
  alignItems: 'center',
  bgcolor: 'action.hover',
  border: '1px solid',
  borderColor: 'divider',
  borderRadius: 1,
  color: 'text.secondary',
  display: 'inline-flex',
  fontSize: 12,
  fontWeight: 500,
  gap: 0.75,
  px: 1.25,
  py: 0.5,
} as const;

type ApiCardProps = {
  component: Api;
  onOpen: (component: Api) => void;
  onDelete?: (component: Api) => void;
};

/** API card styled to match the gateway card family (GatewaysPage). */
export function ApiCard({ component, onOpen, onDelete }: ApiCardProps) {
  const updated = component.updatedAt || component.createdAt;
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);

  const statusColor = componentStatusColor(component.status);
  const statusMain =
    statusColor === 'default' ? 'text.disabled' : `${statusColor}.main`;

  const closeMenu = (event?: React.MouseEvent) => {
    event?.stopPropagation();
    setMenuAnchor(null);
  };

  return (
    <Box
      onClick={() => onOpen(component)}
      sx={{
        bgcolor: 'background.paper',
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: 2,
        cursor: 'pointer',
        display: 'flex',
        flexDirection: 'column',
        height: '100%',
        p: 2.5,
        transition: 'border-color .2s, box-shadow .2s, transform .2s',
        '&:hover': {
          borderColor: 'primary.main',
          boxShadow: 3,
          transform: 'translateY(-2px)',
        },
      }}
    >
      {/* Header: kind tile, name + handler, actions menu */}
      <Stack direction="row" spacing={1.75}>
        <KindIconTile size={46} />
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Typography noWrap sx={{ fontWeight: 600 }} variant="subtitle1">
            {component.displayName}
          </Typography>
          <Typography
            noWrap
            sx={{
              color: 'text.secondary',
              fontFamily: 'monospace',
              fontSize: 12.5,
              mt: 0.25,
            }}
          >
            {component.handler}
          </Typography>
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
              sx={{ alignSelf: 'flex-start', mr: -0.5, mt: -0.5 }}
            >
              <MoreVertical size={16} />
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
      </Stack>

      {/* Two-line clamped description with reserved space so cards align */}
      <Typography
        color="text.secondary"
        sx={{
          display: '-webkit-box',
          fontSize: 12,
          lineHeight: 1.5,
          minHeight: 36,
          mt: 1.5,
          overflow: 'hidden',
          WebkitBoxOrient: 'vertical',
          WebkitLineClamp: 2,
        }}
        variant="body2"
      >
        {component.description || ''}
      </Typography>

      {/* Chips: tinted kind chip, version, per-environment dots */}
      <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 1, mb: 2, mt: 2 }}>
        <Box
          sx={{
            ...chipSx,
            bgcolor: (theme) => alpha(theme.palette.primary.main, 0.14),
            borderColor: (theme) => alpha(theme.palette.primary.main, 0.3),
            color: 'primary.main',
            fontWeight: 600,
          }}
        >
          <Boxes size={13} />
          {COMPONENT_KIND_LABEL[component.kind]}
        </Box>
        {component.version && <Box sx={chipSx}>v{component.version}</Box>}
        <EnvStatusChips componentId={component.id} />
      </Stack>

      {/* Footer: status dot, owner, updated time */}
      <Stack
        alignItems="center"
        direction="row"
        justifyContent="space-between"
        sx={{
          borderColor: 'divider',
          borderTop: '1px solid',
          mt: 'auto',
          pt: 1.75,
        }}
      >
        <Stack
          alignItems="center"
          direction="row"
          spacing={0.875}
          sx={{ color: statusMain }}
        >
          <Box
            sx={{
              bgcolor: statusMain,
              borderRadius: '50%',
              height: 8,
              width: 8,
            }}
          />
          <Typography sx={{ fontSize: 12.5, fontWeight: 500 }}>
            {STATUS_LABEL[component.status]}
          </Typography>
        </Stack>
        <Stack
          alignItems="center"
          direction="row"
          spacing={1.5}
          sx={{ color: 'text.disabled' }}
        >
          {component.owner && (
            <Stack alignItems="center" direction="row" spacing={0.625}>
              <Avatar
                sx={{ fontSize: 9, fontWeight: 600, height: 18, width: 18 }}
              >
                {component.owner.charAt(0).toUpperCase()}
              </Avatar>
              <Typography noWrap sx={{ fontSize: 12 }}>
                {component.owner}
              </Typography>
            </Stack>
          )}
          {updated && (
            <Stack alignItems="center" direction="row" spacing={0.625}>
              <Clock size={13} />
              <Typography sx={{ fontSize: 12 }}>
                {relativeTime(updated)}
              </Typography>
            </Stack>
          )}
        </Stack>
      </Stack>
    </Box>
  );
}
