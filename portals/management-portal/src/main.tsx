import React from "react";
import ReactDOM from "react-dom/client";
import { CssBaseline, ThemeProvider } from "@mui/material";
import { BrowserRouter } from "react-router-dom";
import theme from "./theme";
import App from "./App";
import "@fontsource/poppins/300.css";
import "@fontsource/poppins/400.css";
import "@fontsource/poppins/500.css";
import "@fontsource/poppins/600.css";
import "@fontsource/poppins/700.css";
import { OrganizationProvider } from "./context/OrganizationContext";
import { ProjectProvider } from "./context/ProjectContext";
import { APIFeatureProviders } from "./Providers/APIFeatureProviders";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <OrganizationProvider>
        <ProjectProvider>
          <APIFeatureProviders>
            <BrowserRouter>
              <App />
            </BrowserRouter>
          </APIFeatureProviders>
        </ProjectProvider>
      </OrganizationProvider>
    </ThemeProvider>
  </React.StrictMode>
);
