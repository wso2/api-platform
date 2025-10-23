import { Box, type BoxProps, styled } from '@mui/material';
import type { ComponentType } from 'react';

export interface StyledCardFormProps {
  disabled?: boolean;
}

export const StyledCardForm: ComponentType<StyledCardFormProps & BoxProps> =
  styled(Box)<BoxProps & StyledCardFormProps>(({ disabled, theme }) => ({
    display: 'flex',
    flexDirection: 'column',
    backgroundColor: theme.palette.background.paper,
    borderRadius: theme.shape.borderRadius,
    boxShadow: theme.shadows[1],
    transition: theme.transitions.create(
      ['box-shadow', 'transform', 'background-color'],
      {
        duration: theme.transitions.duration.short,
      }
    ),
    border: `1px solid ${theme.palette.divider}`,
    opacity: disabled ? 0.5 : 1,
    cursor: disabled ? 'not-allowed' : 'pointer',

    '&:hover': {
      backgroundColor: theme.palette.action.hover,
      boxShadow: theme.shadows[3],
      transform: 'translateY(-2px)',
    },

    '&:active': {
      transform: 'translateY(0)',
      boxShadow: theme.shadows[2],
    },
  }));

export const StyledCardFormHeader: ComponentType<BoxProps> = styled(Box)(
  ({ theme }) => ({
    padding: theme.spacing(2),
    borderBottom: `1px solid ${theme.palette.divider}`,
    fontWeight: 500,
  })
);

export const StyledCardFormContent: ComponentType<BoxProps> = styled(Box)(
  ({ theme }) => ({
    padding: theme.spacing(2),
  })
);
