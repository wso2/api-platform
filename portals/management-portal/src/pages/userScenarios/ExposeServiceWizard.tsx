// src/pages/ExposeServiceWizard.tsx
import * as React from "react";
import { Box, Stack, Typography } from "@mui/material";
import GatewayWizard from "../overview/StepHeader";
import { Button } from "../../components/src/components/Button";

type Props = {
  onBackToChoices: () => void;
  onSkip: () => void;
  onFinish: () => void;
};

const ExposeServiceWizard: React.FC<Props> = ({ onBackToChoices, onSkip, onFinish }) => {
  return (
    <Box maxWidth={1240} mx="auto">
      <Stack
        direction={{ xs: "column", sm: "row" }}
        alignItems={{ xs: "flex-start", sm: "center" }}
        justifyContent="space-between"
        spacing={2}
        mb={3}
      >
        <Box>
          <Typography variant="h4" fontWeight={700}>
            Expose My Service Securely
          </Typography>
          <Typography color="#AEAEAE">
            Launch the guided Gateway wizard to publish your service with the
            right guardrails.
          </Typography>
        </Box>
        <Stack direction="row" spacing={1}>
          <Button variant="text" onClick={onBackToChoices}>
            Back to choices
          </Button>
          <Button variant="outlined"  onClick={onSkip}>
            Skip for now
          </Button>
        </Stack>
      </Stack>

      <GatewayWizard onFinish={onFinish} />
    </Box>
  );
};

export default ExposeServiceWizard;
