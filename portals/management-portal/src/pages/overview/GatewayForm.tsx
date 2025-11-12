import React from "react";
import {
  Box,
  Typography,
  Stack,
  FormControlLabel,
  Checkbox,
} from "@mui/material";
import type { GwType, GatewayRecord } from "./types";
import { slugify } from "./utils";
import { Button } from "../../components/src/components/Button";
import { TextInput } from "../../components/src/components/TextInput";

export default function GatewayForm({
  type,
  defaults,
  onCancel,
  onSubmit,
}: {
  type: GwType;
  defaults?: Partial<GatewayRecord>;
  onCancel: () => void;
  onSubmit: (data: {
    displayName: string;
    name: string;
    host: string;
    description: string;
    isCritical: boolean;
  }) => void;
}) {
  const [displayName, setDisplayName] = React.useState(
    defaults?.displayName ?? ""
  );
  const [isCritical, setIsCritical] = React.useState(false);
  const [name, setName] = React.useState(defaults?.name ?? "");
  const [host, setHost] = React.useState(defaults?.host ?? "");
  const [description, setDescription] = React.useState(
    defaults?.description ?? ""
  );

  React.useEffect(() => {
    setName(displayName ? slugify(displayName) : "");
  }, [displayName]);

  return (
    <Box
      maxWidth={640}
      display={"flex"}
      flexDirection="column"
      alignItems={"flex-start"}
    >
      <Typography variant="body1" mb={2} fontWeight={600}>
        {defaults?.id
          ? `Edit ${type === "hybrid" ? "On Premise" : "Cloud"} Gateway`
          : `Create ${type === "hybrid" ? "On Premise" : "Cloud"} Gateway`}
      </Typography>

      <Stack spacing={2} width="100%">
        <TextInput
          label="Display name"
          placeholder="e.g., My Production Gateway"
          value={displayName}
          onChange={(v: string) => setDisplayName(v)}
          fullWidth
          testId="gateway-display-name"
          size="medium"
        />

        <TextInput
          label="Name"
          value={name}
          onChange={() => {}}
          fullWidth
          testId="gateway-name"
          size="medium"
          readonly
        />

        <TextInput
          label="Host"
          placeholder="e.g., gateway.dev.local"
          value={host}
          onChange={(v: string) => setHost(v)}
          fullWidth
          testId="gateway-host"
          size="medium"
        />

        <TextInput
          label="Description"
          placeholder="Optional description for your gateway"
          value={description}
          onChange={(v: string) => setDescription(v)}
          fullWidth
          multiline
          testId="gateway-description"
        />

        <FormControlLabel
          control={
            <Checkbox
              checked={isCritical}
              onChange={(e) => setIsCritical(e.target.checked)}
            />
          }
          label="Mark this as a critical gateway"
        />

        <Stack direction="row" spacing={2} justifyContent="flex-start" mt={1}>
          <Button
            variant="outlined"
            onClick={onCancel}
            sx={{ textTransform: "none" }}
          >
            Cancel
          </Button>
          <Button
            variant="contained"
            onClick={() =>
              onSubmit({ displayName, name, host, description, isCritical })
            }
            disabled={!displayName.trim()}
            sx={{ textTransform: "none" }}
          >
            {defaults?.id ? "Save" : "Add"}
          </Button>
        </Stack>
      </Stack>
    </Box>
  );
}
