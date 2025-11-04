import { createTheme } from "@mui/material/styles";
import defaultTheme from './components/src/theme/theme.json';
import type { Shadows } from "@mui/material/styles";

// Example theme; keep your palette/typography as needed.
// const themes = createTheme({
//   palette: {
//     mode: "light",
//   },
//   typography: {
//     fontFamily:
//       "'Poppins', system-ui, -apple-system, Segoe UI, Roboto, Arial, sans-serif",
//     // (optional) tighten weights if you like
//     h1: { fontWeight: 700 },
//     h2: { fontWeight: 700 },
//     h3: { fontWeight: 700 },
//     h4: { fontWeight: 700 },
//     h5: { fontWeight: 600 },
//     h6: { fontWeight: 600 },
//     button: { textTransform: "none" },
//   },
//   components: {
//     // example: compact buttons globally
//     MuiButton: {
//       styleOverrides: {
//         root: { borderRadius: 8 },
//       },
//     },
//     MuiCssBaseline: {
//       styleOverrides: {
//         body: {
//           WebkitFontSmoothing: "antialiased",
//           MozOsxFontSmoothing: "grayscale",
//         },
//       },
//     },
//   },
// });
    const typography = {
      ...defaultTheme.typography,
      overline: {
        ...defaultTheme.typography.overline,
        textTransform: 'none' as const,
      },
    };

    const theme =  createTheme({
      typography,
      zIndex: defaultTheme.zIndex,
      palette: {
        ...defaultTheme.colorSchemes.light.palette,
        ...defaultTheme.light,
        mode: 'light',
      },
      shadows: defaultTheme.light.shadows as Shadows,
    });

export default theme;
