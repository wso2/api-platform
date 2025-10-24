import React from "react";
import { Box, Typography, Stack, TextField } from "@mui/material";
import type { GwType, GatewayRecord } from "./types";
import { slugify } from "./utils";
import { TextInput } from "../../components/src/components/TextInput/TextInput";
import { Button } from "../../components/src/components/Button";

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
  }) => void;
}) {
  const [displayName, setDisplayName] = React.useState(
    defaults?.displayName ?? ""
  );
  const [name, setName] = React.useState(defaults?.name ?? "");
  const [host, setHost] = React.useState(defaults?.host ?? "");
  const [description, setDescription] = React.useState(
    defaults?.description ?? ""
  );

  React.useEffect(() => {
    setName(displayName ? slugify(displayName) : "");
  }, [displayName]);

  return (
    <Box maxWidth={640} display={"flex"} flexDirection="column" alignItems={"flex-start"}>
      <Typography variant="body1" mb={2} fontWeight={600}>
        {defaults?.id
          ? `Edit ${type === "hybrid" ? "On Premise" : "Cloud"} Gateway`
          : `Create ${type === "hybrid" ? "On Premise" : "Cloud"} Gateway`}
      </Typography>

      <Stack spacing={2} width="100%">
        {/* <TextInput
          placeholder="API Name"
          testId="api-name"
          value="api-name"
          onChange={(text: string) => setName(text)}
        /> */}
        <TextField
          label="Display name"
          value={displayName}
          onChange={(e) => setDisplayName(e.target.value)}
          fullWidth
          autoFocus
          placeholder="e.g., My Production Gateway"
        />

        <TextField
          label="Name"
          value={name}
          fullWidth
          InputProps={{ readOnly: true }}
          helperText="Generated from Display name (read-only)"
        />

        <TextField
          label="Host"
          value={host}
          onChange={(e) => setHost(e.target.value)}
          fullWidth
          placeholder="e.g., gateway.dev.local"
        />

        <TextField
          label="Description"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          fullWidth
          multiline
          minRows={3}
          placeholder="Optional description for your gateway"
        />

        <Stack direction="row" spacing={2} justifyContent="flex-start" mt={1}>
          <Button
            variant="outlined"
            onClick={onCancel}
            sx={{
              textTransform: "none",
            }}
          >
            Cancel
          </Button>
          <Button
            variant="contained"
            onClick={() => onSubmit({ displayName, name, host, description })}
            disabled={!displayName.trim()}
            sx={{
              textTransform: "none",
            }}
          >
            {defaults?.id ? "Save" : "Add"}
          </Button>
        </Stack>
      </Stack>
    </Box>
  );
}
