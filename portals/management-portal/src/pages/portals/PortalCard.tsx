// REPLACE your existing OptionCard with this
import * as React from 'react';
import {
  Box,
  Divider,
  Typography,
  Tooltip,
  CircularProgress,
} from '@mui/material';
import LaunchOutlinedIcon from '@mui/icons-material/LaunchOutlined';
import {
  Card,
  CardActionArea,
  CardContent,
} from '../../components/src/components/Card';
import Edit from '../../components/src/Icons/generated/Edit';
import { Link } from '../../components/src/components/Link';
import { Button } from '../../components/src/components/Button';
import { IconButton } from '../../components/src/components/IconButton';
import { Chip } from '../../components/src/components/Chip';
import { PORTAL_CONSTANTS } from '../../constants/portal';

interface PortalCardProps {
  title: string;
  description: string;
  enabled: boolean;
  onClick: () => void;
  logoSrc?: string;
  logoAlt?: string;
  portalUrl?: string;
  userAuthLabel?: string;
  authStrategyLabel?: string;
  visibilityLabel?: string;
  onEdit?: () => void;
  onActivate?: () => void;
  activating?: boolean;
  testId?: string;
}

const valuePill = (
  text: string,
  variant: 'green' | 'grey' | 'red' = 'grey'
) => {
  if (variant === 'green') {
    return <Chip label={text} color="success" />;
  }
  if (variant === 'red') {
    return <Chip label={text} color="error" variant="outlined" />;
  }
  return <Chip label={text} variant="outlined" color="default" />;
};

const PortalCard: React.FC<PortalCardProps> = ({
  title,
  description,
  enabled, // indicates if the portal is activated/enabled
  onClick,
  logoSrc,
  logoAlt = PORTAL_CONSTANTS.DEFAULT_LOGO_ALT,
  portalUrl = PORTAL_CONSTANTS.DEFAULT_PORTAL_URL,
  userAuthLabel = PORTAL_CONSTANTS.DEFAULT_USER_AUTH_LABEL,
  authStrategyLabel = PORTAL_CONSTANTS.DEFAULT_AUTH_STRATEGY_LABEL,
  visibilityLabel = PORTAL_CONSTANTS.DEFAULT_VISIBILITY_LABEL,
  onEdit,
  onActivate,
  activating,
}) => {
  return (
    <Card testId={''} style={{ maxWidth: 450 }}>
      <CardActionArea onClick={onClick} testId={''}>
        <CardContent>
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
                {logoSrc ? (
                  <Box
                    component="img"
                    src={logoSrc}
                    alt={logoAlt}
                    sx={{ width: 90, height: 90, objectFit: 'contain' }}
                  />
                ) : null}
              </Box>
            </Box>

            {/* Title + description + URL */}
            <Box sx={{ minWidth: 0 }}>
              <Box sx={{ display: 'flex', alignItems: 'center' }}>
                <Typography sx={{ mr: 1 }} fontSize={18} fontWeight={600}>
                  {title}
                </Typography>
                <IconButton
                  component="span"
                  size="small"
                  onClick={(e: React.MouseEvent) => {
                    e.stopPropagation(); // prevent CardActionArea click
                    onEdit?.();
                  }}
                  aria-label={PORTAL_CONSTANTS.ARIA_LABELS.EDIT_PORTAL}
                >
                  <Edit style={{ fontSize: 16 }} />
                </IconButton>
              </Box>

              <Typography
                sx={{
                  mt: 0.5,
                  lineHeight: 1.5,
                  color: 'rgba(0,0,0,0.6)',
                  maxWidth: 300,
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
                  <Tooltip
                    title={
                      enabled
                        ? ''
                        : PORTAL_CONSTANTS.MESSAGES.URL_NOT_AVAILABLE
                    }
                    placement="top"
                  >
                    <span>
                      <Link
                        href={enabled ? portalUrl : undefined}
                        underline="hover"
                        target="_blank"
                        rel="noopener"
                        sx={{
                          fontWeight: 600,
                          color: enabled ? 'inherit' : 'text.disabled',
                          cursor: enabled ? 'pointer' : 'not-allowed',
                        }}
                        onClick={(e: React.MouseEvent) => {
                          if (!enabled) {
                            e.preventDefault();
                            return;
                          }
                          e.stopPropagation();
                        }}
                      >
                        {portalUrl}
                      </Link>
                    </span>
                  </Tooltip>
                </Box>
                <Tooltip
                  title={
                    enabled
                      ? PORTAL_CONSTANTS.MESSAGES.OPEN_PORTAL_URL
                      : PORTAL_CONSTANTS.MESSAGES.URL_NOT_AVAILABLE
                  }
                  placement="top"
                >
                  <span>
                    <IconButton
                      size="small"
                      sx={{ ml: 0.5 }}
                      disabled={!enabled}
                      onClick={(e: React.MouseEvent) => {
                        if (!enabled) return;
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

          {/* Spec rows */}
          <Box sx={{ display: 'grid', gap: 3 }}>
            <Box
              sx={{
                display: 'grid',
                gridTemplateColumns: '1fr auto',
                alignItems: 'center',
                columnGap: 16,
              }}
            >
              <Typography>User authentication</Typography>
              {valuePill(userAuthLabel, 'grey')}
            </Box>

            <Box
              sx={{
                display: 'grid',
                gridTemplateColumns: '1fr auto',
                alignItems: 'center',
                columnGap: 16,
              }}
            >
              <Typography>Authentication strategy</Typography>
              {valuePill(authStrategyLabel, 'grey')}
            </Box>

            <Box
              sx={{
                display: 'grid',
                gridTemplateColumns: '1fr auto',
                alignItems: 'center',
                columnGap: 16,
              }}
            >
              <Typography>Visibility</Typography>
              {valuePill(visibilityLabel, enabled ? 'green' : 'grey')}
            </Box>
          </Box>

          {/* CTA */}
          <Box sx={{ mt: 2 }}>
            {enabled ? (
              <Button
                fullWidth
                disabled
                variant="outlined"
                sx={{
                  backgroundColor: 'success.main',
                  color: 'success.contrastText',
                  '&:hover': {
                    backgroundColor: 'success.main',
                  },
                }}
              >
                {PORTAL_CONSTANTS.STATUS_LABELS.ACTIVATED}
              </Button>
            ) : (
              <Button
                fullWidth
                onClick={(e: React.MouseEvent) => {
                  e.stopPropagation(); // don't trigger card onClick
                  onActivate?.();
                }}
                aria-label={PORTAL_CONSTANTS.ARIA_LABELS.ACTIVATE_PORTAL}
                disabled={Boolean(activating)}
                sx={{ position: 'relative' }}
              >
                {activating ? (
                  <Box
                    sx={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      gap: 1,
                    }}
                  >
                    <CircularProgress size={18} color="inherit" />
                    <span>Activating...</span>
                  </Box>
                ) : (
                  PORTAL_CONSTANTS.STATUS_LABELS.ACTIVATE_PORTAL
                )}
              </Button>
            )}
          </Box>
        </CardContent>
      </CardActionArea>
    </Card>
  );
};

export default PortalCard;
