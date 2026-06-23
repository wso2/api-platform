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

import { useState, useEffect } from "react";
import { useNavigate, Link as RouterLink } from "react-router-dom";
import {
  Box,
  Button,
  Chip,
  TextField,
  Grid,
  PageContent,
  PageTitle,
  Stack,
  FormControl,
  FormLabel,
  Select,
  MenuItem,
  ListItemText,
} from "@wso2/oxygen-ui";
import { ChevronLeft } from "@wso2/oxygen-ui-icons-react";
import { useGatewayList } from "../../../../hooks/useGateway";
import { useAppShell } from "../../../../contexts/AppShellContext";
import { buildOrgPath } from "../../../../utils/projectRouting";
import { setRegistrationToken } from "./registrationTokenStore";
import { useAIWorkspaceSnackbar } from "../../../../hooks/aiWorkspaceSnackbar";
import { PLATFORM_GATEWAY_VERSIONS } from "../../../../config.env";
import type { GatewayVersionEntry } from "../../../../config.env";

// Validation constants
const MAX_NAME_LENGTH = 255;
const MAX_DESCRIPTION_LENGTH = 1023;

/**
 * Normalizes vhost input by stripping http:// or https:// and returning
 * only the hostname and port.
 */
const normalizeVhost = (value: string): string => {
  const trimmed = value.trim();
  if (trimmed.startsWith("https://")) return trimmed.slice(8);
  if (trimmed.startsWith("http://")) return trimmed.slice(7);
  return trimmed;
};

/**
 * Returns the full URL for display (adds https:// if vhost has no protocol).
 */
const getDisplayUrl = (vhost: string): string => {
  if (!vhost || !vhost.trim()) return "";
  const trimmed = vhost.trim();
  if (trimmed.startsWith("http://") || trimmed.startsWith("https://")) {
    return trimmed;
  }
  return `https://${trimmed}`;
};

/**
 * Generates a valid gateway name from a display name.
 */
const generateGatewayName = (displayName: string): string => {
  if (!displayName || displayName.trim().length === 0) {
    return "";
  }

  return displayName
    .trim()
    .toLowerCase()
    .replace(/\s+/g, "-")
    .replace(/[^a-z0-9-]/g, "")
    .replace(/^-+|-+$/g, "")
    .replace(/-+/g, "-")
    .substring(0, 64)
    .replace(/-+$/g, "");
};

