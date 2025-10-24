import { styled } from '@mui/material/styles';
import { MenuItem } from '@mui/material';
import type { ComponentType } from 'react';
import { type SelectMenuItemProps } from './SelectMenuItem';

export const StyledSelectMenuItem: ComponentType<SelectMenuItemProps> = styled(
  MenuItem
)(({ theme }) => ({
  '.selectMenuItem': {
    whiteSpace: 'normal',
    wordWrap: 'break-word',
    display: 'block',
  },
  '.selectMenuItemDisabled': {},
  '.selectMenuSubHeader': {
    pointerEvents: 'none',
    lineHeight: `${theme.spacing(4)}px`,
    color: theme.palette.secondary.main,
    fontSize: theme.spacing(1.25),
    fontWeight: 700,
    textTransform: 'uppercase',
  },
  '.description': {
    color: theme.palette.text.secondary,
    marginTop: theme.spacing(0.5),
    flexGrow: 1,
    wordWrap: 'break-word',
    whiteSpace: 'normal',
    lineHeight: 1.5,
    maxWidth: `calc(100% - ${theme.spacing(1.5)})`,
  },
}));
