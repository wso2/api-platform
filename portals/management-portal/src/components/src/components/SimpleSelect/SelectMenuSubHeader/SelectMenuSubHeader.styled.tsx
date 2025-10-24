import {
  ListSubheader,
  ListSubheaderProps as MUISelectSubHeaderProps,
} from '@mui/material';
import { styled, Theme } from '@mui/material/styles';

interface SelectMenuSubHeaderProps extends MUISelectSubHeaderProps {
  testId: string;
  theme?: Theme;
}

export const StyledSelectMenuSubHeader: React.ComponentType<SelectMenuSubHeaderProps> =
  styled(ListSubheader)(({ theme }) => ({
    '.selectMenuSubHeader': {
      pointerEvents: 'none',
      lineHeight: `${theme.spacing(4)}px`,
      color: theme.palette.secondary.main,
      fontSize: theme.spacing(1.25),
      fontWeight: 700,
      textTransform: 'uppercase',
    },
    '.selectMenuSubHeaderInset': {},
  }));
