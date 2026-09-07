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

import { useState, type FC } from "react";
import {
  Box,
  Button,
  FormControl,
  FormControlLabel,
  FormLabel,
  PageContent,
  PageTitle,
  Stack,
  Switch,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { ChevronLeft } from "@wso2/oxygen-ui-icons-react";
import type { NotifySeverity } from "./hostPort";
import type { EnvironmentPort } from "./types";
import { validateEnvironmentName } from "./utils/name";

export type EnvironmentFormProps = {
  port: EnvironmentPort;
  onBack: () => void;
  notify?: (message: string, severity?: NotifySeverity) => void;
};

/** Shown until the name breaks a rule, so the constraint is known up front. */
const NAME_HELPER_TEXT =
  "Lowercase letters, numbers and hyphens only. The name cannot be changed later.";

const EnvironmentForm: FC<EnvironmentFormProps> = ({ port, onBack, notify }) => {
  const [name, setName] = useState("");
  const [critical, setCritical] = useState(false);
  const [saving, setSaving] = useState(false);
  const nameError = validateEnvironmentName(name);
  const canSubmit = name.trim().length > 0 && !nameError && !saving;

  const handleSubmit = async () => {
    // Guard against a second click issuing a duplicate create while the first is
    // still in flight.
    if (saving) return;
    setSaving(true);
    try {
      const created = await port.create({ name: name.trim(), critical });
      notify?.(`Environment "${created.name}" created.`, "success");
      onBack();
    } catch (error) {
      // Without this the rejection is unhandled: the form sits there having said
      // nothing, and the user has no way to tell the create failed.
      notify?.(
        error instanceof Error
          ? error.message
          : "Unable to create the environment.",
        "error",
      );
    } finally {
      setSaving(false);
    }
  };

  return (
    <PageContent fullWidth>
      <Button
        size="small"
        startIcon={<ChevronLeft size={18} />}
        onClick={onBack}
      >
        Back to environments
      </Button>
      <PageTitle sx={{ mt: 2 }}>
        <PageTitle.Header>Create Environment</PageTitle.Header>
        <PageTitle.SubHeader>
          Create an organization-level deployment environment.
        </PageTitle.SubHeader>
      </PageTitle>

      <Stack spacing={3} sx={{ mt: 3, maxWidth: 720 }}>
        <FormControl fullWidth>
          <FormLabel required>Name</FormLabel>
          <TextField
            fullWidth
            required
            autoFocus
            placeholder="Enter environment name"
            value={name}
            onChange={(event) => setName(event.target.value)}
            error={Boolean(nameError)}
            helperText={nameError ?? NAME_HELPER_TEXT}
          />
        </FormControl>

        <Box>
          <FormControlLabel
            control={
              <Switch
                checked={critical}
                onChange={(event) => setCritical(event.target.checked)}
              />
            }
            label="Critical environment"
          />
          <Typography variant="body2" color="text.secondary">
            Critical environments are used for production-grade deployments and
            require greater care.
          </Typography>
        </Box>

        <Stack direction="row" spacing={1.5}>
          <Button variant="outlined" color="secondary" onClick={onBack}>
            Cancel
          </Button>
          <Tooltip
            title={
              canSubmit || saving
                ? ""
                : nameError ?? "Enter an environment name to continue."
            }
          >
            <span>
              <Button
                variant="contained"
                disabled={!canSubmit}
                onClick={handleSubmit}
              >
                {saving ? "Creating…" : "Create"}
              </Button>
            </span>
          </Tooltip>
        </Stack>
      </Stack>
    </PageContent>
  );
};

export default EnvironmentForm;
