import styled from '@emotion/styled';
import { IconButton } from '@mui/material';
import { alpha, type Theme } from '@mui/material/styles';

const getFocusShadow = (theme: Theme) =>
  `0 ${theme.spacing(0.125)} ${theme.spacing(0.75)} ${theme.spacing(0.25)} ${alpha(
    theme.palette.common.black,
    0.1
  )}`;

export const StyledIconButton = styled(IconButton, {
  shouldForwardProp: (prop) =>
    !['size', 'color', 'disabled', 'edge'].includes(prop as string),
})(({
  theme,
  size = 'medium',
  color = 'default',
  disabled,
}: {
  theme: Theme;
  size?: 'small' | 'medium' | 'tiny';
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
      padding: theme.spacing(0.875),
      '& svg': {
        fontSize: theme.spacing(2),
      },
    },
    medium: {
      padding: theme.spacing(1),
      '& > *:first-of-type': {
        fontSize: theme.spacing(2.5),
      },
    },
    tiny: {
      padding: theme.spacing(0.625),
      '& svg': {
        fontSize: theme.spacing(1.375),
      },
    },
  };

  let colorStyles: { color?: string; '&:hover'?: { backgroundColor: string } } =
    {};
  switch (color) {
    case 'primary':
      colorStyles = {
        color: theme.palette.primary.main,
        '&:hover': {
          backgroundColor: alpha(theme.palette.primary.main, 0.04),
        },
      };
      break;
    case 'secondary':
      colorStyles = {
        color: theme.palette.secondary.main,
        '&:hover': {
          backgroundColor: alpha(theme.palette.secondary.main, 0.04),
        },
      };
      break;
    case 'error':
      colorStyles = {
        color: theme.palette.error.main,
        '&:hover': {
          backgroundColor: alpha(theme.palette.error.main, 0.04),
        },
      };
      break;
    case 'warning':
      colorStyles = {
        color: theme.palette.warning.main,
        '&:hover': {
          backgroundColor: alpha(theme.palette.warning.main, 0.04),
        },
      };
      break;
    case 'info':
      colorStyles = {
        color: theme.palette.info.main,
        '&:hover': {
          backgroundColor: alpha(theme.palette.info.main, 0.04),
        },
      };
      break;
    case 'success':
      colorStyles = {
        color: theme.palette.success.main,
        '&:hover': {
          backgroundColor: alpha(theme.palette.success.main, 0.04),
        },
      };
      break;
    case 'default':
    default:
      colorStyles = {
        color: theme.palette.text.primary,
        '&:hover': {
          backgroundColor: alpha(theme.palette.action.active, 0.04),
        },
      };
      break;
  }

  return {
    borderRadius: theme.spacing(0.625),
    ...sizeStyles[size as keyof typeof sizeStyles],
    ...colorStyles,
    opacity: disabled ? 0.5 : 1,
    cursor: disabled ? 'not-allowed' : 'pointer',
    '&.Mui-disabled': {
      opacity: 0.5,
      cursor: 'not-allowed',
      pointerEvents: 'none' as const,
    },
    '&:focus-visible': {
      boxShadow: getFocusShadow(theme),
    },
    '&:hover': {
      ...colorStyles['&:hover'],
    },
  };
});
