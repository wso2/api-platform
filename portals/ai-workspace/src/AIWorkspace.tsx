/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import React, { useEffect } from "react";
import { BrowserRouter } from "react-router-dom";
import { IntlProvider } from "react-intl";
import {
  AcrylicOrangeTheme,
  AcrylicPurpleTheme,
  ClassicTheme,
  HighContrastTheme,
  OxygenUIThemeProvider,
  Box,
  LinearProgress,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";

import App from "./App";
import "./styles.css";
import { AUTH_MODE } from "./config.env";
import { BASE_PATH } from "./paths";
import { BFFAuthProvider } from "./contexts/BFFAuthProvider";
import { useAppAuth } from "./contexts/AppAuthContext";
import BasicAuthLoginPage from "./pages/login/BasicAuthLoginPage";
import type { AIWorkspaceCloudEntry } from "./extensions";

function LoadingScreen({ message }: { message?: string }) {
  return (
    <Box
      sx={{
        minHeight: "100vh",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        p: 4,
      }}
    >
      <Stack
        spacing={2}
        alignItems="center"
        sx={{ maxWidth: 480, textAlign: "center" }}
      >
        {message && <Typography color="text.secondary">{message}</Typography>}
        <Box sx={{ width: 200 }}>
          <LinearProgress color="primary" />
        </Box>
      </Stack>
    </Box>
  );
}

function OIDCRedirect() {
  const { login } = useAppAuth();
  useEffect(() => {
    void login();
  }, [login]);
  return <LoadingScreen message="Redirecting to sign in…" />;
}

function AppGate({
  extensions,
}: {
  extensions: readonly AIWorkspaceCloudEntry[];
}) {
  const { isAuthenticated, isLoading } = useAppAuth();
  if (isLoading) return <LoadingScreen />;
  if (!isAuthenticated) {
    if (AUTH_MODE === "basic") {
      return (
        <BasicAuthLoginPage
          onSuccess={() => {
            const { pathname, search, hash } = window.location;
            const route = pathname.startsWith(BASE_PATH)
              ? pathname.slice(BASE_PATH.length)
              : pathname;
            window.location.replace(
              route === "/login" || route === "/signin"
                ? `${BASE_PATH}/`
                : pathname + search + hash,
            );
          }}
        />
      );
    }
    return <OIDCRedirect />;
  }
  return (
    <IntlProvider locale="en" defaultLocale="en">
      <App extensions={extensions} />
    </IntlProvider>
  );
}

export type AIWorkspaceProps = {
  extensions?: readonly AIWorkspaceCloudEntry[];
};

export default function AIWorkspace({ extensions = [] }: AIWorkspaceProps) {
  return (
    <OxygenUIThemeProvider
      themes={[
        {
          key: "acrylicOrange",
          label: "Acrylic Orange Theme",
          theme: AcrylicOrangeTheme,
        },
        {
          key: "acrylicPurple",
          label: "Acrylic Purple Theme",
          theme: AcrylicPurpleTheme,
        },
        {
          key: "highContrast",
          label: "High Contrast Theme",
          theme: HighContrastTheme,
        },
        { key: "classic", label: "Classic Theme", theme: ClassicTheme },
      ]}
      initialTheme="acrylicOrange"
    >
      <BrowserRouter basename={BASE_PATH}>
        <BFFAuthProvider>
          <AppGate extensions={extensions} />
        </BFFAuthProvider>
      </BrowserRouter>
    </OxygenUIThemeProvider>
  );
}
