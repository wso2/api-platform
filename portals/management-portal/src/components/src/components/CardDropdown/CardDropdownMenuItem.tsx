import { MenuItem, type MenuItemProps, styled } from '@mui/material';
import type { ComponentType } from 'react';

export const CardDropdownMenuItem: ComponentType<MenuItemProps> = styled(
  MenuItem
)<MenuItemProps>(({ theme }) => ({
  lineHeight: `${theme.spacing(3)}px`,
  padding: theme.spacing(1, 2),
  '&:focus': {
    backgroundColor: theme.palette.secondary.light,
  },
  '&:hover': {
    backgroundColor: theme.palette.secondary.light,
  },
  '&$selected': {
    backgroundColor: theme.palette.secondary.light,
  },
  selected: {},
}));

export default CardDropdownMenuItem;
