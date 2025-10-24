import React from "react";
import { Typography, Box } from "@mui/material";

const ApiDeploy: React.FC = () => (
  <Box>
    <Typography variant="h4" fontWeight={800} gutterBottom>
      APIs · Deploy
    </Typography>
    <Typography color="text.secondary">
      Pipelines, environments, and deployment actions for APIs.
    </Typography>
  </Box>
);

export default ApiDeploy;
