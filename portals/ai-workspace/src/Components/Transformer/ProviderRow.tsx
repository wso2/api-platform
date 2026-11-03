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

/**
 * One attached provider, collapsed to a row.
 *
 * Used once a proxy has more than one provider, or while one is being added:
 * the providers already settled shrink to rows so the form being filled in is
 * the only thing expanded.
 *
 * The row carries what distinguishes one provider from another — its name,
 * whether it is the primary, and what translates for it — and nothing else. The
 * rest is behind the configure action.
 */

import React from "react";
import { FormattedMessage } from "react-intl";
import {
  Box,
  Button,
  Card,
  FormControlLabel,
  Stack,
  Switch,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import {
  PenLine,
  PenSquare,
  Trash,
  Trash2,
  X,
} from "@wso2/oxygen-ui-icons-react";
import type { TransformerResolution } from "../../utils/transformerResolution";

export type ProviderRowProps = {
  displayName: string;
  /**
   * What a client puts in a routing header to select this provider. Shown
   * alongside the translator, because a row that names a provider without it
   * does not tell a reader how to reach that provider.
   */
  requestHandle?: string;
  /**
   * `field` while a provider is being chosen, where the row sits among inputs
   * and reads as one. `plain` on a settled list, where it is a record rather
   * than a control.
   */
  variant?: "field" | "plain";
  isPrimary: boolean;
  resolution: TransformerResolution;
  /** Shown only once the proxy has more than one provider to choose between. */
  showPrimaryToggle: boolean;
  onMakePrimary?: () => void;
  onEdit?: () => void;
  onRemove?: () => void;
  /** Offered where translation is needed but nothing provides it. */
  onConfigureTransformer?: () => void;
  disabled?: boolean;
  "data-cyid"?: string;
};

export default function ProviderRow({
  displayName,
  requestHandle,
  variant = "field",
  isPrimary,
  resolution,
  showPrimaryToggle,
  onMakePrimary,
  onEdit,
  onRemove,
  onConfigureTransformer,
  disabled = false,
  "data-cyid": dataCyId = "provider-row",
}: ProviderRowProps) {
  const needsConfiguring =
    resolution.status === "unresolved" || resolution.status === "invalid";
  const isPlain = variant === "plain";

  return (
    <Card
      variant="outlined"
      sx={{ p: 1.5, bgcolor: "action.hover" }}
      data-cyid={dataCyId}
    >
      <Stack spacing={0.75}>
        <Box display="flex" alignItems="center" gap={1}>
          {/* Read-only: which provider this is was settled when it was added. */}
          {isPlain ? (
            <Typography
              variant="body2"
              sx={{ fontWeight: 600, flex: 1 }}
              data-cyid={`${dataCyId}-name`}
            >
              {displayName}
            </Typography>
          ) : (
            <TextField
              fullWidth
              size="small"
              value={displayName}
              slotProps={{ input: { readOnly: true } }}
              // Painted on the input itself, not on the field wrapping it. The
              // wrapper is a plain rectangle; only the input is rounded, so a
              // fill on the wrapper shows past the curve as a square patch at
              // each corner.
              sx={{ "& .MuiOutlinedInput-root": { bgcolor: "background.paper" } }}
              data-cyid={`${dataCyId}-name`}
            />
          )}
          {showPrimaryToggle && (
            <FormControlLabel
              labelPlacement="start"
              control={
                <Switch
                  size="small"
                  checked={isPrimary}
                  onChange={onMakePrimary}
                  disabled={disabled}
                />
              }
              label={
                <FormattedMessage
                  id="aiWorkspace.components.providerRow.primary"
                  defaultMessage="Primary"
                />
              }
              disabled={disabled}
              data-cyid={`${dataCyId}-primary`}
              sx={{ flexShrink: 0, m: 0, gap: 0.5 }}
            />
          )}
          <Button
            size="small"
            // variant="outlined"
            color="inherit"
            onClick={onEdit}
            disabled={disabled}
            sx={{ minWidth: 0, px: 1, flexShrink: 0 }}
            data-cyid={`${dataCyId}-edit`}
          >
            <PenSquare size={16} />
          </Button>
          <Button
            size="small"
            // variant="outlined"
            onClick={onRemove}
            disabled={disabled || !onRemove}
            sx={{ minWidth: 0, px: 1, flexShrink: 0 }}
            data-cyid={`${dataCyId}-remove`}
            color="error"
          >
            <Trash2 size={16} />
          </Button>
        </Box>

        {/*
          A row reports only what is waiting on someone. A provider that needs
          no translator, or already has the one it needs, has nothing here for a
          reader to act on — and four rows each carrying a line that says so is
          a list nobody reads. What is attached, and the option to change it,
          are a click away behind the edit action.
        */}
        {(requestHandle || (needsConfiguring && resolution.title)) && (
          <Box
            display="flex"
            flexDirection="column"
            alignItems="flex-start"
            gap={0.25}
            sx={{ pl: 0.5 }}
          >
            {requestHandle && (
              <Typography variant="caption" data-cyid={`${dataCyId}-handle`}>
                Provider ID: {requestHandle}
              </Typography>
            )}

            {/*
              Unconfigured, or configured with something that no longer
              translates between these two formats. Both are a provider that
              will not serve until someone opens it, which is the whole reason
              this line survives the rule above.
            */}
            {needsConfiguring && resolution.title && (
              <Typography
                variant="caption"
                color="warning.main"
                data-cyid={`${dataCyId}-transformer-status`}
              >
                Transformer: {resolution.title}
              </Typography>
            )}
          </Box>
        )}
      </Stack>
    </Card>
  );
}
