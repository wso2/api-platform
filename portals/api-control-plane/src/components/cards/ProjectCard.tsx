import { useState, type MouseEvent } from 'react';
import {
  Box,
  Button,
  Chip,
  IconButton,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {
  ArrowRight,
  Boxes,
  Clock,
  GitBranch,
  MoreVertical,
  Rocket,
  Settings,
  Trash2,
} from '@wso2/oxygen-ui-icons-react';
import { Link } from 'react-router-dom';

import { useApis } from '../../api/hooks/useMvpQueries';
import { routes } from '../../routes/paths';
import type { Project } from '../../types/domain';
import { relativeTime } from '../../utils/relativeTime';

type ProjectCardProps = {
  project: Project;
  orgHandle: string;
  onOpen: (project: Project) => void;
  onDelete?: (project: Project) => void;
};

// Decorative brand accents (read on both light and dark surfaces): WSO2 orange
// for regular projects, a cyan tone for the default project.
const ORANGE_ACCENT = 'linear-gradient(90deg, #F47B20, #EF4223)';
const CYAN_ACCENT = 'linear-gradient(90deg, #3AA0D6, #5CD1FF)';

const repoTypeLabel = (type: Project['type']) =>
  type === 'MONO_REPO'
    ? 'Mono repo'
    : type === 'MULTI_REPO'
      ? 'Multi repo'
      : undefined;

export function ProjectCard({
  project,
  orgHandle,
  onOpen,
  onDelete,
}: ProjectCardProps) {
  const stopCardClick = (event: MouseEvent) => event.stopPropagation();
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);

  const closeMenu = (event?: MouseEvent) => {
    event?.stopPropagation();
    setMenuAnchor(null);
  };
  const isDefault =
    project.handler === 'default' || project.name.toLowerCase() === 'default';
  const accent = isDefault ? CYAN_ACCENT : ORANGE_ACCENT;
  const iconTint = isDefault ? 'rgba(92,209,255,0.14)' : 'rgba(255,115,0,0.14)';
  const iconColor = isDefault ? '#3AA0D6' : '#FF7300';
  const repoLabel = repoTypeLabel(project.type);

  const apisQuery = useApis(orgHandle, project.handler);
  const apiCount = apisQuery.data?.length;
  const deployedCount = apisQuery.data?.filter(
    (api) => api.status === 'ACTIVE'
  ).length;

  return (
    <Box
      onClick={() => onOpen(project)}
      sx={{
        bgcolor: 'background.paper',
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: 1,
        cursor: 'pointer',
        display: 'flex',
        flexDirection: 'column',
        height: '100%',
        overflow: 'hidden',
        transition:
          'transform .18s ease, border-color .18s ease, box-shadow .18s ease',
        '&:hover': {
          borderColor: 'primary.main',
          boxShadow: 4,
          transform: 'translateY(-3px)',
        },
      }}
    >
      {/* accent strip */}
      <Box sx={{ background: accent, height: 4 }} />

      <Box sx={{ flexGrow: 1, p: 2.5 }}>
        <Stack alignItems="flex-start" direction="row" spacing={1.75}>
          <Box
            sx={{
              alignItems: 'center',
              bgcolor: iconTint,
              border: '1px solid',
              borderColor: 'divider',
              borderRadius: 1,
              color: iconColor,
              display: 'flex',
              flex: 'none',
              height: 46,
              justifyContent: 'center',
              width: 46,
            }}
          >
            <Boxes size={22} />
          </Box>
          <Box sx={{ minWidth: 0 }}>
            <Stack alignItems="center" direction="row" spacing={1}>
              <Typography noWrap sx={{ fontWeight: 600 }} variant="subtitle1">
                {project.name}
              </Typography>
              {isDefault && (
                <Chip
                  color="info"
                  label="DEFAULT"
                  size="small"
                  sx={{ fontSize: 10, fontWeight: 600, height: 20 }}
                  variant="outlined"
                />
              )}
            </Stack>
            <Typography
              color="text.secondary"
              noWrap
              sx={{ mt: 0.25 }}
              variant="body2"
            >
              {project.description || 'Project workspace'}
            </Typography>
          </Box>
        </Stack>

        {/* info strip — real project metadata */}
        <Stack
          direction="row"
          spacing={2}
          sx={{
            alignItems: 'center',
            bgcolor: 'action.hover',
            border: '1px solid',
            borderColor: 'divider',
            borderRadius: 1,
            color: 'text.secondary',
            mt: 2.25,
            px: 1.75,
            py: 1.25,
          }}
        >
          <Stack
            alignItems="center"
            direction="row"
            spacing={0.75}
            sx={{ minWidth: 0 }}
          >
            <Boxes size={16} />
            <Typography noWrap variant="body2">
              {apisQuery.isLoading
                ? '… APIs'
                : `${apiCount ?? 0} ${apiCount === 1 ? 'API' : 'APIs'}`}
            </Typography>
          </Stack>
          <Stack
            alignItems="center"
            direction="row"
            spacing={0.75}
            sx={{ minWidth: 0 }}
          >
            <Rocket size={16} />
            <Typography noWrap variant="body2">
              {apisQuery.isLoading
                ? '… deployed'
                : `${deployedCount ?? 0} deployed`}
            </Typography>
          </Stack>
          {repoLabel && (
            <Stack alignItems="center" direction="row" spacing={0.75}>
              <GitBranch size={16} />
              <Typography noWrap variant="body2">
                {repoLabel}
              </Typography>
            </Stack>
          )}
        </Stack>
      </Box>

      {/* footer */}
      <Box
        sx={{
          alignItems: 'center',
          bgcolor: 'action.hover',
          borderColor: 'divider',
          borderTop: '1px solid',
          display: 'flex',
          gap: 1,
          px: 2,
          py: 1.25,
        }}
      >
        <Clock size={14} />
        <Typography color="text.secondary" variant="caption">
          {project.updatedAt
            ? `Updated ${relativeTime(project.updatedAt)}`
            : 'Not updated yet'}
        </Typography>
        <Box sx={{ flex: 1 }} />
        <Tooltip title="Project settings">
          <IconButton
            aria-label="Project settings"
            component={Link}
            onClick={stopCardClick}
            size="small"
            to={routes.settings(orgHandle, project.handler)}
          >
            <Settings size={16} />
          </IconButton>
        </Tooltip>
        {onDelete && (
          <>
            <Tooltip title="Project actions">
              <IconButton
                aria-label="Project actions"
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
                <ListItemText>Delete</ListItemText>
              </MenuItem>
            </Menu>
          </>
        )}
        <Button
          endIcon={<ArrowRight size={14} />}
          onClick={(event) => {
            stopCardClick(event);
            onOpen(project);
          }}
          size="small"
          sx={{ borderRadius: 5 }}
          variant="outlined"
        >
          Open
        </Button>
      </Box>
    </Box>
  );
}
