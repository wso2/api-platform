import { styled, type Theme } from '@mui/material/styles';
import { Chip, type ChipProps } from '@mui/material';
import type { ComponentType } from 'react';

export const StyledTag: ComponentType<ChipProps> = styled(Chip, {
  shouldForwardProp: (prop) =>
    !['size', 'variant', 'color', 'disabled'].includes(prop as string),
})(({
  theme,
  size = 'medium',
  variant = 'filled',
  color = 'default',
  disabled,
}: {
  theme: Theme;
  size?: 'small' | 'medium' | 'large';
  variant?: 'filled' | 'outlined';
  color?:
    | 'default'
    | 'primary'
    | 'secondary'
    | 'error'
    | 'warning'
    | 'info'
    | 'success';
  disabled?: boolean;
}) => {
  const sizeStyles = {
    small: {
      padding: theme.spacing(0.4, 0.5, 0.5, 0.8),
      fontSize: theme.spacing(1.625),
      borderRadius: theme.spacing(0.375),
      lineHeight: 1.6,
      height: theme.spacing(3),
    },
    medium: {
      padding: theme.spacing(1, 1.2, 1, 1.5),
      fontSize: theme.spacing(1.625),
      borderRadius: theme.spacing(0.625),
      lineHeight: 1.23,
      height: theme.spacing(4),
    },
    large: {
      padding: theme.spacing(1.2, 1.5, 1.2, 1.8),
      fontSize: theme.spacing(1.625),
      borderRadius: theme.spacing(0.625),
      lineHeight: 1.23,
      height: theme.spacing(4.5),
    },
  };

  let variantStyles = {};

  if (variant === 'filled') {
    switch (color) {
      case 'primary':
        variantStyles = {
          backgroundColor: theme.palette.primary.main,
          color: theme.palette.common.white,
        };
        break;
      case 'secondary':
        variantStyles = {
          backgroundColor: theme.palette.secondary.main,
          color: theme.palette.common.white,
        };
        break;
      case 'error':
        variantStyles = {
          backgroundColor: theme.palette.error.main,
          color: theme.palette.common.white,
        };
        break;
      case 'warning':
        variantStyles = {
          backgroundColor: theme.palette.warning.main,
          color: theme.palette.common.white,
        };
        break;
      case 'info':
        variantStyles = {
          backgroundColor: theme.palette.info.main,
          color: theme.palette.common.white,
        };
        break;
      case 'success':
        variantStyles = {
          backgroundColor: theme.palette.success.main,
          color: theme.palette.common.white,
        };
        break;
      case 'default':
      default:
        variantStyles = {
          backgroundColor: theme.palette.secondary.light,
          border: `1px solid ${theme.palette.grey[100]}`,
          color: theme.palette.text.primary,
        };
        break;
    }
  }

  if (variant === 'outlined') {
    switch (color) {
      case 'primary':
        variantStyles = {
          border: `1px solid ${theme.palette.primary.main}`,
          color: theme.palette.primary.main,
          backgroundColor: theme.palette.background.paper,
        };
        break;
      case 'secondary':
        variantStyles = {
          border: `1px solid ${theme.palette.secondary.main}`,
          color: theme.palette.secondary.main,
          backgroundColor: theme.palette.background.paper,
        };
        break;
      case 'error':
        variantStyles = {
          border: `1px solid ${theme.palette.error.main}`,
          color: theme.palette.error.main,
          backgroundColor: theme.palette.background.paper,
        };
        break;
      case 'warning':
        variantStyles = {
          border: `1px solid ${theme.palette.warning.main}`,
          color: theme.palette.warning.main,
          backgroundColor: theme.palette.background.paper,
        };
        break;
      case 'info':
        variantStyles = {
          border: `1px solid ${theme.palette.info.main}`,
          color: theme.palette.info.main,
          backgroundColor: theme.palette.background.paper,
        };
        break;
      case 'success':
        variantStyles = {
          border: `1px solid ${theme.palette.success.main}`,
          color: theme.palette.success.main,
          backgroundColor: theme.palette.background.paper,
        };
        break;
      case 'default':
      default:
        variantStyles = {
          border: `1px solid ${theme.palette.grey[100]}`,
          color: theme.palette.text.primary,
          backgroundColor: theme.palette.background.paper,
        };
        break;
    }
  }

  return {
    ...sizeStyles[size],
    ...variantStyles,
    opacity: disabled ? 0.5 : 1,
    cursor: disabled ? 'not-allowed' : 'default',
    pointerEvents: disabled ? 'none' : 'auto',
    '&:hover': {
      backgroundColor: (variantStyles as any).backgroundColor,
      color: (variantStyles as any).color,
      border: (variantStyles as any).border,
      boxShadow: 'none',
      filter: 'none',
    },
    '& .MuiChip-label': {
      paddingLeft: 0,
      paddingRight: 0,
      whiteSpace: 'initial',
      textOverflow: 'initial',
      overflowWrap: 'break-word' as const,
      lineHeight: 1.3,
    },
    '& .MuiChip-deleteIcon': {
      width: '0.8em',
      height: '0.8em',
      marginLeft: theme.spacing(1),
    },
  };
});
