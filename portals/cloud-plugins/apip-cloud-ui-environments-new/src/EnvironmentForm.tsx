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
import { createEnvironment } from "./mocks/environmentsStore";
import type { NotifySeverity } from "./hostPort";

export type EnvironmentFormProps = {
  onBack: () => void;
  notify?: (message: string, severity?: NotifySeverity) => void;
};

const EnvironmentForm: FC<EnvironmentFormProps> = ({ onBack, notify }) => {
  const [name, setName] = useState("");
  const [critical, setCritical] = useState(false);
  const canSubmit = name.trim().length > 0;

  const handleSubmit = () => {
    const created = createEnvironment({ name: name.trim(), critical });
    notify?.(`Environment "${created.name}" created.`, "success");
    onBack();
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
            title={canSubmit ? "" : "Enter an environment name to continue."}
          >
            <span>
              <Button
                variant="contained"
                disabled={!canSubmit}
                onClick={handleSubmit}
              >
                Create
              </Button>
            </span>
          </Tooltip>
        </Stack>
      </Stack>
    </PageContent>
  );
};

export default EnvironmentForm;
