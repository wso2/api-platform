import React from 'react';
import { Box, Divider, Grid, Paper, Typography, Tooltip } from '@mui/material';
import LaunchOutlinedIcon from '@mui/icons-material/LaunchOutlined';
import { Button } from '../../../components/src/components/Button';
import { IconButton } from '../../../components/src/components/IconButton';
import { Chip } from '../../../components/src/components/Chip';
import { PORTAL_CONSTANTS } from '../../../constants/portal';
import BijiraDPLogo from '../../BijiraDPLogo.png';
import type { ApiPublicationWithPortal } from '../../../hooks/apiPublish';

const Card: React.FC<React.PropsWithChildren<{ sx?: any }>> = ({
  children,
  ...props
}) => (
  <Paper
    elevation={0}
    {...props}
    sx={{
      p: 3,
      borderRadius: 3,
      border: (t) => `1px solid ${t.palette.divider}`,
      width: 380,
      height: 230, // fixed height for consistency
      display: 'flex',
      flexDirection: 'column',
      justifyContent: 'space-between',
      ...(props as any).sx,
    }}
  >
    {children}
  </Paper>
);

type Props = {
  portal: ApiPublicationWithPortal;
  apiId: string;
  publishingIds: Set<string>;
  onPublish: (portal: ApiPublicationWithPortal) => void;
};

const DevPortalDeployCard: React.FC<Props> = ({
  portal,
  apiId,
  publishingIds,
  onPublish,
}) => {
  const isPublished = portal.isPublished;
  const publication = portal.publication;

  const description = portal.description || '';

  let status = 'NOT_PUBLISHED';
  let success = false;

  if (isPublished) {
    if (publication?.status === 'published') {
      status = 'PUBLISHED';
      success = true;
    } else if (publication?.status === 'failed') {
      status = 'FAILED';
      success = false;
    } else {
      status = 'PUBLISHED'; // default to published if status is unknown
      success = true;
    }
  }

  const title = portal.name || 'Dev Portal';
  const isPublishingThis = publishingIds.has(portal.uuid);
  const portalUrl = portal.portalUrl || portal.apiUrl;

  return (
    <Grid key={portal.uuid}>
      <Card>
        <Box
          sx={{
            display: 'grid',
            gridTemplateColumns: 'auto 1fr auto',
            gap: 2,
            alignItems: 'start',
          }}
        >
          {/* Logo block */}
          <Box
            sx={{
              width: 100,
              height: 100,
              borderRadius: 2,
              bgcolor: 'transparent',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <Box
              sx={{
                width: 100,
                height: 100,
                borderRadius: '8px',
                border: '0.5px solid #abb8c2ff',
                bgcolor: '#d9e0e4ff',
                overflow: 'hidden',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              <Box
                component="img"
                src={BijiraDPLogo}
                alt="Bijira Dev Portal Logo"
                sx={{ width: 90, height: 90, objectFit: 'contain' }}
              />
            </Box>
          </Box>

          {/* Title + description + URL */}
          <Box sx={{ minWidth: 0 }}>
            <Box sx={{ display: 'flex', alignItems: 'center' }}>
              <Typography sx={{ mr: 1 }} fontSize={18} fontWeight={600}>
                {title}
              </Typography>
            </Box>

            <Typography
              sx={{
                mt: 0.5,
                lineHeight: 1.5,
                color: 'rgba(0,0,0,0.6)',
                maxWidth: 300,
                minHeight: '3em',
                display: '-webkit-box',
                WebkitLineClamp: 2,
                WebkitBoxOrient: 'vertical',
                overflow: 'hidden',
                textOverflow: 'ellipsis',
              }}
              variant="body2"
            >
              {description}
            </Typography>

            <Box sx={{ mt: 1, display: 'flex', alignItems: 'center' }}>
              <Box
                sx={{
                  flex: 1,
                  minWidth: 0,
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                  mr: 0.5,
                }}
              >
                <Typography
                  variant="body2"
                  sx={{
                    fontWeight: 600,
                    color: 'inherit',
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    whiteSpace: 'nowrap',
                  }}
                  title={portalUrl}
                >
                  {portalUrl}
                </Typography>
              </Box>
              <Tooltip
                title={PORTAL_CONSTANTS.MESSAGES.OPEN_PORTAL_URL}
                placement="top"
              >
                <span>
                  <IconButton
                    size="small"
                    sx={{ ml: 0.5 }}
                    onClick={(e: React.MouseEvent) => {
                      e.stopPropagation();
                      window.open(portalUrl, '_blank', 'noopener,noreferrer');
                    }}
                    aria-label={PORTAL_CONSTANTS.ARIA_LABELS.OPEN_PORTAL_URL}
                  >
                    <LaunchOutlinedIcon fontSize="inherit" />
                  </IconButton>
                </span>
              </Tooltip>
            </Box>
          </Box>

          {/* right spacer */}
          <Box />
        </Box>

        {/* Divider */}
        <Divider sx={{ my: 2 }} />
        {isPublished && (
          <Box
            sx={{
              backgroundColor: (t) =>
                status === 'FAILED'
                  ? t.palette.mode === 'dark'
                    ? 'rgba(239,68,68,0.12)'
                    : '#FDECEC'
                  : success
                    ? t.palette.mode === 'dark'
                      ? 'rgba(16,185,129,0.12)'
                      : '#E8F7EC'
                    : t.palette.mode === 'dark'
                      ? 'rgba(239,68,68,0.12)'
                      : '#FDECEC',
              border: (t) =>
                `1px solid ${
                  status === 'FAILED'
                    ? t.palette.error.light
                    : success
                      ? '#D8EEDC'
                      : t.palette.error.light
                }`,
              borderRadius: 2,
              px: 2,
              py: 1.25,
              mb: 2,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
            }}
          >
            <Typography fontWeight={500}>Publish Status</Typography>
            <Chip
              label={status}
              color={
                status === 'FAILED'
                  ? 'error'
                  : success
                    ? 'success'
                    : 'error'
              }
              variant={success ? 'filled' : 'outlined'}
              size="small"
            />
          </Box>
        )}

        {/* Publish / Re-publish action (pinned to bottom) */}
        {!isPublished && (
          <Button
            variant="contained"
            fullWidth
            disabled={!apiId || isPublishingThis}
            onClick={() => onPublish(portal)}
          >
            {isPublishingThis ? 'Adding…' : 'Add API'}
          </Button>
        )}
      </Card>
    </Grid>
  );
};

export default DevPortalDeployCard;
