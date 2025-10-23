import { createTheme } from "@mui/material/styles";

// Example theme; keep your palette/typography as needed.
const theme = createTheme({
  palette: {
    mode: "light",
  },
  typography: {
    fontFamily:
      "'Poppins', system-ui, -apple-system, Segoe UI, Roboto, Arial, sans-serif",
    // (optional) tighten weights if you like
    h1: { fontWeight: 700 },
    h2: { fontWeight: 700 },
    h3: { fontWeight: 700 },
    h4: { fontWeight: 700 },
    h5: { fontWeight: 600 },
    h6: { fontWeight: 600 },
    button: { textTransform: "none" },
  },
  components: {
    // example: compact buttons globally
    MuiButton: {
      styleOverrides: {
        root: { borderRadius: 8 },
      },
    },
    MuiCssBaseline: {
      styleOverrides: {
        body: {
          WebkitFontSmoothing: "antialiased",
          MozOsxFontSmoothing: "grayscale",
        },
      },
    },
  },
});

export default theme;
