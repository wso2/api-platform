import { Box, type BoxProps, styled } from '@mui/material';
import type { ComponentType } from 'react';

export interface StyledSplitButtonProps extends BoxProps {
  disabled?: boolean;
}

export const StyledSplitButton: ComponentType<StyledSplitButtonProps> = styled(
  Box
)<StyledSplitButtonProps>(({ disabled, theme }) => ({
  display: 'flex',
  alignItems: 'center',
  cursor: disabled ? 'not-allowed' : 'default',
  opacity: disabled ? 0.5 : 1,
  pointerEvents: disabled ? 'none' : 'auto',
  filter: disabled ? 'saturate(0.3) brightness(1.1)' : 'none',
  fontFamily: theme.typography.fontFamily,
  splitButton: {},
  '.splitButtonLabel': {
    marginRight: theme.spacing(1),
  },
  '.splitButtonLabelSecondary': {
    color: theme.palette.secondary.main,
  },
  '.splitIconButton': {
    flex: `0 0 ${theme.spacing(5)}px`,
  },
  '.splitButtonMain': {
    justifyContent: 'center',
  },
  '.splitButtonLabelPrimaryOutlined': {
    color: theme.palette.primary.light,
  },
  '& .Outlined': {
    backgroundColor: 'transparent',
    boxShadow: theme.shadows[1],
  },
}));
