import * as React from "react";
import { Box, FormControl, Grid, Typography } from "@mui/material";
import { Button } from "../../components/src/components/Button";
import { TextInput } from "../../components/src";

type Props = {
  portalName: string;
  portalIdentifier: string;
  portalDomains: string; // reuse this prop to hold the description text

  onChangeName: (v: string) => void;
  onChangeIdentifier: (v: string) => void;
  onChangeDomains: (v: string) => void; // used for description

  onContinue: () => void;
  canContinue?: boolean; // default: true
};

const MAX_IDENTIFIER_LEN = 64;

const PortalForm: React.FC<Props> = ({
  portalName,
  portalIdentifier,
  portalDomains,
  onChangeName,
  onChangeIdentifier,
  onChangeDomains,
  onContinue,
  canContinue = true,
}) => {
  return (
    <Box>
      <Typography variant="h5" sx={{ mb: 1 }}>
        Add Your Portal Details
      </Typography>

      <Grid container spacing={2}>
        {/* Row 1: Name / Identifier */}
        <Grid size={{ xs: 12, md: 6 }}>
          <FormControl fullWidth>
            <TextInput
              size="medium"
              label="Portal name"
              placeholder="Provide a name for your portal"
              value={portalName}
              onChange={onChangeName}
              testId=""
            />
          </FormControl>
        </Grid>

        <Grid size={{ xs: 12, md: 6 }}>
          <FormControl fullWidth>
            <TextInput
              size="medium"
              label="Identifier"
              placeholder="Auto-generated from name"
              value={portalIdentifier}
              onChange={(v: string) =>
                onChangeIdentifier(v.slice(0, MAX_IDENTIFIER_LEN))
              }
              testId=""
            />
          </FormControl>
        </Grid>

        {/* Row 2: Description */}
        <Grid size={{ xs: 12 }}>
          <FormControl fullWidth>
            <TextInput
              label="Description"
              placeholder="Briefly describe your developer portal..."
              value={portalDomains} // using existing prop to store description text
              onChange={onChangeDomains} // reuse handler
              multiline
              testId=""
            />
          </FormControl>
        </Grid>
      </Grid>

      <Button
        variant="contained"
        disabled={!canContinue}
        onClick={onContinue}
        sx={{ mt: 2 }}
      >
        Create and continue
      </Button>
    </Box>
  );
};

export default PortalForm;
