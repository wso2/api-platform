import { Box, type BoxProps, styled } from '@mui/material';
import type { ComponentType } from 'react';

export interface StyledCheckboxProps extends BoxProps {
  disabled?: boolean;
}

export const StyledCheckbox: ComponentType<StyledCheckboxProps> = styled(Box, {
  shouldForwardProp: (prop) => !['disabled'].includes(prop as string),
})<StyledCheckboxProps>(({ disabled, theme }) => ({
  display: 'flex',
  alignItems: 'center',
  cursor: disabled ? 'not-allowed' : 'pointer',
  opacity: disabled ? theme.palette.action.disabledOpacity : 1,
  textAlign: 'left',
  gap: theme.spacing(0.5),
  backgroundColor: 'transparent',
  '& span': {
    color: theme.palette.text.primary,
    fontFamily: theme.typography.fontFamily,
    fontSize: theme.typography.body1.fontSize,
    fontWeight: theme.typography.fontWeightRegular,
  },
  '&:disabled': {
    cursor: 'not-allowed',
    opacity: theme.palette.action.disabledOpacity,
    pointerEvents: 'none',
  },
}));
