import type { ComponentType } from 'react';
import { CardDropdown, type CardDropdownProps } from '../CardDropdown';
import { styled } from '@mui/material';

export const StyledCardDropdownMenItemCreate: ComponentType<CardDropdownProps> =
  styled(CardDropdown)<CardDropdownProps>(({ theme }) => ({
    createMenuItem: {
      color: theme.palette.primary.main,
      alignItems: 'center',
    },
    createIcon: {
      marginRight: theme.spacing(1),
      fontSize: theme.spacing(1.5),
      alignItems: 'center',
      display: 'flex',
    },
    createText: {},
  }));
