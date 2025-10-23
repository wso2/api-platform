import { styled } from '@mui/material/styles';
import {
  Chip as MUIChip,
  type ChipProps as MUIChipProps,
  alpha,
} from '@mui/material';
import type { ComponentType } from 'react';

export const StyledChip: ComponentType<MUIChipProps> = styled(MUIChip)(
  ({ theme, disabled, variant = 'filled' }) => ({
    opacity: disabled ? 0.5 : 1,
    cursor: disabled ? 'not-allowed' : 'default',
    pointerEvents: disabled ? 'none' : 'auto',

    '& .MuiTouchRipple-root': {
      display: 'none',
    },
    '&.MuiChip-sizeSmall': {
      fontSize: theme.spacing(1.25),
      borderRadius: theme.spacing(0.375),
      lineHeight: 1.2,
      height: theme.spacing(2),
      '& .MuiChip-label': {
        padding: theme.spacing(0, 0.5, 0.125, 0.5),
      },
      '& .MuiChip-icon': {
        marginLeft: theme.spacing(0.5),
        marginRight: 0,
        fontSize: theme.spacing(1.25),
        '& > *': {
          fontSize: 'inherit',
        },
      },
    },
    '&.MuiChip-sizeMedium': {
      fontSize: theme.spacing(1.625),
      borderRadius: theme.spacing(0.625),
      lineHeight: 1.23,
      height: theme.spacing(3),
      '& .MuiChip-label': {
        padding: theme.spacing(0.1, 1),
      },
      '& .MuiChip-icon': {
        marginLeft: theme.spacing(1),
        marginRight: 0,
        fontSize: theme.spacing(1.5),
        '& > *': {
          fontSize: 'inherit',
        },
      },
    },
    '&.MuiChip-sizeLarge': {
      fontSize: theme.spacing(1.625),
      borderRadius: theme.spacing(0.625),
      lineHeight: 1.23,
      '& .MuiChip-label': {
        padding: theme.spacing(1, 1.5),
      },
      '& .MuiChip-icon': {
        marginLeft: theme.spacing(1.5),
        marginRight: 0,
        fontSize: theme.spacing(1.75),
        '& > *': {
          fontSize: 'inherit',
        },
      },
    },

    ...(variant === 'filled' && {
      '&.MuiChip-colorDefault': {
        backgroundColor: theme.palette.grey[200],
      },
      '&.MuiChip-colorPrimary': {
        backgroundColor: theme.palette.primary.main,
        color: theme.palette.common.white,
      },
      '&.MuiChip-colorSecondary': {
        backgroundColor: theme.palette.secondary.contrastText,
        color: theme.palette.secondary.dark,
      },
      '&.MuiChip-colorSuccess': {
        backgroundColor: theme.palette.success.main,
        color: theme.palette.common.white,
      },
      '&.MuiChip-colorError': {
        backgroundColor: theme.palette.error.main,
        color: theme.palette.common.white,
      },
      '&.MuiChip-colorWarning': {
        backgroundColor: theme.palette.warning.dark,
        color: theme.palette.common.white,
      },
      '&.MuiChip-colorInfo': {
        backgroundColor: theme.palette.info.main,
        color: theme.palette.common.white,
      },
    }),

    ...(variant === 'outlined' && {
      '&.MuiChip-colorDefault': {
        backgroundColor: theme.palette.grey[100],
        border: `1px solid ${theme.palette.grey[200]}`,
      },
      '&.MuiChip-colorPrimary': {
        border: `1px solid ${theme.palette.primary.main}`,
        color: theme.palette.primary.main,
        backgroundColor: theme.palette.primary.light,
      },
      '&.MuiChip-colorSecondary': {
        backgroundColor: theme.palette.common.white,
        border: `1px solid ${theme.palette.grey[200]}`,
        color: theme.palette.secondary.dark,
      },
      '&.MuiChip-colorSuccess': {
        border: `1px solid ${theme.palette.success.main}`,
        color: theme.palette.success.main,
        backgroundColor: theme.palette.success.light,
      },
      '&.MuiChip-colorError': {
        border: `1px solid ${theme.palette.error.main}`,
        color: theme.palette.error.main,
        backgroundColor: theme.palette.error.light,
      },
      '&.MuiChip-colorWarning': {
        border: `1px solid ${theme.palette.warning.dark}`,
        color: theme.palette.warning.dark,
        backgroundColor: theme.palette.warning.light,
      },
      '&.MuiChip-colorInfo': {
        color: theme.palette.info.main,
        border: `1px solid ${theme.palette.info.main}`,
        backgroundColor: alpha(theme.palette.info.main, 0.1),
      },
    }),
  })
);
