import * as React from "react";
import { Box, Stack, Typography } from "@mui/material";
import { Button } from "../../components/src/components/Button";
import { DevPortalProvider } from "../../context/DevPortalContext";
import PublishPortalFlow from "./PublishPortalFlow";

type Props = {
  onBackToChoices: () => void;
  onSkip: () => void;
  onFinish: () => void;
};

const PublishPortalWizard: React.FC<Props> = ({ onBackToChoices, onSkip, onFinish }) => {
  return (
    <DevPortalProvider>
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
              Publish to Developer Portal
            </Typography>
            <Typography color="#AEAEAE">
              Import your API and publish it to a developer portal for discovery and subscription.
            </Typography>
          </Box>
          <Stack direction="row" spacing={1}>
            <Button variant="text" onClick={onBackToChoices}>
              Back to choices
            </Button>
            <Button variant="outlined" onClick={onSkip}>
              Skip for now
            </Button>
          </Stack>
        </Stack>

        <PublishPortalFlow onFinish={onFinish} />
      </Box>
    </DevPortalProvider>
  );
};

export default PublishPortalWizard;
