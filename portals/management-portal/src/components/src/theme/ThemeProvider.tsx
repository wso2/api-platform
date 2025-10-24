import { createTheme, type PaletteOptions, type Shadows } from '@mui/material';
import { ThemeProvider as MuiThemeProvider } from '@mui/material/styles';
import defaultTheme from './theme.json';
import React from 'react';
import '../fonts/fonts.css';
import './initialLoader.css';

export interface ThemeProviderProps {
  children: React.ReactNode;
  mode?: 'light' | 'dark';
  customPalette?: { dark: PaletteOptions; light: PaletteOptions };
}

export function ThemeProvider(props: ThemeProviderProps) {
  const { children, mode = 'light', customPalette } = props;

  const darkTheme = React.useMemo(() => {
    const typography = {
      ...defaultTheme.typography,
      overline: {
        ...defaultTheme.typography.overline,
        textTransform: 'none' as const,
      },
    };

    return createTheme({
      typography,
      zIndex: defaultTheme.zIndex,
      palette: {
        ...defaultTheme.colorSchemes.dark.palette,
        ...customPalette?.dark,
        mode: 'dark',
      },
      shadows: defaultTheme.dark.shadows as Shadows,
    });
  }, [customPalette]);

  const lightTheme = React.useMemo(() => {
    const typography = {
      ...defaultTheme.typography,
      overline: {
        ...defaultTheme.typography.overline,
        textTransform: 'none' as const,
      },
    };

    return createTheme({
      typography,
      zIndex: defaultTheme.zIndex,
      palette: {
        ...defaultTheme.colorSchemes.light.palette,
        ...customPalette?.light,
        mode: 'light',
      },
      shadows: defaultTheme.light.shadows as Shadows,
    });
  }, [customPalette]);

  return (
    <MuiThemeProvider theme={mode === 'dark' ? darkTheme : lightTheme}>
      {children}
    </MuiThemeProvider>
  );
}
