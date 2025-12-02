import React from 'react';
import { Box, Card, CardActionArea, Checkbox, Typography, useTheme } from '@mui/material';
import { PORTAL_CONSTANTS } from '../../constants/portal';

interface WizardPortalCardProps {
  title: string;
  description?: string;
  portalUrl?: string;
  selected: boolean;
  onSelect: () => void;
  logoSrc?: string;
  logoAlt?: string;
}

const WizardPortalCard: React.FC<WizardPortalCardProps> = ({
  title,
  description,
  portalUrl,
  selected,
  onSelect,
  logoSrc,
  logoAlt = PORTAL_CONSTANTS.DEFAULT_LOGO_ALT,
}) => {
  const theme = useTheme();
  const themeGreen = theme.palette.primary.dark;

  return (
    <Card
      sx={{
        position: 'relative',
        border: selected ? `2px solid ${themeGreen}` : '1px solid #e0e0e0',
        borderRadius: 2,
        transition: 'all 0.2s',
        boxShadow: 'none',
        '&:hover': {
          borderColor: themeGreen,
        },
      }}
    >
      <CardActionArea onClick={onSelect}>
        <Box
          sx={{
            display: 'flex',
            flexDirection: 'column',
            padding: 3,
            minHeight: 240,
          }}
        >
          <Box sx={{ position: 'absolute', top: 8, right: 8 }}>
            <Checkbox
              checked={selected}
              onChange={(e) => {
                e.stopPropagation();
                onSelect();
              }}
              sx={{ padding: 0.5 }}
            />
          </Box>

          <Box
            sx={{
              width: 100,
              height: 100,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              border: '1px solid #e0e0e0',
              borderRadius: 1,
              backgroundColor: '#fff',
              marginBottom: 2,
              alignSelf: 'center',
            }}
          >
            {logoSrc ? (
              <img
                src={logoSrc}
                alt={logoAlt}
                style={{
                  maxWidth: '90px',
                  maxHeight: '90px',
                  objectFit: 'contain',
                }}
              />
            ) : (
              <Typography variant="body2" color="text.secondary">
                No Logo
              </Typography>
            )}
          </Box>

          <Box
            component={portalUrl ? "a" : "div"}
            {...(portalUrl && {
              href: portalUrl,
              target: "_blank",
              rel: "noopener noreferrer",
            })}
            onClick={(e) => {
              if (portalUrl) e.stopPropagation();
            }}
            sx={{
              textDecoration: 'none',
              color: 'inherit',
              ...(portalUrl && {
                '&:hover': {
                  '& .portal-title': {
                    color: themeGreen,
                    textDecoration: 'underline',
                  },
                },
              }),
            }}
          >
            <Typography
              className="portal-title"
              variant="body1"
              sx={{
                fontWeight: 500,
                textAlign: 'center',
                marginBottom: 1,
                transition: 'color 0.2s',
              }}
            >
              {title}
            </Typography>
          </Box>

          <Typography
            variant="body2"
            color="text.secondary"
            sx={{
              textAlign: 'center',
              height: '40px',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              display: '-webkit-box',
              WebkitLineClamp: 2,
              WebkitBoxOrient: 'vertical',
              lineHeight: '20px',
              fontStyle: 'normal',
            }}
          >
            {description || 'No description provided'}
          </Typography>
        </Box>
      </CardActionArea>
    </Card>
  );
};

export default WizardPortalCard;
