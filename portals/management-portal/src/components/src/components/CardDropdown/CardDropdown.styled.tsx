import { alpha, Box, type BoxProps, styled } from '@mui/material';
import { type ComponentType } from 'react';

export interface StyledCardDropdownProps {
  disabled?: boolean;
}

export const StyledCardDropdown: ComponentType<
  StyledCardDropdownProps & BoxProps
> = styled(Box)<BoxProps & StyledCardDropdownProps>(({ theme }) => ({
  backgroundColor: theme.palette.common.white, // From CardButton
  display: 'flex',
  flexDirection: 'row',
  padding: theme.spacing(1.75), // From CardButton
  boxShadow: 'none',
  borderRadius: 8, // From CardButton
  border: `1px solid ${theme.palette.grey[100]}`, // From CardButton
  color: theme.palette.text.primary, // From CardButton
  justifyContent: 'flex-start', // From CardButton
  alignItems: 'center', // Added for better vertical alignment

  // Hover styles from CardButton
  '&:hover': {
    backgroundColor: theme.palette.common.white,
    borderColor: theme.palette.grey[200],
  },

  '& .popoverPaper': {
    border: `1px solid ${theme.palette.grey[100]}`,
    marginTop: theme.spacing(0.5),
    borderRadius: 8, // Match the button's border radius
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

  '&[data-button-root-full-height="true"]': {
    height: '100%',
  },

  // Size specific styles for the icon
  '&[data-card-dropdown-size="small"]': {
    padding: theme.spacing(1.25), // Smaller padding for small size
    '& .startIcon': {
      width: theme.spacing(4), // 32px
      height: theme.spacing(4), // 32px
      minWidth: theme.spacing(4),
      minHeight: theme.spacing(4),
    },
  },

  '&[data-card-dropdown-size="medium"]': {
    padding: theme.spacing(1.5), // Medium padding
    '& .startIcon': {
      width: theme.spacing(5), // 40px
      height: theme.spacing(5), // 40px
      minWidth: theme.spacing(5),
      minHeight: theme.spacing(5),
    },
  },

  '&[data-card-dropdown-size="large"]': {
    minHeight: theme.spacing(5.5),
    '& .startIcon': {
      width: theme.spacing(6), // 48px
      height: theme.spacing(6), // 48px
      minWidth: theme.spacing(6),
      minHeight: theme.spacing(6),
    },
  },

  // Text styles
  '& > Box:nth-of-type(2)': {
    fontWeight: 600, // From CardButton
    lineHeight: `${theme.spacing(3)}px`,
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
    overflow: 'hidden',
  },

  // End icon styles
  '& .endIcon': {
    display: 'flex',
    justifyContent: 'flex-end',
    flexGrow: 1,
    alignItems: 'center', // Changed to center for better alignment
    fontSize: theme.spacing(1.5), // From CardButton
  },

  // Start icon styles
  '& .startIcon': {
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

  // Popover menu styling
  '& .MuiPopover-paper': {
    marginTop: theme.spacing(0.5),
    boxShadow: theme.shadows[3],
    border: `1px solid ${theme.palette.grey[100]}`,
  },

  // Menu item styling
  '& .MuiMenuItem-root': {
    lineHeight: `${theme.spacing(3)}px`,
    padding: theme.spacing(1, 2),
    '&:focus, &:hover, &.Mui-selected': {
      backgroundColor: theme.palette.secondary.light,
    },
  },
}));
