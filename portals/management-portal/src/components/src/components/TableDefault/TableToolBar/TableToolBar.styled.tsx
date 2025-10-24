import {
  styled,
  Toolbar as MUITableToolBar,
  type ToolbarProps as MUITableToolBarProps,
  type Theme,
  lighten,
} from '@mui/material';

interface ToolbarProps extends MUITableToolBarProps {
  theme?: Theme;
}

export const StyledTableToolbar: React.ComponentType<ToolbarProps> = styled(
  MUITableToolBar,
  {
    shouldForwardProp: (prop) => prop !== 'numSelected' && prop !== 'theme',
  }
)<ToolbarProps>(({ theme }) => ({
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'center',
  root: {
    paddingLeft: theme.spacing(2),
    paddingRight: theme.spacing(1),
    highlight:
      theme.palette.mode === 'light'
        ? {
            color: theme.palette.secondary.main,
            backgroundColor: lighten(theme.palette.secondary.light, 0.85),
          }
        : {
            color: theme.palette.text.primary,
            backgroundColor: theme.palette.secondary.dark,
          },
  },
  title: {
    flex: '1 1 100%',
  },

  virtualHidden: {
    border: 0,
    clip: 'rect(0 0 0 0)',
    height: 1,
    margin: -1,
    overflow: 'hidden',
    padding: 0,
    position: 'absolute',
    top: theme.spacing(2.5),
    width: 1,
  },
}));
