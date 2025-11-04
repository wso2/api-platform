import { alpha, Box, type BoxProps, styled } from '@mui/material';
import type { ComponentType } from 'react';

export interface StyledSearchBarProps extends BoxProps {
  disabled?: boolean;
  size?: 'small' | 'medium';
  color?: 'primary' | 'secondary';
  bordered?: boolean;
  focused?: boolean;
  filter?: boolean;
}

export const StyledSearchBar: ComponentType<StyledSearchBarProps> = styled(
  Box
)<StyledSearchBarProps>(
  ({
    disabled,
    size = 'medium',
    color = 'secondary',
    bordered = false,
    focused = false,
    filter = false,
    theme,
  }) => ({
    opacity: disabled ? 0.5 : 1,
    cursor: disabled ? 'not-allowed' : 'pointer',
    pointerEvents: disabled ? 'none' : 'auto',
    backgroundColor: 'transparent',

    '& .search': {
      position: 'relative',
      width: '100%',
    },

    '& .searchIcon': {
      padding: theme.spacing(0, 1.5),
      height: '100%',
      zIndex: 1,
      pointerEvents: 'none',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      color: theme.palette.secondary.dark,

      ...(focused && {
        color: theme.palette.primary.main,
      }),
    },

    '& .textField': {
      flexGrow: 1,
    },

    '& .MuiInputBase-root, & .inputRoot': {
      color: 'inherit',
      borderRadius: theme.shape.borderRadius,
      transition: 'all 0.3s',
      width: '100%',
      backgroundColor:
        color === 'secondary'
          ? theme.palette.secondary.light
          : theme.palette.common.white,
      height: size === 'small' ? theme.spacing(4) : theme.spacing(5),
      boxSizing: 'border-box',
      border: '1px solid transparent',

      '&.MuiInputBase-adornedEnd': {
        paddingLeft: theme.spacing(2),
      },

      ...(bordered && {
        borderColor: theme.palette.grey[100],
      }),

      ...(focused && {
        backgroundColor: theme.palette.common.white,
        borderColor: theme.palette.primary.light,
        '& .searchIcon': {
          color: theme.palette.primary.main,
        },
      }),

      '&:hover': {
        borderColor: theme.palette.grey[200],
      },

      ...(filter && {
        '&.MuiInputBase-adornedEnd': {
          paddingLeft: 0,
        },
      }),
    },

    '& .rootSmall': {
      height: theme.spacing(4),
      '& .MuiSvgIcon-fontSizeSmall': {
        fontSize: theme.spacing(2),
      },
    },

    '& .rootMedium': {
      height: theme.spacing(5),
    },

    '& .inputRootSecondary': {
      border: `1px solid ${theme.palette.grey[100]}`,
      boxShadow: `0 1px 2px -1px ${alpha(
        theme.palette.common.black,
        0.08
      )}, 0 -3px 9px 0 ${alpha(theme.palette.common.black, 0.04)} inset`,
      backgroundColor: theme.palette.common.white,
    },

    '& .MuiInputBase-input, & .input': {
      padding: theme.spacing(1, 0),
      '&::placeholder': {
        color: theme.palette.secondary.main,
      },
    },

    '& .filterWrap': {
      height: '100%',
      paddingLeft: theme.spacing(0.5),
      position: 'relative',
      border: 'none !important',
      borderRadius: 0,
      backgroundColor: 'transparent',
      '&>div': {
        marginTop: theme.spacing(-0.125),
      },
      '&:before': {
        content: '""',
        position: 'absolute',
        top: theme.spacing(1),
        bottom: theme.spacing(1),
        left: 0,
        width: 1,
        backgroundColor: theme.palette.grey[100],
      },
      '& .MuiSelect-root, & .MuiInputBase-root': {
        border: 'none !important',
        borderRadius: 0,
        backgroundColor: 'transparent',
      },
    },
  })
);
