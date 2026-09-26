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
              sx={{ bgcolor: "background.paper" }}
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
          What translates for this provider, in one line. Nothing is shown for a
          provider still being chosen; the matching case says so plainly rather
          than leaving a gap that could equally mean "still loading".
        */}
        {(resolution.title || requestHandle) && (
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

            {resolution.status === "none" ? (
              resolution.title ? (
                <Typography variant="caption" color="info.main">
                  Transformer:{" "}
                  <FormattedMessage
                    id="aiWorkspace.components.providerRow.noTransformerNeeded"
                    defaultMessage="No transformer needed"
                  />
                </Typography>
              ) : null
            ) : resolution.policy ? (
              <>
                <Typography variant="caption" color="success.main">
                  Transformer: {resolution.policy.name}
                </Typography>

                {resolution.note && (
                  <Typography variant="caption" color="warning.main">
                    {resolution.note}
                  </Typography>
                )}
              </>
            ) : resolution.title ? (
              <Box display="flex" alignItems="center" gap={1}>
                <Typography variant="caption" color="warning.main">
                  Transformer: {resolution.title}
                </Typography>

                {/* {needsConfiguring && onConfigureTransformer && (
                  <Button
                    size="small"
                    onClick={onConfigureTransformer}
                    disabled={disabled}
                    sx={{ px: 0.5, minHeight: 0 }}
                    data-cyid={`${dataCyId}-configure-transformer`}
                  >
                    <FormattedMessage
                      id="aiWorkspace.components.providerRow.configureTransformer"
                      defaultMessage="Configure transformer"
                    />
                  </Button>
                )} */}
              </Box>
            ) : null}
          </Box>
        )}
      </Stack>
    </Card>
  );
}
