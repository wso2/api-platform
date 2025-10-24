import { useTheme as useMuiTheme, type Theme } from '@mui/material/styles';
import { useMediaQuery as useMuiMediaQuery } from '@mui/material';

export function useMediaQuery(
  query: number | 'lg' | 'md' | 'sm',
  side: 'up' | 'down' = 'up'
) {
  const theme = useMuiTheme();
  return useMuiMediaQuery(theme.breakpoints[side](query));
}

export function useChoreoTheme(): {
  pallet: Theme['palette'];
  shadows: Theme['shadows'];
  typography: Theme['typography'];
  zIndex: Theme['zIndex'];
  breakpoints: Theme['breakpoints'];
  components: Theme['components'];
  transitions: Theme['transitions'];
  spacing: Theme['spacing'];
  shape: Theme['shape'];
} {
  const theme = useMuiTheme();
  return {
    pallet: theme.palette,
    shadows: theme.shadows,
    typography: theme.typography,
    zIndex: theme.zIndex,
    breakpoints: theme.breakpoints,
    components: theme.components,
    transitions: theme.transitions,
    spacing: theme.spacing,
    shape: theme.shape,
  };
}