export default function AddGateway() {
  const navigate = useNavigate();
  const { currentOrganization } = useAppShell();
  const { createGateway, isCreating } = useGatewayList();
  const showSnackbar = useAIWorkspaceSnackbar();

  const [displayName, setDisplayName] = useState("");
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [vhost, setVhost] = useState(() =>
    normalizeVhost("https://localhost:8443"),
  );
  const [selectedVersion, setSelectedVersion] = useState<GatewayVersionEntry | null>(
    () => PLATFORM_GATEWAY_VERSIONS[0] ?? null,
  );

  // Always use AI gateway type
  const functionalityType = "ai";

  // Generate name from display name
  useEffect(() => {
    setName(generateGatewayName(displayName));
  }, [displayName]);

  const isFormValid = (): boolean => {
    if (!selectedVersion) return false;
    if (!displayName || displayName.trim().length === 0) return false;
    if (displayName.length > MAX_NAME_LENGTH) return false;
    if (description.length > MAX_DESCRIPTION_LENGTH) return false;
    const normalizedVhost = normalizeVhost(vhost || "");
    if (!normalizedVhost || normalizedVhost.length === 0) return false;
    return true;
  };

  const handleSubmit = async (event?: React.FormEvent) => {
    if (event) {
      event.preventDefault();
    }

    if (!isFormValid()) return;

    try {
      const createdGateway = await createGateway({
        displayName,
        name,
        vhost: normalizeVhost(vhost),
        functionalityType,
        description: description || undefined,
        version: selectedVersion!.version,
      });

      showSnackbar("AI Gateway registered successfully", "success");

      // Store token in memory if returned (one-time view)
      if (createdGateway.token) {
        setRegistrationToken(createdGateway.token);
      }

      // Redirect to the gateway view page
      const viewPath = buildOrgPath(
        currentOrganization,
        `/gateways/view/${createdGateway.name}`,
      );
      navigate(viewPath);
    } catch (error: any) {
      showSnackbar(
        error?.message || "Failed to register self-hosted gateway",
        "error",
      );
    }
  };

  const handleCancel = () => {
    const listPath = buildOrgPath(currentOrganization, "/gateways");
    navigate(listPath);
  };

  return (
    <PageContent fullWidth>
      <Button
        component={RouterLink}
        to={buildOrgPath(currentOrganization, "/gateways")}
        size="small"
        startIcon={<ChevronLeft size={24} />}
      >
        Back to list
      </Button>

      <Stack spacing={2} mt={2}>
        <PageTitle>
          <PageTitle.Header>Add AI Gateway</PageTitle.Header>
        </PageTitle>
      </Stack>

      <Box sx={{ mt: 2, maxWidth: 820 }}>
        <Box component="form" onSubmit={handleSubmit} noValidate>
          <Grid container spacing={2}>
            <Grid size={{ xs: 12 }}>
              <FormControl fullWidth>
                <FormLabel required>Gateway Version</FormLabel>
                <Select
                  value={selectedVersion?.version ?? ""}
                  onChange={(e) => {
                    const entry = PLATFORM_GATEWAY_VERSIONS.find(
                      (v) => v.version === e.target.value,
                    );
                    if (entry) setSelectedVersion(entry);
                  }}
                  renderValue={(value) => {
                    const entry = PLATFORM_GATEWAY_VERSIONS.find(
                      (v) => v.version === value,
                    );
                    if (!entry) return value;
                    const isFirst =
                      PLATFORM_GATEWAY_VERSIONS[0]?.version === entry.version;
                    const isFirstLTS =
                      entry.channel === "LTS" &&
                      PLATFORM_GATEWAY_VERSIONS.find(
                        (v) => v.channel === "LTS",
                      )?.version === entry.version;
                    const badge = isFirst
                      ? "Latest"
                      : isFirstLTS
                        ? "LTS"
                        : null;
                    return (
                      <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                        <span>{`AI-Gateway v${entry.version}`}</span>
                        {badge && (
                          <Chip
                            label={badge}
                            size="small"
                            color={badge === "Latest" ? "success" : "primary"}
                          />
                        )}
                      </Box>
                    );
                  }}
                >
                  {(() => {
                    let firstLTSSeen = false;
                    return PLATFORM_GATEWAY_VERSIONS.map((entry, index) => {
                      const isFirst = index === 0;
                      const isFirstLTS =
                        !firstLTSSeen && entry.channel === "LTS";
                      if (entry.channel === "LTS" && !firstLTSSeen)
                        firstLTSSeen = true;
                      const badge = isFirst
                        ? "Latest"
                        : isFirstLTS
                          ? "LTS"
                          : null;
                      return (
                        <MenuItem key={entry.version} value={entry.version}>
                          <ListItemText
                            primary={
                              <Box
                                sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", width: "100%" }}
                              >
                                <span>{`AI-Gateway v${entry.version}`}</span>
                                {badge && (
                                  <Chip
                                    label={badge}
                                    size="small"
                                    color={
                                      badge === "Latest" ? "success" : "primary"
                                    }
                                  />
                                )}
                              </Box>
                            }
                          />
                        </MenuItem>
                      );
                    });
                  })()}
                </Select>
              </FormControl>
            </Grid>

            <Grid size={{ xs: 12 }}>
              <FormControl fullWidth>
                <FormLabel required>Name</FormLabel>
                <TextField
                  fullWidth
                  required
                  value={displayName}
                  onChange={(e) => setDisplayName(e.target.value)}
                  placeholder="Enter gateway name"
                  error={displayName.length > MAX_NAME_LENGTH}
                  helperText={
                    displayName.length > MAX_NAME_LENGTH
                      ? `Name must not exceed ${MAX_NAME_LENGTH} characters (${displayName.length}/${MAX_NAME_LENGTH})`
                      : ""
                  }
                />
              </FormControl>
            </Grid>

            <Grid size={{ xs: 12 }}>
              <FormControl fullWidth>
                <FormLabel>Description (Optional)</FormLabel>
                <TextField
                  fullWidth
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  placeholder="Enter description"
                  multiline
                  minRows={3}
                  error={description.length > MAX_DESCRIPTION_LENGTH}
                  helperText={
                    description.length > MAX_DESCRIPTION_LENGTH
                      ? `Description must not exceed ${MAX_DESCRIPTION_LENGTH} characters (${description.length}/${MAX_DESCRIPTION_LENGTH})`
                      : ""
                  }
                />
              </FormControl>
            </Grid>
            <Grid size={{ xs: 12 }}>
              <FormControl fullWidth>
                <FormLabel required>URL</FormLabel>
                <TextField
                  fullWidth
                  required
                  value={getDisplayUrl(vhost)}
                  onChange={(e) => setVhost(normalizeVhost(e.target.value))}
                  placeholder="Enter gateway URL"
                />
              </FormControl>
            </Grid>
          </Grid>

          <Box
            sx={{
              mt: 3,
              display: "flex",
              justifyContent: "flex-start",
              gap: 1,
            }}
          >
            <Button
              variant="outlined"
              component={RouterLink}
              to={buildOrgPath(currentOrganization, "/gateways")}
              color="secondary"
              type="button"
            >
              Cancel
            </Button>
            <Button
              variant="contained"
              type="submit"
              disabled={isCreating || !isFormValid()}
            >
              {isCreating ? "Adding Gateway..." : "Add Gateway"}
            </Button>
          </Box>
        </Box>
      </Box>
    </PageContent>
  );
}
