import { alpha, Button, type ButtonProps, styled } from '@mui/material';
import type { ComponentType } from 'react';

export interface StyledCardButtonProps extends ButtonProps {
  disabled?: boolean;
}

export const StyledCardButton: ComponentType<StyledCardButtonProps> = styled(
  Button
)<StyledCardButtonProps>(({ theme }) => ({
  padding: theme.spacing(1.75),
  backgroundColor: theme.palette.common.white,
  textTransform: 'none',
  boxShadow: 'none',
  borderRadius: 8,
  border: `1px solid ${theme.palette.grey[100]}`,
  color: theme.palette.text.primary,
  justifyContent: 'flex-start',
  '&:hover': {
    backgroundColor: theme.palette.common.white,
    borderColor: theme.palette.grey[200],
  },
  '&[data-button-root-active="true"]': {
    borderColor: theme.palette.primary.light,
    boxShadow: `0 0 0 1px ${theme.palette.primary.light}`,
    backgroundColor: alpha(theme.palette.primary.main, 0.08),
    '&:hover': {
      borderColor: theme.palette.primary.light,
      boxShadow: `0 0 0 1px ${theme.palette.primary.light}`,
      backgroundColor: alpha(theme.palette.primary.main, 0.12),
    },
  },
  '&[data-button-root-error="true"]': {
    borderColor: theme.palette.error.main,
    boxShadow: `0 0 0 1px ${theme.palette.error.light}`,
    backgroundColor: theme.palette.error.light,
    '&:hover': {
      borderColor: theme.palette.error.main,
      boxShadow: `0 0 0 1px ${theme.palette.error.main}`,
      backgroundColor: theme.palette.error.light,
    },
  },
  '&[data-button-root-full-height="true"]': {
    height: '100%',
  },
  '&.Mui-buttonEndIcon': {
    marginLeft: 'auto',
    marginRight: 0,
    '&.MuiButton-iconSizeSmall > *:first-child': {
      fontSize: theme.spacing(1.5),
    },
    '&.MuiButton-iconSizeMedium > *:first-child': {
      fontSize: theme.spacing(1.5),
    },
    '&.MuiButton-iconSizeLarge > *:first-child': {
      fontSize: theme.spacing(1.5),
    },
  },
  // Base styles for startIcon - these apply to all sizes
  '& .MuiButton-startIcon': {
    margin: 0,
    marginRight: theme.spacing(2),
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    overflow: 'visible',
    '& > *': {
      maxWidth: '100%',
      maxHeight: '100%',
      width: 'auto',
      height: 'auto',
      objectFit: 'contain',
    },
  },
  // Size-specific styles
  '&[data-button-label-size="small"]': {
    '& .MuiButton-startIcon': {
      width: theme.spacing(4), // 32px
      height: theme.spacing(4), // 32px
      minWidth: theme.spacing(4),
      minHeight: theme.spacing(4),
    },
  },
  '&[data-button-label-size="medium"]': {
    '& .MuiButton-startIcon': {
      width: theme.spacing(5), // 40px
      height: theme.spacing(5), // 40px
      minWidth: theme.spacing(5),
      minHeight: theme.spacing(5),
    },
  },
  '&[data-button-label-size="large"]': {
    minHeight: theme.spacing(5.5),
    '& .MuiButton-startIcon': {
      width: theme.spacing(6), // 48px
      height: theme.spacing(6), // 48px
      minWidth: theme.spacing(6),
      minHeight: theme.spacing(6),
    },
  },
  '&.Mui-buttonLabel': {
    lineHeight: `${theme.spacing(3)}px`,
    fontWeight: 600,
    display: 'flex',
    alignItems: 'center',
    gridGap: theme.spacing(2),
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    '& $buttonStartIcon': {
      marginRight: 0,
      marginLeft: 0,
    },
  },
  '& .buttonLabelText': {
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
    overflow: 'hidden',
  },
  '& .endIcon': {
    display: 'flex',
    justifyContent: 'flex-end',
    flexGrow: 1,
    alignItems: 'flex-end',
  },
  '&$disabled': {
    boxShadow: 'none',
    pointerEvents: 'none',
  },
}));
