import React from "react";
import ReactDOM from "react-dom/client";
import {CssBaseline, ThemeProvider } from "@mui/material";
import theme from "./theme";
import App from "./App";
import "@fontsource/poppins/300.css";
import "@fontsource/poppins/400.css";
import "@fontsource/poppins/500.css";
import "@fontsource/poppins/600.css";
import "@fontsource/poppins/700.css";
import { OrganizationProvider } from "./context/OrganizationContext";
import { ProjectProvider } from "./context/ProjectContext";
import { ApiProvider } from "./context/ApiContext";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <ThemeProvider theme={theme}>
        <CssBaseline />
        <OrganizationProvider>
          <ProjectProvider>
            <ApiProvider>
              <App />
            </ApiProvider>
          </ProjectProvider>
        </OrganizationProvider>
    </ThemeProvider>
  </React.StrictMode>
);
